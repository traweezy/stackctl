# Compatibility Policy

`stackctl` is still in `0.x`, but the project is already freezing the surfaces
that are intended to define the `1.x` compatibility contract.

This document explains what should be considered stable, what is intentionally
human-oriented and allowed to evolve, and how platform support is qualified.

## SemVer intent

The project follows Semantic Versioning for the public CLI contract.

For `1.x`, the intent is:

- breaking changes happen only in a new major version
- additive features may ship in minor releases
- patch releases fix bugs without intentionally breaking documented stable
  behavior

## Stable in `1.x`

The following surfaces are intended to be stable in `1.x` once `1.0.0` ships.

### CLI surface

- documented command names
- documented flag names and meanings
- documented root flags
- documented command argument ordering
- documented installer flags in `scripts/install.sh`

### Environment and selection surface

- `STACKCTL_STACK`
- documented root-output env overrides such as `ACCESSIBLE`,
  `STACKCTL_WIZARD_PLAIN`, `STACKCTL_LOG_LEVEL`, `STACKCTL_LOG_FORMAT`, and
  `STACKCTL_LOG_FILE`

### Saved config

- the saved YAML config is part of the public API
- current config files written by maintained releases include
  `schema_version: 1`
- legacy config files without `schema_version` are upgraded in-memory on load
  and rewritten with `schema_version: 1` on save
- config files with a newer unknown `schema_version` are rejected rather than
  guessed at

### Machine-readable output

The stable machine-readable outputs are documented in
[output-contract.md](./output-contract.md):

- `stackctl version --json`
- `stackctl env --json`
- `stackctl services --json`
- `stackctl status --json`

## Allowed to evolve in `1.x`

The following surfaces are intentionally human-oriented and may change between
minor releases as long as the command behavior remains compatible.

- table formatting
- spacing and color choices
- wizard copy and prompt phrasing
- TUI layout, navigation hints, and visual hierarchy
- spinner and progress wording
- non-JSON diagnostic text

These should still aim to remain familiar, but they are not treated as strict
automation contracts.

## JSON stability rules

For the documented machine-readable outputs:

- existing top-level commands and `--json` flags should remain available in
  `1.x`
- existing documented field names should not be renamed or removed in `1.x`
- existing documented field meanings and types should not change in `1.x`
- minor releases may add new optional fields
- consumers should ignore unknown fields

## Exit behavior

`stackctl` primarily guarantees:

- `0` on success
- non-zero on failure

Specific non-zero exit code numbers are not currently treated as a stable
public contract.

## Platform support policy

`stackctl` targets Linux and macOS for local runtime setup.

The supported managed-runtime floor for the `1.x` contract is:

- `podman` `4.9.3+`
- a `podman compose` provider `1.0.6+`

When a detected runtime is below that floor, `stackctl doctor` warns and
managed runtime commands fail fast with upgrade guidance instead of guessing
through older behavior.

Support is qualified at two levels:

### Continuously verified

These paths are exercised in hosted CI on every normal development cycle:

- build, lint, race, coverage, and security checks
- installer smoke
- Linux Podman integration coverage
- Linux package-manager smoke using disposable distro containers

The hosted Linux Podman integration path is the source of truth for the managed
runtime version floor above.

### Release-qualified

These paths are qualified on a weekly schedule and as part of the tagged
release gate, but they are not run on every push:

- full-host Linux distro journeys on self-hosted runners
- macOS Homebrew plus `podman machine` journeys on self-hosted runners

Some release-qualified install flows may still prove that older distro packages
can be installed. That installability coverage does not expand the supported
managed-runtime floor unless the compatibility policy is updated intentionally.

The release-qualified workflows live in
`.github/workflows/platform-lab.yml`.

## Install and rollback guidance

Operational install, upgrade, rollback, and config-backup guidance lives in
[install-and-upgrade.md](./install-and-upgrade.md).

The `1.x` rollback expectation assumes both versions understand
`schema_version: 1`.

Until `1.0.0` ships, rolling back to an older pre-schema `0.x` build should be
paired with restoring the config backup you made before the upgrade.

## Changing the contract

If a future change would break one of the stable surfaces above, it should be
handled in one of these ways:

- make the change additive instead of breaking
- stage the change behind a new flag or output mode
- defer the change to a new major version
