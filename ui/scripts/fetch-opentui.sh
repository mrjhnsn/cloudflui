#!/usr/bin/env sh
# Downloads the OpenTUI native library for the current platform into
# ui/vendor/opentui-native. The Rust crate links this library; see
# ui/.cargo/config.toml and https://opentui.com/docs/bindings/rust/install.
set -eu

VERSION="0.9.9"

# Keep this in sync with the `opentui` dependency in ui/Cargo.toml.
os="$(uname -s)"
arch="$(uname -m)"

case "$os:$arch" in
  Darwin:arm64)  asset="darwin-arm64" ;;
  Darwin:x86_64) asset="darwin-x64" ;;
  Linux:x86_64)  asset="linux-x64" ;;
  Linux:aarch64) asset="linux-arm64" ;;
  *)
    echo "unsupported platform: $os $arch" >&2
    echo "download opentui-native-v$VERSION-<platform>.zip from" >&2
    echo "https://github.com/anomalyco/opentui/releases/tag/v$VERSION" >&2
    exit 1
    ;;
esac

name="opentui-native-v${VERSION}-${asset}.zip"
url="https://github.com/anomalyco/opentui/releases/download/v${VERSION}/${name}"
dest="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)/vendor/opentui-native"

mkdir -p "$dest"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "fetching $name"
curl -fsSL "$url" -o "$tmp/$name"
unzip -oq "$tmp/$name" -d "$dest"

echo "installed to $dest"
