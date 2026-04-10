#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
container_image="${STACKCTL_VHS_IMAGE:-ghcr.io/charmbracelet/vhs:v0.11.0}"
container_engine="${STACKCTL_VHS_ENGINE:-}"
tape_path="$repo_root/examples/vhs/help.tape"
output_path=""
binary_path=""
build_binary=1
render_binary_path="$repo_root/dist/stackctl"

usage() {
  cat <<'EOF'
Render a reproducible stackctl demo with Charm VHS in a pinned container image.

Usage:
  render-vhs-demo.sh [options]

Options:
  --tape PATH     Tape to render (default: examples/vhs/help.tape).
  --output PATH   Override the tape Output path. PATH must stay under the repo.
  --binary PATH   Use an existing binary instead of building ./dist/stackctl.
  --no-build      Skip the default go build step and use the tape as-is.
  --engine NAME   Container engine to use (podman or docker).
  --image IMAGE   Override the pinned VHS image.
  --help          Show this help text.
EOF
}

fail() {
  echo "$1" >&2
  exit 1
}

require_bin() {
  local bin="$1"
  command -v "$bin" >/dev/null 2>&1 || fail "Missing required command: $bin"
}

contains_parent_traversal() {
  local path="$1"
  [[ "/$path/" == *"/../"* ]]
}

resolve_repo_path() {
  local path="$1"
  if [[ "$path" = /* ]]; then
    printf '%s\n' "$path"
    return 0
  fi
  contains_parent_traversal "$path" && fail "Path must stay under the repo: $path"
  printf '%s/%s\n' "$repo_root" "$path"
}

resolve_any_path() {
  local path="$1"
  if [[ "$path" = /* ]]; then
    printf '%s\n' "$path"
    return 0
  fi
  printf '%s/%s\n' "$repo_root" "$path"
}

ensure_within_repo() {
  local path="$1"
  case "$path" in
    "$repo_root"/*|"$repo_root") ;;
    *)
      fail "Path must stay under the repo root: $path"
      ;;
  esac
}

select_engine() {
  if [[ -n "$container_engine" ]]; then
    require_bin "$container_engine"
    return 0
  fi
  if command -v podman >/dev/null 2>&1; then
    container_engine="podman"
    return 0
  fi
  if command -v docker >/dev/null 2>&1; then
    container_engine="docker"
    return 0
  fi
  fail "Missing required container engine: podman or docker"
}

prepare_binary() {
  if [[ -n "$binary_path" ]]; then
    [[ -f "$binary_path" ]] || fail "Binary path does not exist: $binary_path"
    [[ -x "$binary_path" ]] || fail "Binary path is not executable: $binary_path"
    return 0
  fi

  if [[ "$build_binary" -eq 1 ]]; then
    require_bin go
    mkdir -p "$(dirname "$render_binary_path")"
    echo "Building $render_binary_path"
    (cd "$repo_root" && go build -trimpath -o "$render_binary_path" .)
    return 0
  fi

  [[ -f "$render_binary_path" ]] || fail "Expected prebuilt binary at $render_binary_path"
  [[ -x "$render_binary_path" ]] || fail "Expected executable binary at $render_binary_path"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tape)
      tape_path="$(resolve_repo_path "${2:?missing value for --tape}")"
      shift 2
      ;;
    --output)
      output_path="$(resolve_repo_path "${2:?missing value for --output}")"
      shift 2
      ;;
    --binary)
      binary_path="$(resolve_any_path "${2:?missing value for --binary}")"
      build_binary=0
      shift 2
      ;;
    --no-build)
      build_binary=0
      shift
      ;;
    --engine)
      container_engine="${2:?missing value for --engine}"
      shift 2
      ;;
    --image)
      container_image="${2:?missing value for --image}"
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

ensure_within_repo "$tape_path"
[[ -f "$tape_path" ]] || fail "Tape path does not exist: $tape_path"

default_output_rel="$(awk '$1 == "Output" { print $2; exit }' "$tape_path")"
[[ -n "$default_output_rel" ]] || fail "Tape is missing an Output directive: $tape_path"
contains_parent_traversal "$default_output_rel" && fail "Tape Output must stay under the repo: $default_output_rel"

if [[ -z "$output_path" ]]; then
  output_path="$repo_root/$default_output_rel"
fi
ensure_within_repo "$output_path"

select_engine
prepare_binary

output_rel="${output_path#"$repo_root"/}"
mkdir -p "$(dirname "$output_path")"
mkdir -p "$repo_root/tmp/vhs"

temp_tape_path="$(mktemp "$repo_root/tmp/vhs/$(basename "${tape_path%.tape}").XXXXXX.tape")"
temp_tape_rel="${temp_tape_path#"$repo_root"/}"

cleanup() {
  rm -f "$temp_tape_path"
}
trap cleanup EXIT

awk -v output="$output_rel" '
  !rewritten && $1 == "Output" {
    print "Output " output
    rewritten = 1
    next
  }
  { print }
  END {
    if (!rewritten) {
      exit 11
    }
  }
' "$tape_path" > "$temp_tape_path"

echo "Rendering $output_rel with $container_engine using $container_image"
"$container_engine" run --rm \
  --user "$(id -u):$(id -g)" \
  -v "$repo_root:/vhs" \
  -w /vhs \
  "$container_image" \
  "/vhs/$temp_tape_rel"

[[ -f "$output_path" ]] || fail "VHS did not produce the expected output: $output_path"
echo "Rendered VHS demo: $output_path"
