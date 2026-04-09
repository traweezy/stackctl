#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${STACKCTL_PGO_OUTPUT_DIR:-$repo_root/tmp/perf/pgo}"
profile_path="${STACKCTL_PGO_PROFILE:-$output_dir/main-startup.pgo}"
off_bench_path="${STACKCTL_PGO_OFF_BENCH:-$output_dir/main-bench-off.txt}"
pgo_bench_path="${STACKCTL_PGO_PGO_BENCH:-$output_dir/main-bench-pgo.txt}"
off_binary_path="${STACKCTL_PGO_OFF_BINARY:-$output_dir/stackctl-off}"
pgo_binary_path="${STACKCTL_PGO_PGO_BINARY:-$output_dir/stackctl-pgo}"
bench_regex="${STACKCTL_PGO_BENCH_REGEX:-BenchmarkCLI(Help|Version|TUIHelp)$}"
bench_count="${STACKCTL_PGO_BENCH_COUNT:-5}"
warmup_count="${STACKCTL_PGO_WARMUP:-3}"
run_count="${STACKCTL_PGO_RUNS:-30}"

mkdir -p "$output_dir"

echo "Generating representative startup profile at $profile_path"
(
  cd "$repo_root"
  go test . -run '^$' -bench "$bench_regex" -benchmem -count=1 -cpuprofile "$profile_path"
)

echo "Comparing main-package benchmarks with and without PGO"
(
  cd "$repo_root"
  go test . -run '^$' -bench "$bench_regex" -benchmem -count="$bench_count" -pgo=off >"$off_bench_path"
  go test . -run '^$' -bench "$bench_regex" -benchmem -count="$bench_count" -pgo="$profile_path" >"$pgo_bench_path"
)

echo "Building release-style binaries with and without PGO"
(
  cd "$repo_root"
  go build -trimpath -pgo=off -o "$off_binary_path" .
  go build -trimpath -pgo="$profile_path" -o "$pgo_binary_path" .
)

if command -v hyperfine >/dev/null 2>&1; then
  echo "Running real-binary startup comparisons with hyperfine"
  hyperfine \
    --warmup "$warmup_count" \
    --runs "$run_count" \
    --export-json "$output_dir/help.json" \
    "$off_binary_path --help" \
    "$pgo_binary_path --help"
  hyperfine \
    --warmup "$warmup_count" \
    --runs "$run_count" \
    --export-json "$output_dir/version.json" \
    "$off_binary_path version" \
    "$pgo_binary_path version"
  hyperfine \
    --warmup "$warmup_count" \
    --runs "$run_count" \
    --export-json "$output_dir/tui-help.json" \
    "$off_binary_path tui --help" \
    "$pgo_binary_path tui --help"
else
  echo "hyperfine is unavailable; skipping release-binary startup timing" >&2
fi

echo
echo "PGO evaluation artifacts:"
echo "- profile: $profile_path"
echo "- benchmarks without PGO: $off_bench_path"
echo "- benchmarks with PGO: $pgo_bench_path"
echo "- binary without PGO: $off_binary_path"
echo "- binary with PGO: $pgo_binary_path"
echo
echo "Only commit default.pgo when both the benchmark samples and the real-binary timing stay positive."
