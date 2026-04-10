# Security Policy

## Supported Versions

Security fixes are provided for:

- the latest tagged release
- the current `master` branch on a best-effort basis before the next release

Older tagged releases are not supported.

## Reporting a Vulnerability

Do not open public issues for suspected vulnerabilities.

Prefer GitHub private vulnerability reporting for this repository if it is
available. If private reporting is unavailable, contact the maintainer
privately through GitHub before disclosing details publicly.

When reporting a vulnerability, include:

- the affected `stackctl` version and platform
- clear reproduction steps or a proof of concept
- the security impact you observed or expect
- any mitigation or workaround already identified

## Continuous Security Posture

The repository continuously checks its source, workflows, dependency graph, and
release packaging with hosted automation.

For the current supply-chain monitoring and release-artifact posture, see
[docs/supply-chain.md](./docs/supply-chain.md).

## Release Artifact Verification

Releases cut from the current tagged-release workflow are expected to ship
with:

- `checksums.txt`
- `checksums.txt.sigstore.json`
- per-archive SPDX SBOMs (`*.spdx.json`)
- GitHub artifact attestations for the archives listed in `checksums.txt`

Older `0.x` tags may predate some of these assets. Consumers should always
verify `checksums.txt`, then layer on Sigstore bundle or GitHub attestation
checks only when the release actually carries those artifacts.
