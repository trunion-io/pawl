#!/bin/sh
# pawl installer (PAWL-013 AC14–AC18).
#
#   sh install.sh                    latest, into ~/.local/bin
#   PAWL_VERSION=0.1.0 sh install.sh a pinned version
#   PAWL_BIN_DIR=/usr/local/bin sh install.sh
#
# You are encouraged to read this before running it. `curl … | sh` is a strange
# thing for a supply-chain assurance tool to ask for — it runs code you have not
# read — so downloading it first works exactly as well, and the documentation
# shows that path first.
#
# What it does: works out your platform, downloads the release binary and the
# published SHA256SUMS, refuses to install unless they match, and tells you if
# the install directory is not on your PATH.

set -eu

REPO="trunion-io/pawl"
BIN_DIR="${PAWL_BIN_DIR:-$HOME/.local/bin}"

say()  { printf '%s\n' "$*"; }
die()  { printf 'install: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required"; }
need uname
need mkdir
need install

# One of these must exist; both are near-universal.
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1" -o "$2"; }
  fetch_stdout() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO "$2" "$1"; }
  fetch_stdout() { wget -qO- "$1"; }
else
  die "curl or wget is required"
fi

# --- platform -------------------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  mingw*|msys*|cygwin*) die "on Windows, download the .exe from https://github.com/$REPO/releases" ;;
  *) die "unsupported OS: $os" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac

# --- version --------------------------------------------------------------
version="${PAWL_VERSION:-}"
if [ -z "$version" ]; then
  say "resolving the latest release…"
  version=$(fetch_stdout "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name" *: *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$version" ] || die "could not determine the latest release; set PAWL_VERSION"
fi

asset="pawl-${version}-${os}-${arch}"
base="https://github.com/$REPO/releases/download/v${version}"

tmp=$(mktemp -d)
# shellcheck disable=SC2064
trap "rm -rf '$tmp'" EXIT INT TERM

say "downloading ${asset}…"
fetch "$base/$asset" "$tmp/$asset" || die "no such release asset: $asset"
fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS" || die "release has no SHA256SUMS; refusing to install unverified"

# --- verify ---------------------------------------------------------------
# AC15. Not optional: pawl's whole argument is that you can verify what you were
# given, and an installer that skipped this would contradict the pitch at the
# first moment anyone met the tool.
expected=$(grep " \{1,2\}\*\{0,1\}${asset}\$" "$tmp/SHA256SUMS" | awk '{print $1}' | head -n 1)
[ -n "$expected" ] || die "$asset is not listed in SHA256SUMS; refusing to install"

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
else
  die "sha256sum or shasum is required to verify the download"
fi

if [ "$expected" != "$actual" ]; then
  die "checksum mismatch for $asset
  expected $expected
  got      $actual
Refusing to install. Report this — a mismatch is a finding, not a glitch."
fi
say "checksum verified"

# --- install --------------------------------------------------------------
mkdir -p "$BIN_DIR" || die "cannot create $BIN_DIR"
install -m 0755 "$tmp/$asset" "$BIN_DIR/pawl" || die "cannot write to $BIN_DIR"
say "installed pawl ${version} to $BIN_DIR/pawl"

# --- PATH -----------------------------------------------------------------
# AC16. This exact silence has already cost a working day here: a hook naming a
# binary nothing could find, failing quietly on every edit.
case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *)
    say ""
    say "WARNING: $BIN_DIR is not on your PATH."
    say "Nothing that runs \`pawl\` will find it, and some of those things fail"
    say "silently. Add this to your shell profile:"
    say ""
    say "    export PATH=\"$BIN_DIR:\$PATH\""
    ;;
esac

say ""
"$BIN_DIR/pawl" version
say ""
say "Next: pawl setup claude    (installs the accounting hook)"
say "      pawl setup claude --check    (confirms it actually works)"
