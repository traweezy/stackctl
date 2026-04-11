# Release Checklist

Use this checklist when cutting a `stackctl` release.

The workflows enforce a lot automatically. This page covers the human checks
that still matter before and after tagging.

## Before tagging

Confirm that:

- `CHANGELOG.md` matches the release scope
- compatibility-sensitive changes were checked against
  [compatibility.md](./compatibility.md)
- JSON output changes were checked against
  [output-contract.md](./output-contract.md)
- install or rollback changes were reflected in
  [install-and-upgrade.md](./install-and-upgrade.md)
- generated docs, man pages, and completions are current

## Local verification

Run the normal local release gates:

```bash
bash scripts/check-workflows.sh
bash scripts/check-shell-scripts.sh
bash scripts/check-links.sh
go test ./... -count=1
go test ./... -race -count=1
go test ./integration -tags=integration -count=1
bash scripts/check-coverage.sh
bash scripts/install-smoke.sh
bash scripts/journey-smoke.sh
goreleaser release --snapshot --clean
```

`scripts/check-coverage.sh` enforces a `100.0%` baseline by default. Only
override it when you are intentionally comparing history or doing local
diagnostics.

If the release changes CLI flags or help text, also run:

```bash
bash scripts/generate-cli-assets.sh
git diff --exit-code docs/cli docs/man docs/completions
```

If the release changes performance-sensitive TUI or runtime code, also run:

```bash
go test . -run '^$' -bench '^BenchmarkCLI(Help|Version|TUIHelp)$' -benchmem -count=10 > /tmp/stackctl-main-bench.txt
go test ./cmd ./internal/tui -run '^$' -bench . -benchmem -count=10 > /tmp/stackctl-hot-paths-bench.txt
STACKCTL_IDLE_BENCH_DURATION=5s go test ./internal/tui -run '^$' -bench '^BenchmarkIdleProgram(DefaultFPS|FPS30)$' -benchmem -count=1 -benchtime=1x
```

Keep the comparison artifacts under ignored `tmp/`, `.tmp/`, or `local/`.

If the release changes TUI layout or README screenshots, refresh and inspect
the checked-in still from a real rendered terminal window. Keep scratch
captures or demo material under ignored `tmp/`, `.tmp/`, or `local/`; do not
check in GIFs or maintainer-only demo tapes.

If `homebrew_casks` is enabled with `skip_upload: true`, inspect the generated
cask in `dist/` after the snapshot dry-run and confirm that its binary, man
page, completion, and caveat paths still match the release archive.

## CI and release gates

Before a tag is treated as releasable, verify:

- the normal `ci` workflow is green on the release commit
- the tagged `release` workflow is green
- `platform-lab` passes for the release tag
- the snapshot release dry-run is green

The tag gate is stricter than the normal push gate because it covers packaging
and cross-platform release qualification.

## Expected release artifacts

Tagged releases are expected to publish:

- Linux and macOS archives
- `checksums.txt`
- `checksums.txt.sigstore.json`
- per-archive SPDX SBOMs
- GitHub artifact attestations
- generated docs, man pages, and shell completions in the release archives

See [../SECURITY.md](../SECURITY.md) for verification posture and
[install-and-upgrade.md](./install-and-upgrade.md) for install and rollback
steps.

## After tagging

After the release workflow completes:

- walk the manual verification flow in
  [install-and-upgrade.md](./install-and-upgrade.md) against the new tag,
  including checksum verification and, when present, `gh release verify-asset`
  plus `cosign verify-blob`
- run `stackctl version --json` from the released binary
- spot-check the generated docs in the archive
- confirm the release notes and attached assets look complete

## Homebrew note

Homebrew distribution is still planned, not live. Until a tap repository,
publish token, and macOS signing decision exist, GitHub Releases remain the
official binary channel. See [homebrew.md](./homebrew.md).
