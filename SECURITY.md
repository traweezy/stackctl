# Security Policy

## Supported Versions

Security fixes are provided for:

- the latest tagged release
- the current `master` branch on a best-effort basis before the next release

Older tagged releases are not supported.

## Reporting a Vulnerability

Do not open public issues for suspected vulnerabilities.

Use GitHub private vulnerability reporting for this repository:

- <https://github.com/traweezy/stackctl/security/advisories/new>

If the advisory form is unavailable, start from the repository security page
and request a private disclosure path before sharing details publicly:

- <https://github.com/traweezy/stackctl/security>

When reporting a vulnerability, include:

- the affected `stackctl` version and platform
- clear reproduction steps or a proof of concept
- the security impact you observed or expect
- any mitigation or workaround already identified

## Disclosure timeline

For non-spam reports that include enough detail to reproduce or assess:

- initial acknowledgement target: within 3 business days
- next status update target: within 7 calendar days
- coordinated public disclosure target: after a fix or mitigation is ready, or
  after an agreed disclosure window if a fix needs more time

The exact remediation timeline depends on severity, exploitability, and whether
the issue affects the latest tagged release, `master`, or only unsupported
historical tags.

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
