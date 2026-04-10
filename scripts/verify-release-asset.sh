#!/usr/bin/env bash
set -euo pipefail

REPO="${STACKCTL_VERIFY_RELEASE_REPO:-traweezy/stackctl}"
TAG="${STACKCTL_VERIFY_RELEASE_TAG:-}"
ASSET="${STACKCTL_VERIFY_RELEASE_ASSET:-}"
WORK_DIR="${STACKCTL_VERIFY_RELEASE_DIR:-}"
REQUIRE_ATTESTATIONS=0
REQUIRE_SIGSTORE_BUNDLE=0

usage() {
  cat <<'EOF'
Verify a released stackctl archive against the published checksum manifest and,
when present, the GitHub attestation and Sigstore bundle.

Usage:
  verify-release-asset.sh --tag TAG [options]

Options:
  --tag TAG                    Release tag to verify.
  --asset NAME                 Release asset name to verify.
  --repo OWNER/REPO            Override the GitHub repository.
  --dir DIR                    Download artifacts into DIR instead of a temp dir.
  --require-attestations       Fail if the release does not publish attestations.
  --require-sigstore-bundle    Fail if the release does not publish checksums.txt.sigstore.json.
  --help                       Show this help text.

Defaults:
  --asset defaults to the current host platform archive, for example:
  stackctl_Linux_x86_64.tar.gz
EOF
}

for cmd in gh awk grep uname mktemp; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
done

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag)
      TAG="${2:?missing value for --tag}"
      shift 2
      ;;
    --asset)
      ASSET="${2:?missing value for --asset}"
      shift 2
      ;;
    --repo)
      REPO="${2:?missing value for --repo}"
      shift 2
      ;;
    --dir)
      WORK_DIR="${2:?missing value for --dir}"
      shift 2
      ;;
    --require-attestations)
      REQUIRE_ATTESTATIONS=1
      shift
      ;;
    --require-sigstore-bundle)
      REQUIRE_SIGSTORE_BUNDLE=1
      shift
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

if [[ -z "$TAG" ]]; then
  echo "--tag is required" >&2
  usage >&2
  exit 1
fi

infer_asset_name() {
  local os arch

  os="$(uname -s)"
  case "$os" in
    Linux) os="Linux" ;;
    Darwin) os="Darwin" ;;
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

  printf 'stackctl_%s_%s.tar.gz\n' "$os" "$arch"
}

sha256_tool=""
if [[ "$(uname -s)" == "Darwin" ]]; then
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

if [[ -z "$ASSET" ]]; then
  ASSET="$(infer_asset_name)"
fi

release_assets="$(gh release view "$TAG" --repo "$REPO" --json assets --jq '.assets[].name')"

if ! printf '%s\n' "$release_assets" | grep -Fx "$ASSET" >/dev/null 2>&1; then
  echo "Release ${TAG} does not publish ${ASSET} in ${REPO}." >&2
  available_archives="$(printf '%s\n' "$release_assets" | grep '^stackctl_.*\.tar\.gz$' || true)"
  if [[ -n "$available_archives" ]]; then
    echo "Available archives:" >&2
    while IFS= read -r available_archive; do
      echo "  - ${available_archive}" >&2
    done <<< "$available_archives"
  fi
  echo "Choose another asset, a newer release, or build from source." >&2
  exit 1
fi

cleanup() {
  if [[ -n "${temp_dir:-}" ]]; then
    rm -rf "$temp_dir"
  fi
}

if [[ -z "$WORK_DIR" ]]; then
  temp_dir="$(mktemp -d)"
  WORK_DIR="$temp_dir"
  trap cleanup EXIT
else
  mkdir -p "$WORK_DIR"
fi

checksums_path="${WORK_DIR}/checksums.txt"
asset_path="${WORK_DIR}/${ASSET}"
bundle_path="${WORK_DIR}/checksums.txt.sigstore.json"

echo "Downloading checksums and ${ASSET} from ${REPO}@${TAG}..."
gh release download "$TAG" --repo "$REPO" --dir "$WORK_DIR" --clobber \
  -p 'checksums.txt' \
  -p "$ASSET"

expected_checksum="$(awk -v asset="$ASSET" '$2 == asset { print $1; exit }' "$checksums_path")"
if [[ -z "$expected_checksum" ]]; then
  echo "Could not find ${ASSET} in ${checksums_path}" >&2
  exit 1
fi

actual_checksum="$(compute_sha256 "$asset_path")"
if [[ "$actual_checksum" != "$expected_checksum" ]]; then
  echo "Checksum verification failed for ${ASSET}" >&2
  echo "Expected: ${expected_checksum}" >&2
  echo "Actual:   ${actual_checksum}" >&2
  exit 1
fi

echo "Verified archive checksum."

if printf '%s\n' "$release_assets" | grep -Fx 'checksums.txt.sigstore.json' >/dev/null 2>&1; then
  if ! command -v cosign >/dev/null 2>&1; then
    echo "Missing required command for Sigstore verification: cosign" >&2
    exit 1
  fi
  gh release download "$TAG" --repo "$REPO" --dir "$WORK_DIR" --clobber \
    -p 'checksums.txt.sigstore.json'
  cosign verify-blob \
    --bundle "$bundle_path" \
    --certificate-identity "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/${TAG}" \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    "$checksums_path"
  echo "Verified Sigstore bundle for checksums.txt."
else
  if [[ "$REQUIRE_SIGSTORE_BUNDLE" -eq 1 ]]; then
    echo "Release ${TAG} does not publish checksums.txt.sigstore.json." >&2
    exit 1
  fi
  echo "Skipping Sigstore bundle verification: release does not publish checksums.txt.sigstore.json."
fi

attestation_output=""
if attestation_output="$(gh release verify-asset "$TAG" "$asset_path" --repo "$REPO" 2>&1)"; then
  echo "Verified GitHub artifact attestation."
else
  if printf '%s\n' "$attestation_output" | grep -q 'no attestations found'; then
    if [[ "$REQUIRE_ATTESTATIONS" -eq 1 ]]; then
      printf '%s\n' "$attestation_output" >&2
      exit 1
    fi
    echo "Skipping GitHub artifact attestation verification: release does not publish attestations."
  else
    printf '%s\n' "$attestation_output" >&2
    exit 1
  fi
fi

echo "Release verification completed for ${REPO}@${TAG}: ${ASSET}"
echo "Artifacts stored in ${WORK_DIR}"
