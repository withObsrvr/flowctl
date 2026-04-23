#!/usr/bin/env sh
set -eu

REPO="withObsrvr/flowctl"
BIN="flowctl"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${VERSION:-latest}"
TAG_VERSION=""
ASSET_VERSION=""

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

need curl
need tar
need uname
need mktemp
need install

checksum_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    echo "shasum -a 256"
    return 0
  fi
  echo ""
  return 1
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  linux|darwin) ;;
  *)
    echo "error: unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  TAG_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | \
    grep '"tag_name":' | head -n1 | sed -E 's/.*"([^"]+)".*/\1/')
  ASSET_VERSION="${TAG_VERSION#v}"
else
  case "$VERSION" in
    v*)
      TAG_VERSION="$VERSION"
      ASSET_VERSION="${VERSION#v}"
      ;;
    *)
      TAG_VERSION="v$VERSION"
      ASSET_VERSION="$VERSION"
      ;;
  esac
fi

if [ -z "$TAG_VERSION" ] || [ -z "$ASSET_VERSION" ]; then
  echo "error: failed to resolve release version" >&2
  exit 1
fi

archive="${BIN}_${ASSET_VERSION}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${TAG_VERSION}/${archive}"
checksum_url="https://github.com/${REPO}/releases/download/${TAG_VERSION}/checksums.txt"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

echo "Installing ${BIN} ${TAG_VERSION} for ${os}/${arch}..."
echo "Downloading ${url}"

curl -fL "$url" -o "$tmpdir/$archive"
curl -fL "$checksum_url" -o "$tmpdir/checksums.txt"

checksum_tool=$(checksum_cmd || true)
if [ -z "$checksum_tool" ]; then
  echo "error: neither sha256sum nor shasum is available for checksum verification" >&2
  exit 1
fi

expected=$(grep "  ${archive}\$" "$tmpdir/checksums.txt" | awk '{print $1}')
if [ -z "$expected" ]; then
  echo "error: checksum for ${archive} not found in checksums.txt" >&2
  exit 1
fi

actual=$(eval "$checksum_tool \"$tmpdir/$archive\"" | awk '{print $1}')
if [ "$expected" != "$actual" ]; then
  echo "error: checksum verification failed for ${archive}" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
tar -xzf "$tmpdir/$archive" -C "$tmpdir"
install "$tmpdir/$BIN" "$INSTALL_DIR/$BIN"

echo "Installed to $INSTALL_DIR/$BIN"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "Add this to your shell profile if needed:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo ""
echo "Next steps:"
echo "  flowctl init --preset testnet-duckdb"
echo "  flowctl run stellar-pipeline.yaml"
