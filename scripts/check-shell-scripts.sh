#!/usr/bin/env bash
set -euo pipefail

shellcheck_bin="${STACKCTL_SHELLCHECK_BIN:-shellcheck}"

if ! command -v "$shellcheck_bin" >/dev/null 2>&1; then
  echo "shellcheck is required; set STACKCTL_SHELLCHECK_BIN or add it to PATH" >&2
  exit 1
fi

mapfile -t script_files < <(
  {
    find scripts -maxdepth 1 -type f -name '*.sh'
    find .clusterfuzzlite -maxdepth 1 -type f -name '*.sh'
  } | sort
)
if [[ "${#script_files[@]}" -eq 0 ]]; then
  echo "No shell scripts found under scripts/ or .clusterfuzzlite/" >&2
  exit 1
fi

"$shellcheck_bin" "${script_files[@]}"
