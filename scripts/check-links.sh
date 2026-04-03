#!/usr/bin/env bash
set -euo pipefail

lychee_bin="${STACKCTL_LYCHEE_BIN:-lychee}"

if ! command -v "$lychee_bin" >/dev/null 2>&1; then
  echo "lychee is required; set STACKCTL_LYCHEE_BIN or add it to PATH" >&2
  exit 1
fi

if [[ -z "${GITHUB_TOKEN:-}" ]] && command -v gh >/dev/null 2>&1; then
  if gh auth status >/dev/null 2>&1; then
    export GITHUB_TOKEN
    GITHUB_TOKEN="$(gh auth token)"
  fi
fi

inputs=(README.md SECURITY.md CONTRIBUTING.md CHANGELOG.md)
mapfile -t doc_files < <(find docs -type f -name '*.md' ! -path 'docs/wiki-seed/_Sidebar.md' | sort)
inputs+=("${doc_files[@]}")

"$lychee_bin" \
  --no-progress \
  --include-fragments \
  --exclude 'raw\.githubusercontent\.com/traweezy/stackctl/\$\{STACKCTL_VERSION\}/scripts/install\.sh' \
  "${inputs[@]}"
