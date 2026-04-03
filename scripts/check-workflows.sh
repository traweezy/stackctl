#!/usr/bin/env bash
set -euo pipefail

actionlint_bin="${STACKCTL_ACTIONLINT_BIN:-actionlint}"
shellcheck_bin="${STACKCTL_SHELLCHECK_BIN:-}"

if ! command -v "$actionlint_bin" >/dev/null 2>&1; then
  echo "actionlint is required; set STACKCTL_ACTIONLINT_BIN or add it to PATH" >&2
  exit 1
fi

actionlint_args=()
if [[ -n "$shellcheck_bin" ]]; then
  if ! command -v "$shellcheck_bin" >/dev/null 2>&1; then
    echo "Configured shellcheck binary was not found: $shellcheck_bin" >&2
    exit 1
  fi
  actionlint_args+=("-shellcheck=$shellcheck_bin")
elif command -v shellcheck >/dev/null 2>&1; then
  actionlint_args+=("-shellcheck=shellcheck")
fi

"$actionlint_bin" "${actionlint_args[@]}" .github/workflows/*.yml
