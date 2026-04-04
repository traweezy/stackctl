#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${STACKCTL_BENCH_OUTPUT_DIR:-$repo_root/tmp/perf}"
binary_path="${STACKCTL_BENCH_BINARY:-$output_dir/stackctl-bench}"
json_path="${STACKCTL_BENCH_JSON:-$output_dir/cli-bench.json}"
markdown_path="${STACKCTL_BENCH_MARKDOWN:-$output_dir/cli-bench.md}"
warmup_count="${STACKCTL_BENCH_WARMUP:-3}"

if ! command -v hyperfine >/dev/null 2>&1; then
  echo "hyperfine is required; install it before running scripts/bench-cli.sh" >&2
  exit 1
fi

mkdir -p "$output_dir"

echo "Building benchmark binary at $binary_path"
(
  cd "$repo_root"
  go build -trimpath -o "$binary_path" .
)

declare -a commands=(
  "$binary_path --help"
  "$binary_path version"
  "$binary_path tui --help"
)

echo "Running CLI startup benchmarks"
hyperfine \
  --warmup "$warmup_count" \
  --export-json "$json_path" \
  --export-markdown "$markdown_path" \
  "${commands[@]}"

echo
echo "Benchmark artifacts:"
echo "- JSON: $json_path"
echo "- Markdown: $markdown_path"
