#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${STACKCTL_TUI_IDLE_OUTPUT_DIR:-$repo_root/tmp/perf/idle}"
duration="${STACKCTL_TUI_IDLE_DURATION:-5s}"

mkdir -p "$output_dir"

run_idle_bench() {
  local name="$1"
  local output_txt="$2"
  local cpu_profile="$3"
  local top_txt="$4"

  (
    cd "$repo_root"
    STACKCTL_IDLE_BENCH_DURATION="$duration" \
      go test ./internal/tui -run '^$' -bench "^${name}$" -benchmem -count=1 -benchtime=1x \
      -cpuprofile "$cpu_profile" >"$output_txt"
  )

  (
    cd "$repo_root"
    go tool pprof -top "$cpu_profile" >"$top_txt"
  )
}

run_idle_bench \
  "BenchmarkIdleProgramDefaultFPS" \
  "$output_dir/default.txt" \
  "$output_dir/default.cpu.out" \
  "$output_dir/default.top.txt"

run_idle_bench \
  "BenchmarkIdleProgramFPS30" \
  "$output_dir/fps30.txt" \
  "$output_dir/fps30.cpu.out" \
  "$output_dir/fps30.top.txt"

echo "Idle TUI evaluation artifacts:"
echo "- default benchmark: $output_dir/default.txt"
echo "- default CPU profile: $output_dir/default.cpu.out"
echo "- default pprof top: $output_dir/default.top.txt"
echo "- fps30 benchmark: $output_dir/fps30.txt"
echo "- fps30 CPU profile: $output_dir/fps30.cpu.out"
echo "- fps30 pprof top: $output_dir/fps30.top.txt"
