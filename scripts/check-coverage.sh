#!/usr/bin/env bash
set -euo pipefail

threshold="${STACKCTL_COVERAGE_THRESHOLD:-${1:-100.0}}"
profile_path="${STACKCTL_COVERAGE_PROFILE:-${2:-tmp/coverage.out}}"

mkdir -p "$(dirname "$profile_path")"

go test ./... -coverprofile="$profile_path" -count=1

total="$(go tool cover -func="$profile_path" | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')"
if [[ -z "$total" ]]; then
  echo "Failed to determine total coverage from $profile_path" >&2
  exit 1
fi

printf 'Total coverage: %s%% (threshold %s%%)\n' "$total" "$threshold"

if ! awk -v actual="$total" -v threshold="$threshold" 'BEGIN { exit (actual + 0 >= threshold + 0 ? 0 : 1) }'; then
  echo "Coverage gate failed: ${total}% is below ${threshold}%." >&2
  exit 1
fi
