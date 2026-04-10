# Release Checklist

This is the operator-facing checklist for cutting a `stackctl` release.

It complements the enforced CI/release workflows. Use it to make sure the human
side of the release still matches the documented `1.x` contract.

## Before tagging

Confirm that the branch is in a releasable state:

- `CHANGELOG.md` reflects the release scope
- compatibility-sensitive changes were reviewed against
  [compatibility.md](./compatibility.md)
- machine-readable output changes were reviewed against
  [output-contract.md](./output-contract.md)
- install or rollback behavior changes were reflected in
  [install-and-upgrade.md](./install-and-upgrade.md)
- generated docs/man/completions are current

## Local verification

Run the normal local release qualification commands:

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

`scripts/check-coverage.sh` now enforces a `100.0%` baseline by default. Only
override it intentionally when you are comparing historical branches or doing a
temporary local diagnostic run.

If the release changes CLI flags or help text, also run:

```bash
bash scripts/generate-cli-assets.sh
git diff --exit-code docs/cli docs/man docs/completions
```

If the release includes performance-sensitive TUI or runtime changes, also run:

```bash
bash scripts/bench-cli.sh
bash scripts/evaluate-pgo.sh
bash scripts/evaluate-tui-idle.sh
```

For code-level regressions, compare benchmark runs with the workflow in
[performance.md](./performance.md).

If the release includes TUI layout changes or README/wiki media refreshes,
also run:

```bash
bash scripts/capture-docs-media.sh
```

Then inspect `docs/media/tui-services.png` and confirm the checked-in still
matches the current rendered experience.

If `homebrew_casks` is enabled with `skip_upload: true`, also inspect the
generated cask in `dist/` after the snapshot dry-run and confirm that its
binary, man page, completion, and caveat paths still match the release
archive.

## CI and release gates

Before a tag is treated as releasable, verify:

- the normal `ci` workflow is green on the release commit
- the tagged `release` workflow is green
- `platform-lab` passes for the release tag
- the snapshot release dry-run is green

The tag gate is intentionally stronger than the normal push gate. Hosted CI and
`platform-lab` together are the release qualification path.

## Artifact expectations

Tagged releases are expected to publish:

- Linux and macOS archives
- `checksums.txt`
- `checksums.txt.sigstore.json`
- per-archive SPDX SBOMs
- GitHub artifact attestations
- generated docs, man pages, and shell completions in the release archives

See [../SECURITY.md](../SECURITY.md) for the verification posture and
[install-and-upgrade.md](./install-and-upgrade.md) for the install and rollback
flows.

## Post-tag checks

After the release workflow completes:

- download one archive and verify it against `checksums.txt`
- if the release includes GitHub artifact attestations, run
  `gh release verify-asset <tag> <archive>`
- if the release includes `checksums.txt.sigstore.json`, verify it with
  `cosign verify-blob`
- run `stackctl version --json` from the released binary
- spot-check the generated docs in the archive
- confirm the release notes and attached assets look complete

## Homebrew note

Homebrew distribution is still a planned path, not the authoritative binary
channel.

Until a tap repository, publish token, and macOS signing decision exist,
GitHub Releases remain the official install source. See
[homebrew.md](./homebrew.md).
