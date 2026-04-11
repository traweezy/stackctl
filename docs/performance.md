# Performance

This is the local performance playbook for `stackctl`.

Use it when you are changing startup, rendering, config flows, or other code
that users will feel directly.

## Principles

- measure first, optimize second
- use Go benchmarks and profiles before guessing
- treat CLI startup and TUI redraw cost as real UX work
- keep benchmark flows easy to rerun on a branch

## Benchmarks

### Go benchmarks plus `benchstat`

Use package benchmarks for stable A/B comparisons of hot code.

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

Use this for:

- root-command startup and command construction
- TUI rendering
- palette filtering or paging
- runtime service shaping
- output formatting
- config or wizard helpers

Committed startup benchmarks live in `main_benchmark_test.go`:

- `BenchmarkCLIHelp`
- `BenchmarkCLIVersion`
- `BenchmarkCLITUIHelp`

### CPU, memory, and trace captures

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

Use this when you need to answer:

- where is CPU time actually going
- which code paths allocate most heavily
- whether the TUI is blocked on rendering, I/O, or scheduler behavior

### `hyperfine` for real CLI timing

Use `hyperfine` for end-to-end command latency instead of only relying on
microbenchmarks.

```bash
bash scripts/bench-cli.sh
```

That script builds a local benchmark binary and measures:

- `stackctl --help`
- `stackctl version`
- `stackctl tui --help`

These are config-independent and safe on any development host.

### Idle-session TUI FPS evaluation

Use the local evaluator before deciding whether to carry an explicit
`tea.WithFPS(...)` cap:

```bash
bash scripts/evaluate-tui-idle.sh
```

This compares the default Bubble Tea renderer behavior with a capped `30 FPS`
idle run and writes benchmark plus `pprof` artifacts under `tmp/perf/idle/`.

Current decision as of 2026-04-10:

- do not set an explicit `tea.WithFPS(...)` cap yet
- the first 5-second idle-session comparison was effectively flat:
  - default: `5.002 s/op`, `3.74 MB/op`, `40986 allocs/op`
  - `WithFPS(30)`: `5.001 s/op`, `3.73 MB/op`, `40843 allocs/op`
- sampled CPU time was tiny in both captures and dominated by runtime polling
  and parking
- re-evaluate only if a later trace shows real idle redraw pressure or another
  animation-heavy path lands

### Profile-guided optimization

Once representative profiles exist, the repo can carry a committed
`default.pgo` for release builds.

Rules:

- do not generate `default.pgo` from a toy benchmark only
- prefer representative TUI and command paths
- refresh the profile when performance-sensitive code changes substantially

The Go toolchain automatically applies `default.pgo` when it is present in the
main package directory.

Use the local evaluator before committing one:

```bash
bash scripts/evaluate-pgo.sh
```

Current decision as of 2026-04-09:

- do not commit `default.pgo` yet
- a candidate profile generated from the committed root startup benchmarks
  improved `go test` samples for `--help` and `tui --help`
- the same candidate produced mixed `go build -trimpath` startup results in
  `hyperfine`
- re-evaluate after another representative idle-session or runtime-heavy
  profile capture, not from synthetic startup samples alone

## Likely hotspots

The main candidates to measure before release are:

- `main_benchmark_test.go`
- `internal/tui/tui.go`
- `internal/tui/palette.go`
- `cmd/runtime.go`
- `cmd/tui.go`
- `internal/config/prompt_huh.go`

The practical questions are:

- how much startup latency do common commands add
- how expensive is a full TUI render on medium and compact terminals
- how much work does palette filtering do as services and stacks grow
- whether idle redraw or allocation work is higher than it should be

## Release-candidate checklist

Before `1.0.0`, keep these in place:

- at least one committed benchmark file for `main`, `cmd`, or `internal/tui`
- a repeatable `hyperfine` CLI benchmark flow
- one representative profile capture used to evaluate PGO
- a documented decision on whether the TUI should set an explicit Bubble Tea
  FPS cap

The current FPS-cap decision is to keep the Bubble Tea default.

## Non-goals

- no speculative performance churn without measurements
- no UX regressions just to improve one benchmark number
- no heavyweight continuous benchmarking service before the local story is
  stable
