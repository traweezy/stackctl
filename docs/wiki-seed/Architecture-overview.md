# Architecture overview

`stackctl` is a Go CLI that combines three main surfaces:

- command-line operator workflows
- guided configuration flows
- an interactive terminal UI

## High-level layout

- `cmd/`
  Cobra command wiring and the top-level user-facing command surface.
- `internal/config/`
  config loading, schema normalization, prompt flows, validation, and stack
  file handling.
- `internal/compose/`
  compose and container runtime orchestration helpers.
- `internal/system/`
  host runtime detection, package-manager helpers, and environment checks.
- `internal/doctor/`
  diagnostics and remediation guidance.
- `internal/output/`
  human-oriented and machine-oriented output helpers.
- `internal/tui/`
  the Bubble Tea-based dashboard and command palette surface.
- `scripts/`
  installer, smoke tests, journey coverage, and release qualification helpers.
- `tools/generate-cli-assets/`
  generation of CLI Markdown docs, man pages, and shell completions.

## Runtime model

The project supports two main stack modes:

- managed stacks, where `stackctl` owns and renders the stack files
- external stacks, where `stackctl` operates on an existing compose setup

The CLI, wizard, and TUI all sit on top of the same config and runtime model so
operators are not learning different products for each surface.

## Release and verification model

The repo treats CI and release qualification as part of the product contract:

- hosted CI verifies build, lint, security, unit, integration, installer, and
  packaging paths
- `platform-lab` extends that coverage to full-host Linux and macOS journeys
- release archives ship the binary plus the docs, security policy, and license

## Documentation split

The repo intentionally keeps the stable contract in versioned docs and uses the
future wiki only for narrative guidance.

That separation matters for `1.x`: command docs and compatibility guarantees
should remain tied to tagged releases, while troubleshooting notes and platform
walkthroughs can evolve more freely.
