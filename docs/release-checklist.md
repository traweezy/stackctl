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
go test ./... -count=1
go test ./... -race -count=1
go test ./integration -tags=integration -count=1
bash scripts/check-coverage.sh
bash scripts/install-smoke.sh
bash scripts/journey-smoke.sh
goreleaser release --snapshot --clean
```

If the release changes CLI flags or help text, also run:

```bash
bash scripts/generate-cli-assets.sh
git diff --exit-code docs/cli docs/man docs/completions
```

## CI and release gates

Before a tag is treated as releasable, verify:

- the branch watcher reports a green `ci` run for the release commit:

```bash
bash scripts/watch-ci.sh
```

- the normal `ci` workflow is green on the release commit
- the tagged `release` workflow is green
- `platform-lab` passes for the release tag
- the snapshot release dry-run is green

The tag gate is intentionally stronger than the normal push gate. Hosted CI and
`platform-lab` together are the release qualification path.

When you expect multiple pushes on the same branch, use:

```bash
bash scripts/watch-ci.sh --latest-branch
```

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
- run `stackctl version --json` from the released binary
- spot-check the generated docs in the archive
- confirm the release notes and attached assets look complete

## Homebrew note

Homebrew distribution is still a planned path, not the authoritative binary
channel.

Until a tap repository and publish token exist, GitHub Releases remain the
official install source. See [homebrew.md](./homebrew.md).
