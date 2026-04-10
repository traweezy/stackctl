# Supply-Chain and Security Checks

This document describes the continuous supply-chain checks around `stackctl`.

It complements [../SECURITY.md](../SECURITY.md), the main CI workflow, and the
tagged-release verification path.

## Continuous hosted checks

The main `ci` workflow continuously enforces:

- `gitleaks` for secret scanning
- `gosec` for Go static security findings
- `govulncheck` for reachable Go vulnerability checks
- `codeql` for hosted semantic code scanning on the Go codebase
- `actionlint` for GitHub workflow linting
- `shellcheck` for shell script linting
- `lychee` for README and docs link validation
- `golangci-lint`, `go vet`, unit tests, race tests, coverage, integration, and
  installer/runtime smoke paths

The dedicated `codeql` workflow runs on pushes to `master`, pull requests, and
the weekly hosted security schedule so OpenSSF Scorecard can see a first-class
SAST signal instead of inferring from general linting alone.

## Pull request dependency review

The `dependency-review` workflow runs on pull requests and inspects dependency
changes before merge.

Current policy:

- fail on newly introduced `moderate`, `high`, or `critical` runtime
  vulnerabilities
- show OpenSSF Scorecard data for changed dependencies in the job summary
- retry briefly while GitHub dependency snapshots are still being prepared

License enforcement is intentionally not enabled in this first pass. The repo
already ships SBOMs for tagged releases, but transitive dependency license
allow-listing is deferred until the dependency graph is baselined with low
false-positive risk.

The dependency-review policy lives in
[../.github/dependency-review-config.yml](../.github/dependency-review-config.yml).

Separately, [../.github/dependabot.yml](../.github/dependabot.yml) opens
scheduled update pull requests for both `gomod` dependencies and GitHub
Actions pins. That keeps runtime dependencies and the workflow action surface
moving forward without relying on manual upgrade sweeps.

## Repository scorecards

The `scorecards` workflow runs on pushes to `master` and on the weekly
Saturday schedule.

It publishes:

- StepSecurity Harden-Runner audit data for runner egress visibility
- SARIF results to GitHub code scanning
- a short-lived workflow artifact for debugging
- the latest repository score so the README badge stays current

This does not replace the repo's other security checks. It adds a separate
OpenSSF-oriented view of branch protection, dependency pinning, token
permissions, release posture, and other supply-chain signals.

The Scorecards workflow also runs StepSecurity Harden-Runner in `audit` mode so
the repo can observe outbound runner behavior without blocking the job.

Workflow token scopes are intentionally explicit. The general hosted checks and
Scorecards paths only request read access plus the write scopes required for
SARIF uploads, while the tagged-release path elevates to `contents: write`,
`attestations: write`, and `id-token: write` only in the publish job that
actually needs them.

## Tagged release artifacts

Releases cut from the current tagged-release workflow are expected to ship
with:

- `checksums.txt`
- `checksums.txt.sigstore.json`
- per-archive SPDX SBOMs (`*.spdx.json`)
- GitHub artifact attestations

Operators should verify artifacts before use. See
[install-and-upgrade.md](./install-and-upgrade.md) and
[../SECURITY.md](../SECURITY.md).

Older `0.x` tags may predate some of these assets. For those historical
releases, checksum verification is still the baseline, but Sigstore bundle and
GitHub attestation checks only apply when the release actually publishes them.
