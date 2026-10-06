#!/bin/sh
# End-to-end: install from release archives with the real install.sh, then
# drive a profile through its life in fresh login shells, the way someone at a
# terminal would. Everything happens under a throwaway $HOME.
#
#   make dist VERSION=v0.0.0
#   sh test/e2e.sh dist v0.0.0
#
# CI runs it on macOS and Linux. What it cannot reach: signing an account in,
# launching Claude Desktop, opening VS Code -- those need a person.
set -eu

dist=$(cd "$1" && pwd)
version=$2
repo=$(cd "$(dirname "$0")/.." && pwd)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
H="$work/home"
mkdir -p "$H" "$work/rel/$version"
cp "$dist"/*.tar.gz "$dist/checksums.txt" "$work/rel/$version/"

os=$(uname -s)
failures=0
step() { printf '\n== %s\n' "$*"; }
ok()   { printf 'ok    %s\n' "$*"; }
bad()  { printf 'FAIL  %s\n' "$*"; failures=$((failures + 1)); }
check() { # check <description> <command...>
	d=$1; shift
	if "$@" >/dev/null 2>&1; then ok "$d"; else bad "$d"; fi
}

# A clean environment, so nothing from the runner's own shell setup leaks in.
clean_env() {
	env -i HOME="$H" USER="${USER:-runner}" TERM=dumb \
		PATH=/usr/bin:/bin:/usr/sbin:/sbin "$@"
}

install_as() { # install_as <shell>
	clean_env SHELL="$(command -v "$1")" CLAUDE_PROFILE_BASE_URL="file://$work/rel" \
		sh "$repo/install.sh" --version "$version"
}

# A new terminal: macOS starts login shells, so bash there reads only
# .bash_profile; Linux terminals start interactive non-login shells, which
# read .bashrc. Testing each the way it really starts is the point.
in_shell() { # in_shell <shell> <script>
	case "$1" in
	zsh) clean_env SHELL="$(command -v zsh)" zsh -ic "$2" ;;
	bash)
		if [ "$os" = Darwin ]; then
			clean_env SHELL="$(command -v bash)" bash -lc "$2"
		else
			clean_env SHELL="$(command -v bash)" bash -ic "$2"
		fi
		;;
	esac
}

rc_for() {
	case "$1" in
	zsh) echo "$H/.zshrc" ;;
	bash) if [ "$os" = Darwin ]; then echo "$H/.bash_profile"; else echo "$H/.bashrc"; fi ;;
	esac
}

blocks() { grep -c '^# >>> claude-profile >>>$' "$1" 2>/dev/null || true; }

has_fn() { # has_fn <shell> <name>
	in_shell "$1" "type $2" 2>/dev/null | grep -q function
}

if [ "$os" = Darwin ]; then
	entry() { echo "$H/Applications/Claude ($1).app/Contents/Info.plist"; }
else
	entry() { echo "$H/.local/share/applications/claude-desktop-$1.desktop"; }
fi

for sh in zsh bash; do
	if ! command -v "$sh" >/dev/null 2>&1; then
		step "$sh: not installed here, skipped"
		continue
	fi
	rc=$(rc_for "$sh")

	step "$sh: install"
	install_as "$sh"
	check "the installer wrote one block to ${rc#"$H"/}" test "$(blocks "$rc")" = 1
	install_as "$sh" >/dev/null
	check "a rerun still leaves one block" test "$(blocks "$rc")" = 1
	check "a new shell finds claude-profile in ~/.local/bin" \
		test "$(in_shell "$sh" 'command -v claude-profile' 2>/dev/null)" = "$H/.local/bin/claude-profile"
	check "the integration is loaded" \
		test "$(in_shell "$sh" 'echo $CLAUDE_PROFILE_SHELL' 2>/dev/null)" = 1
	check "claude-profile reports $version" \
		sh -c "'$H/.local/bin/claude-profile' --version | grep -qx 'claude-profile $version'"

	step "$sh: add"
	in_shell "$sh" 'claude-profile add e2e --label "e2e test"'
	check "~/.claude-e2e is registered" test -f "$H/.claude-e2e/profile.env"
	check "the Desktop entry exists" test -f "$(entry e2e)"
	check "the Desktop launcher exists" test -x "$H/.local/bin/claude-desktop-e2e"
	check "ls lists it" sh -c "HOME='$H' '$H/.local/bin/claude-profile' ls | grep -q '^e2e '"
	check "path names it" \
		test "$(clean_env "$H/.local/bin/claude-profile" path e2e)" = "$H/.claude-e2e"
	check "a new shell has claude-e2e" has_fn "$sh" claude-e2e
	check "a new shell has vse2e" has_fn "$sh" vse2e

	step "$sh: rename"
	in_shell "$sh" 'claude-profile rename e2e e2f'
	check "~/.claude-e2f exists" test -f "$H/.claude-e2f/profile.env"
	check "~/.claude-e2e is gone" test ! -e "$H/.claude-e2e"
	check "the Desktop entry moved" sh -c "test -f '$(entry e2f)' && test ! -e '$(entry e2e)'"
	check "a new shell has claude-e2f" has_fn "$sh" claude-e2f
	if has_fn "$sh" claude-e2e; then bad "claude-e2e survived the rename"; else ok "and no longer claude-e2e"; fi
	check "doctor runs clean for the new name" in_shell "$sh" 'claude-profile doctor e2f'

	step "$sh: rm"
	in_shell "$sh" 'claude-profile rm e2f --purge -y'
	check "~/.claude-e2f is gone" test ! -e "$H/.claude-e2f"
	check "the Desktop entry is gone" test ! -e "$(entry e2f)"
	check "the Desktop launcher is gone" test ! -e "$H/.local/bin/claude-desktop-e2f"
	if has_fn "$sh" claude-e2f; then bad "claude-e2f survived rm"; else ok "a new shell no longer has claude-e2f"; fi
done

if command -v zsh >/dev/null 2>&1; then
	rc=$(rc_for zsh)
	update() {
		clean_env SHELL="$(command -v zsh)" CLAUDE_PROFILE_BASE_URL="file://$work/rel" \
			CLAUDE_PROFILE_INSTALLER="$repo/install.sh" "$H/.local/bin/claude-profile" update "$@"
	}

	step "update"
	check "an install on the target release is up to date" \
		sh -c "$(command -v env) -i HOME='$H' '$H/.local/bin/claude-profile' update --version $version | grep -q 'up to date'"
	out=$(update --version "$version" --force 2>&1) || true
	printf '%s\n' "$out"
	check "--force reinstalls through the installer" sh -c "printf '%s' \"\$1\" | grep -q 'Reload your shell'" _ "$out"
	check "the binary still runs afterwards" "$H/.local/bin/claude-profile" --version
	check "the startup file still has one block" test "$(blocks "$rc")" = 1

	# Someone who installed with --no-modify-rc: an update must not add one.
	awk '/^# >>> claude-profile >>>$/{s=1} !s; /^# <<< claude-profile <<<$/{s=0}' "$rc" >"$work/rc" && cat "$work/rc" >"$rc"
	update --version "$version" --force >/dev/null 2>&1 || true
	check "an update never adds a block that was not there" test "$(blocks "$rc")" = 0
fi

printf '\n'
if [ "$failures" -gt 0 ]; then
	printf '%s check(s) failed\n' "$failures"
	exit 1
fi
printf 'all checks passed\n'
