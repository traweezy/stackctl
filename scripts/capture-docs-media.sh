#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_path="${STACKCTL_DOCS_TUI_SCREENSHOT_PATH:-$repo_root/docs/media/tui-services.png}"
window_title="${STACKCTL_DOCS_TUI_WINDOW_TITLE:-stackctl-docs-tui}"
window_geometry="${STACKCTL_DOCS_TUI_WINDOW_GEOMETRY:-144x44}"
window_font="${STACKCTL_DOCS_TUI_WINDOW_FONT:-DejaVu Sans Mono}"
window_font_size="${STACKCTL_DOCS_TUI_WINDOW_FONT_SIZE:-11}"
capture_delay_seconds="${STACKCTL_DOCS_TUI_CAPTURE_DELAY_SECONDS:-2}"
capture_duration="${STACKCTL_DOCS_TUI_CAPTURE_DURATION:-10s}"
output_width="${STACKCTL_DOCS_TUI_SCREENSHOT_WIDTH:-1600}"

require_bin() {
  local bin="$1"
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "$bin is required to capture docs media" >&2
    exit 1
  fi
}

require_bin go
require_bin xterm
require_bin xwininfo
require_bin import
require_bin convert

mkdir -p "$(dirname "$output_path")"

xterm_pid=""

cleanup() {
  if [ -n "$xterm_pid" ] && kill -0 "$xterm_pid" >/dev/null 2>&1; then
    kill "$xterm_pid" >/dev/null 2>&1 || true
    wait "$xterm_pid" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

xterm \
  -T "$window_title" \
  -geometry "$window_geometry" \
  -fa "$window_font" \
  -fs "$window_font_size" \
  -bg "#0f1117" \
  -fg "#e6edf3" \
  -e bash -lc "cd \"$repo_root\" && STACKCTL_DOCS_TUI_CAPTURE_DURATION=\"$capture_duration\" go run -tags docs_capture ./scripts/render-docs-tui.go" &
xterm_pid=$!

window_id=""
for _ in $(seq 1 100); do
  if window_id="$(xwininfo -frame -name "$window_title" 2>/dev/null | awk '/Window id:/ { print $4; exit }')"; then
    if [ -n "$window_id" ]; then
      break
    fi
  fi
  sleep 0.1
done

if [ -z "$window_id" ]; then
  echo "could not find xterm window titled $window_title" >&2
  exit 1
fi

sleep "$capture_delay_seconds"
import -window "$window_id" "$output_path"
if [ "$output_width" -gt 0 ]; then
  convert "$output_path" -strip -resize "${output_width}x" "$output_path"
else
  convert "$output_path" -strip "$output_path"
fi

echo "Captured docs screenshot: $output_path"
