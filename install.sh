#!/bin/sh
# claude-profile installer.
#
#   curl -fsSL https://raw.githubusercontent.com/aymenkrifa/claude-profile/main/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --prefix /usr/local --version v1.0.0
#
# Downloads the release archive for this platform, verifies it against the
# release's checksums, and installs the binary plus the shell integrations under
# --prefix (default ~/.local), then hooks the integration into the startup file
# of the shell $SHELL names. That one marked block is the only thing written
# outside the prefix (--no-modify-rc skips it), and nothing is run with sudo; if
# the prefix is not writable the install fails and says so rather than
# escalating.
#
# Written for POSIX sh, not bash: macOS still ships bash 3.2 and /bin/sh there
# is not what a `curl | sh` reader would expect it to be.
set -eu

REPO="aymenkrifa/claude-profile"
# Overridable so the installer can be tested against a local archive, or
# pointed at a mirror.
BASE_URL="${CLAUDE_PROFILE_BASE_URL:-https://github.com/$REPO/releases/download}"
API_URL="${CLAUDE_PROFILE_API_URL:-https://api.github.com/repos/$REPO/releases/latest}"

prefix="${PREFIX:-$HOME/.local}"
version=""
want_shell=1
modify_rc=1

say()  { printf '%s\n' "$*"; }
warn() { printf 'install.sh: warning: %s\n' "$*" >&2; }
die()  { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

usage() {
	cat <<'USAGE'
usage: install.sh [--prefix DIR] [--version vX.Y.Z] [--no-shell] [--no-modify-rc]

  --prefix DIR     install under DIR (default: ~/.local)
  --version TAG    install a specific release (default: the latest)
  --no-shell       skip the shell integrations, install only the binary
  --no-modify-rc   install the integrations but leave your shell's startup
                   file alone ('claude-profile shell-init' prints the line)
  -h, --help       this message
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
	--prefix) [ $# -ge 2 ] || die "--prefix needs a directory"; prefix="$2"; shift 2 ;;
	--prefix=*) prefix="${1#--prefix=}"; shift ;;
	--version) [ $# -ge 2 ] || die "--version needs a tag"; version="$2"; shift 2 ;;
	--version=*) version="${1#--version=}"; shift ;;
	--no-shell) want_shell=0; shift ;;
	--no-modify-rc) modify_rc=0; shift ;;
	-h | --help) usage; exit 0 ;;
	*) die "unknown option $1 (try --help)" ;;
	esac
done

# --- what are we ---------------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
linux | darwin) ;;
*) die "claude-profile supports Linux and macOS; this is $os" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "no build for $arch (amd64 and arm64 only)" ;;
esac

if ! have curl && ! have wget; then
	die "need curl or wget"
fi

fetch() { # fetch <url> <dest>
	if have curl; then
		curl -fsSL "$1" -o "$2"
	else
		wget -qO "$2" "$1"
	fi
}

sha256_of() {
	if have sha256sum; then
		sha256sum "$1" | cut -d' ' -f1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		printf ''
	fi
}

# --- which release -------------------------------------------------------
if [ -z "$version" ]; then
	tmp_json=$(mktemp) || die "cannot create a temporary file"
	if fetch "$API_URL" "$tmp_json" 2>/dev/null; then
		version=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp_json" | head -1)
	fi
	rm -f "$tmp_json"
fi
[ -n "$version" ] || die "could not work out the latest version; pass --version vX.Y.Z"

asset="claude-profile_${version}_${os}_${arch}.tar.gz"
say "claude-profile $version ($os/$arch)"

work=$(mktemp -d) || die "cannot create a temporary directory"
# PIPE and HUP are in the list because without them a reader that stops early
# -- `install.sh | head` -- kills the script before the EXIT trap runs and
# leaves the download behind.
# shellcheck disable=SC2064  # $work is intentionally expanded now, not later
trap "rm -rf '$work'" EXIT HUP INT PIPE TERM

fetch "$BASE_URL/$version/$asset" "$work/$asset" ||
	die "no release archive at $BASE_URL/$version/$asset"

# --- verify --------------------------------------------------------------
if fetch "$BASE_URL/$version/checksums.txt" "$work/checksums.txt" 2>/dev/null; then
	want=$(grep "  *$asset\$" "$work/checksums.txt" | cut -d' ' -f1 | head -1)
	got=$(sha256_of "$work/$asset")
	if [ -z "$want" ]; then
		warn "$asset is not listed in checksums.txt; continuing unverified"
	elif [ -z "$got" ]; then
		warn "no sha256 tool found; continuing unverified"
	elif [ "$want" != "$got" ]; then
		die "checksum mismatch for $asset
  expected $want
  got      $got"
	else
		say "checksum ok"
	fi
else
	warn "no checksums.txt for $version; continuing unverified"
fi

# --- install -------------------------------------------------------------
tar -xzf "$work/$asset" -C "$work" || die "could not unpack $asset"
src="$work/claude-profile_${version}_${os}_${arch}"
[ -f "$src/claude-profile" ] || die "$asset does not contain a claude-profile binary"

mkdir -p "$prefix/bin" || die "cannot create $prefix/bin"
install -m 0755 "$src/claude-profile" "$prefix/bin/claude-profile" ||
	die "cannot write $prefix/bin/claude-profile"
say "installed $prefix/bin/claude-profile"

# All of the shell integrations, not just the current shell's: they are a few
# kilobytes each, and it means 'shell-init bash' can answer on a machine set up
# from a fish shell.
if [ "$want_shell" = 1 ]; then
	shells=""
	for f in "$src"/claude-profile.*; do
		[ -f "$f" ] || continue
		mkdir -p "$prefix/share/claude-profile"
		install -m 0644 "$f" "$prefix/share/claude-profile/${f##*/}"
		shells="${shells:+$shells }${f##*.}"
	done
	[ -n "$shells" ] && say "installed $prefix/share/claude-profile/ ($shells)"
fi

# --- hook up the shell --------------------------------------------------
# The block between the markers belongs to the installer: a rerun replaces it,
# so a new --prefix lands in place of the old one, and deleting it undoes the
# hookup. A source line written by hand is left alone instead, so a shell set
# up before the installer did this is not given the integration twice.
begin_mark="# >>> claude-profile >>>"
end_mark="# <<< claude-profile <<<"

# $HOME spelled as $HOME, so the startup file still reads right if it is
# synced to a machine with a different home directory.
home_rel() {
	case "$1" in
	"$HOME"/*) printf '%s' "\$HOME/${1#"$HOME"/}" ;;
	*) printf '%s' "$1" ;;
	esac
}
tilde() {
	case "$1" in
	"$HOME"/*) printf '~/%s' "${1#"$HOME"/}" ;;
	*) printf '%s' "$1" ;;
	esac
}

login_shell=$(basename "${SHELL:-sh}")
rc=""
case "$login_shell" in
zsh) rc="${ZDOTDIR:-$HOME}/.zshrc" ;;
# macOS terminals start bash as a login shell, which reads .bash_profile and
# never .bashrc.
bash) if [ "$os" = darwin ]; then rc="$HOME/.bash_profile"; else rc="$HOME/.bashrc"; fi ;;
fish) rc="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
esac

hooked=""      # "added", "updated" or "by hand" once the startup file loads it
if [ "$want_shell" = 1 ] && [ "$modify_rc" = 1 ] && [ -n "$rc" ]; then
	bin_rel=$(home_rel "$prefix/bin")
	integ_rel=$(home_rel "$prefix/share/claude-profile/claude-profile.$login_shell")
	if [ "$login_shell" = fish ]; then
		path_line="contains -- \"$bin_rel\" \$PATH; or set -gx PATH \"$bin_rel\" \$PATH"
		source_line="test -f \"$integ_rel\"; and source \"$integ_rel\""
	else
		path_line="case \":\$PATH:\" in *\":$bin_rel:\"*) ;; *) export PATH=\"$bin_rel:\$PATH\" ;; esac"
		source_line="[ -f \"$integ_rel\" ] && source \"$integ_rel\""
	fi

	# Everything but an earlier block, and the blank line that preceded it.
	rest="$work/rc.rest"
	if [ -f "$rc" ]; then
		grep -qxF "$begin_mark" "$rc" && had_block=1 || had_block=0
		awk -v b="$begin_mark" -v e="$end_mark" '
			$0 == b { skip = 1; blanks = ""; next }
			skip { if ($0 == e) skip = 0; next }
			/^[ \t]*$/ { blanks = blanks $0 "\n"; next }
			{ printf "%s", blanks; blanks = ""; print }
			END { printf "%s", blanks }
		' "$rc" >"$rest"
	else
		had_block=0
		: >"$rest"
	fi

	if grep -q "claude-profile\.$login_shell" "$rest"; then
		hooked="by hand"
	else
		{
			cat "$rest"
			[ -s "$rest" ] && [ -n "$(tail -n 1 "$rest")" ] && echo
			echo "$begin_mark"
			echo "# Written by the claude-profile installer; delete this block to undo."
			echo "# Set any CLAUDE_PROFILE_* options above it ('claude-profile shell-init')."
			echo "$path_line"
			echo "$source_line"
			echo "$end_mark"
		} >"$work/rc.new"
		# cat rather than mv: keeps the file's mode, and keeps a symlinked
		# startup file (a dotfiles repo) a symlink.
		if mkdir -p "$(dirname "$rc")" && cat "$work/rc.new" >"$rc"; then
			[ "$had_block" = 1 ] && hooked="updated" || hooked="added"
		else
			warn "could not write $rc; add the line from 'claude-profile shell-init' yourself"
		fi
	fi
fi

# --- what now ------------------------------------------------------------
if [ -t 1 ]; then bold=$(printf '\033[1m'); off=$(printf '\033[0m'); else bold=""; off=""; fi

if [ -n "$hooked" ]; then
	case "$hooked" in
	added) say "added claude-profile to $(tilde "$rc")" ;;
	updated) say "updated claude-profile in $(tilde "$rc")" ;;
	"by hand") say "$(tilde "$rc") already loads claude-profile; left it as it is" ;;
	esac
	say ""
	say "${bold}Reload your shell before using claude-profile:${off}"
	say ""
	say "    ${bold}exec $login_shell${off}"
	say ""
	say "or close this terminal and open a new one. Then:"
	say ""
	say "    claude-profile add <name>      # create your first account"
	exit 0
fi

case ":$PATH:" in
*":$prefix/bin:"*) ;;
*) warn "$prefix/bin is not on your PATH -- add it, or the shell will not find claude-profile" ;;
esac

if [ "$want_shell" = 1 ] && [ "$modify_rc" = 1 ] && [ -z "$rc" ]; then
	warn "no integration for ${SHELL:-an unset \$SHELL}; zsh, bash and fish are supported"
fi

say ""
say "next:"
if [ "$want_shell" = 1 ]; then
	say "  claude-profile shell-init    # the line to add to your shell startup file"
fi
say "  claude-profile add <name>    # create your first account"
