#!/usr/bin/env bash
set -euo pipefail

REPO="${STACKCTL_INSTALL_REPO:-traweezy/stackctl}"
INSTALL_DIR="${STACKCTL_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${STACKCTL_INSTALL_VERSION:-}"
API_BASE_URL="${STACKCTL_INSTALL_API_BASE_URL:-https://api.github.com}"
DOWNLOAD_BASE_URL="${STACKCTL_INSTALL_DOWNLOAD_BASE_URL:-https://github.com}"
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

for cmd in curl tar mktemp install uname sha256sum sed head awk; do
  case "$cmd" in
    sha256sum)
      continue
      ;;
  esac
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
done

os="$(uname -s)"
case "$os" in
  Linux) os="Linux" ;;
  Darwin) os="Darwin" ;;
  *)
    echo "Unsupported OS: $os" >&2
    exit 1
    ;;
esac

sha256_tool=""
if [[ "$os" == "Darwin" ]]; then
  checksum_candidates=("shasum -a 256" sha256sum "openssl dgst -sha256")
else
  checksum_candidates=(sha256sum "shasum -a 256" "openssl dgst -sha256")
fi
for candidate in "${checksum_candidates[@]}"; do
  tool="${candidate%% *}"
  if command -v "$tool" >/dev/null 2>&1; then
    sha256_tool="$candidate"
    break
  fi
done

if [[ -z "$sha256_tool" ]]; then
  echo "Missing required checksum tool: sha256sum, shasum, or openssl" >&2
  exit 1
fi

compute_sha256() {
  local file="$1"
  case "$sha256_tool" in
    sha256sum)
      sha256sum "$file" | awk '{print $1}'
      ;;
    "shasum -a 256")
      shasum -a 256 "$file" | awk '{print $1}'
      ;;
    "openssl dgst -sha256")
      openssl dgst -sha256 "$file" | awk '{print $NF}'
      ;;
  esac
}

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
  api_url="${API_BASE_URL%/}/repos/${REPO}/releases/latest"
  VERSION="$(curl -fsSL "$api_url" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
fi

if [[ -z "$VERSION" ]]; then
  echo "Failed to determine the latest release tag for ${REPO}" >&2
  exit 1
fi

download_url="${DOWNLOAD_BASE_URL%/}/${REPO}/releases/download/${VERSION}/${asset}"
checksums_url="${DOWNLOAD_BASE_URL%/}/${REPO}/releases/download/${VERSION}/checksums.txt"
tmp_dir="$(mktemp -d)"
archive_path="${tmp_dir}/${asset}"
checksums_path="${tmp_dir}/checksums.txt"
binary_path="${tmp_dir}/stackctl"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

echo "Installing stackctl ${VERSION} for ${os}/${arch}..."
echo "Downloading ${checksums_url}"
curl -fsSL "$checksums_url" -o "$checksums_path"

expected_checksum="$(awk -v asset="$asset" '$2 == asset { print $1; exit }' "$checksums_path")"
if [[ -z "$expected_checksum" ]]; then
  echo "Release ${VERSION} does not publish ${asset}." >&2
  available_assets="$(awk '{print $2}' "$checksums_path" | grep '^stackctl_.*\.tar\.gz$' || true)"
  if [[ -n "$available_assets" ]]; then
    echo "Available archives in ${checksums_url}:" >&2
    while IFS= read -r available_asset; do
      echo "  - ${available_asset}" >&2
    done <<< "$available_assets"
  fi
  echo "Choose a newer release for this platform or build from source." >&2
  exit 1
fi

echo "Downloading ${download_url}"
curl -fsSL "$download_url" -o "$archive_path"

actual_checksum="$(compute_sha256 "$archive_path")"
if [[ "$actual_checksum" != "$expected_checksum" ]]; then
  echo "Checksum verification failed for ${asset}" >&2
  echo "Expected: ${expected_checksum}" >&2
  echo "Actual:   ${actual_checksum}" >&2
  exit 1
fi

echo "Verified archive checksum."

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
