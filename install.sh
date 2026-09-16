#!/bin/sh
# claude-profile installer.
#
#   curl -fsSL https://raw.githubusercontent.com/aymenkrifa/claude-profile/main/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --prefix /usr/local --version v1.0.0
#
# Downloads the release archive for this platform, verifies it against the
# release's checksums, and installs the binary plus the shell integrations under
# --prefix (default ~/.local). Nothing is installed outside that prefix and
# nothing is run with sudo; if the prefix is not writable the install fails and
# says so rather than escalating.
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

say()  { printf '%s\n' "$*"; }
warn() { printf 'install.sh: warning: %s\n' "$*" >&2; }
die()  { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

usage() {
	cat <<'USAGE'
usage: install.sh [--prefix DIR] [--version vX.Y.Z] [--no-shell]

  --prefix DIR     install under DIR (default: ~/.local)
  --version TAG    install a specific release (default: the latest)
  --no-shell       skip the shell integrations, install only the binary
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

# --- what now ------------------------------------------------------------
case ":$PATH:" in
*":$prefix/bin:"*) ;;
*) warn "$prefix/bin is not on your PATH -- add it, or the shell will not find claude-profile" ;;
esac

say ""
say "next:"
if [ "$want_shell" = 1 ]; then
	say "  claude-profile shell-init    # the line to add to your shell startup file"
fi
say "  claude-profile add personal  # create your first account"
