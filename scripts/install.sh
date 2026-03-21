#!/usr/bin/env bash
set -euo pipefail

REPO="${STACKCTL_INSTALL_REPO:-traweezy/stackctl}"
INSTALL_DIR="${STACKCTL_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${STACKCTL_INSTALL_VERSION:-}"
USE_SUDO=0

usage() {
  cat <<'EOF'
Install stackctl from GitHub Releases.

Usage:
  install.sh [--system] [--dir DIR] [--version TAG] [--repo OWNER/REPO]

Options:
  --system       Install to /usr/local/bin using sudo.
  --dir DIR      Install to a custom directory.
  --version TAG  Install a specific release tag instead of the latest release.
  --repo REPO    Override the GitHub repository (default: traweezy/stackctl).
  --help         Show this help text.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --system)
      INSTALL_DIR="/usr/local/bin"
      USE_SUDO=1
      shift
      ;;
    --dir)
      INSTALL_DIR="${2:?missing value for --dir}"
      USE_SUDO=0
      shift 2
      ;;
    --version)
      VERSION="${2:?missing value for --version}"
      shift 2
      ;;
    --repo)
      REPO="${2:?missing value for --repo}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

for cmd in curl tar mktemp install uname; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
done

os="$(uname -s)"
case "$os" in
  Linux) os="Linux" ;;
  *)
    echo "Unsupported OS: $os" >&2
    exit 1
    ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="x86_64" ;;
  aarch64|arm64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

asset="stackctl_${os}_${arch}.tar.gz"

if [[ -z "$VERSION" ]]; then
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  VERSION="$(curl -fsSL "$api_url" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
fi

if [[ -z "$VERSION" ]]; then
  echo "Failed to determine the latest release tag for ${REPO}" >&2
  exit 1
fi

download_url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
tmp_dir="$(mktemp -d)"
archive_path="${tmp_dir}/${asset}"
binary_path="${tmp_dir}/stackctl"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

echo "Installing stackctl ${VERSION} for ${os}/${arch}..."
echo "Downloading ${download_url}"
curl -fsSL "$download_url" -o "$archive_path"

tar -xzf "$archive_path" -C "$tmp_dir"

if [[ ! -f "$binary_path" ]]; then
  echo "Downloaded archive did not contain a stackctl binary" >&2
  exit 1
fi

if [[ "$USE_SUDO" -eq 1 ]]; then
  sudo mkdir -p "$INSTALL_DIR"
  sudo install -m 0755 "$binary_path" "$INSTALL_DIR/stackctl"
else
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$binary_path" "$INSTALL_DIR/stackctl"
fi

echo "Installed to ${INSTALL_DIR}/stackctl"

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "Add this to your shell config:"
  echo "export PATH=\"$INSTALL_DIR:\$PATH\""
fi

echo "Done."
