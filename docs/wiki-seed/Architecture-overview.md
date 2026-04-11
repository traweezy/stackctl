# Architecture overview

`stackctl` is one Go application with three user-facing entry points:

- CLI commands
- setup and config flows
- a Bubble Tea TUI

Those entry points sit on the same config and runtime model so users do not
have to learn three different products.

## Package layout

- `cmd/`
  Cobra command wiring and root command behavior.
- `internal/config/`
  config loading, normalization, prompts, validation, and stack files.
- `internal/compose/`
  compose rendering and runtime orchestration helpers.
- `internal/system/`
  host detection, package-manager integration, and environment checks.
- `internal/doctor/`
  diagnostics and remediation guidance.
- `internal/output/`
  human-readable and machine-readable output helpers.
- `internal/tui/`
  the interactive dashboard and palette.
- `scripts/`
  installer, smoke coverage, and release-maintenance helpers.

## Runtime model

`stackctl` supports two stack modes:

- managed stacks, where `stackctl` owns the generated compose files
- external stacks, where `stackctl` works against an existing compose setup

That shared model is why `stackctl services`, `stackctl env`, the wizard, and
the TUI all agree on what is configured and running.

## Release shape

Release archives ship the binary plus the generated docs, man pages, license,
security policy, and changelog. Hosted CI covers the everyday build and test
path; the scheduled `platform-lab` workflows extend that coverage to full-host
Linux and macOS journeys.

## Documentation split

Use the versioned docs for the stable command and JSON contract. Use the wiki
pages for guidance, troubleshooting, and platform walkthroughs that may evolve
between releases.
