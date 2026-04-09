# Performance

This document is the local performance playbook for `stackctl`.

The goal is to measure startup, rendering, and allocation changes before we
claim that a refactor or release candidate is faster.

## Principles

- measure first, optimize second
- prefer built-in Go profiling and tracing over guesswork
- treat CLI startup latency and TUI redraw cost as first-class operator UX
- keep benchmark paths reproducible and easy to rerun on a branch

## Primary tools

### Go benchmarks plus `benchstat`

Use package benchmarks for stable A/B comparisons of hot paths.

Install `benchstat` when needed:

```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

Collect two samples and compare them:

```bash
go test ./cmd ./internal/tui -run '^$' -bench . -benchmem -count=10 > /tmp/stackctl-old.txt
go test ./cmd ./internal/tui -run '^$' -bench . -benchmem -count=10 > /tmp/stackctl-new.txt
benchstat /tmp/stackctl-old.txt /tmp/stackctl-new.txt
```

Use this when a change affects:

- root-command startup and command construction
- TUI rendering
- palette filtering or paging
- runtime service shaping
- output formatting
- config or wizard helpers

Current committed startup benchmarks live in `main_benchmark_test.go`:

- `BenchmarkCLIHelp`
- `BenchmarkCLIVersion`
- `BenchmarkCLITUIHelp`

### CPU, memory, and execution traces

Use Go profiling flags before changing runtime behavior.

Example:

```bash
go test ./internal/tui -run '^$' -bench . -benchmem \
  -cpuprofile=/tmp/stackctl-tui.cpu.out \
  -memprofile=/tmp/stackctl-tui.mem.out \
  -trace=/tmp/stackctl-tui.trace.out

go tool pprof -http=:0 /tmp/stackctl-tui.cpu.out
go tool trace /tmp/stackctl-tui.trace.out
```

Prefer this path when you need to answer:

- where is CPU time actually going?
- which code paths allocate most heavily?
- is the TUI blocked on rendering, I/O, or scheduler behavior?

### `hyperfine` for real CLI timing

Use `hyperfine` for end-to-end command latency instead of package-level
microbenchmarks.

The repo-local wrapper is:

```bash
bash scripts/bench-cli.sh
```

It builds a local benchmark binary and measures:

- `stackctl --help`
- `stackctl version`
- `stackctl tui --help`

These are intentionally config-independent and safe on any development host.

If you want config-dependent commands too, pass them explicitly to `hyperfine`
or extend the script for the target branch.

### Profile-guided optimization

Once representative profiles exist, we can add a committed `default.pgo` for
release builds.

Rules:

- do not generate `default.pgo` from a toy benchmark only
- prefer representative TUI and command paths
- refresh the profile when performance-sensitive code changes substantially

The Go toolchain will automatically apply `default.pgo` when it is present in
the main package directory.

Use the repo-local evaluation flow before committing one:

```bash
bash scripts/evaluate-pgo.sh
```

Current decision as of 2026-04-09:

- do not commit `default.pgo` yet
- a candidate profile generated from the committed root startup benchmarks
  improved `go test` benchmark samples for `--help` and `tui --help`
- the same candidate produced mixed release-style `go build -trimpath`
  startup results in `hyperfine`: `--help` was slower, while `version` and
  `tui --help` were effectively neutral to slightly positive
- re-evaluate after another representative idle-session or runtime-heavy
  profile capture, not from synthetic startup samples alone

## Repo-specific hotspots

The main candidates to measure before release are:

- `main_benchmark_test.go`
- `internal/tui/tui.go`
- `internal/tui/palette.go`
- `cmd/runtime.go`
- `cmd/tui.go`
- `internal/config/prompt_huh.go`

The practical questions for this repo are:

- how much startup latency do common operator commands add?
- how expensive is a full TUI render on medium and compact terminals?
- how much work does palette filtering do as services and stacks grow?
- are we redrawing or allocating more than necessary while idle?

## Release-candidate checks

Before `1.0.0`, we should have:

- at least one committed benchmark file for `main`, `cmd`, or `internal/tui`
- a repeatable `hyperfine` CLI benchmark path
- one representative profile capture used to evaluate PGO
- a documented decision on whether the TUI should set an explicit Bubble Tea FPS
  cap

## Non-goals

- no speculative performance churn without measurements
- no UX regressions just to improve one benchmark number
- no heavyweight continuous benchmarking service before the local benchmark
  story is stable
