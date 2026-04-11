# Compatibility Policy

Use this page when you need to know what a future `1.x` release can change
without breaking you.

`stackctl` is still in `0.x`, but the project is already tightening the parts
that are meant to become the `1.x` contract.

## SemVer intent

For `1.x`:

- breaking changes land only in a new major version
- additive features can land in minor releases
- patch releases fix bugs without breaking documented behavior

## Stable in `1.x`

These are the parts callers should be able to rely on.

### CLI and installer contract

- documented command names
- documented flag names and meanings
- documented root flags
- documented argument ordering
- documented installer flags in `scripts/install.sh`

### Environment and selection

- `STACKCTL_STACK`
- documented root-output env overrides such as `ACCESSIBLE`,
  `STACKCTL_WIZARD_PLAIN`, `STACKCTL_LOG_LEVEL`, `STACKCTL_LOG_FORMAT`, and
  `STACKCTL_LOG_FILE`

### Saved config

- the saved YAML config format
- `schema_version: 1` for current maintained releases
- in-memory upgrade of legacy config files that predate `schema_version`
- rejection of unknown future schema versions instead of guessing

### Machine-readable output

These commands are the documented JSON contract:

- `stackctl version --json`
- `stackctl env --json`
- `stackctl services --json`
- `stackctl status --json`

See [output-contract.md](./output-contract.md) for fields and expectations.

## Allowed to change in `1.x`

These user-facing details can evolve as long as command behavior stays
compatible:

- table formatting
- spacing and color choices
- wizard copy and prompt phrasing
- TUI layout and navigation hints
- spinner text and progress wording
- non-JSON diagnostic text

Treat them as UI, not as a scripting interface.

## JSON stability rules

For the documented JSON outputs:

- existing commands and `--json` flags should stay available in `1.x`
- documented field names should not be renamed or removed in `1.x`
- documented field meanings and types should not change in `1.x`
- minor releases may add new optional fields
- consumers should ignore unknown fields

## Exit codes

The stable guarantee today is simple:

- `0` on success
- non-zero on failure

Specific non-zero numbers are not yet part of the public contract.

## Runtime support

`stackctl` targets Linux and macOS for local runtime setup.

The supported managed-runtime floor for `1.x` is:

- `podman` `4.9.3+`
- a `podman compose` provider `1.0.6+`

If the detected runtime is older than that floor, `stackctl doctor` warns and
managed runtime commands fail fast with upgrade guidance.

## How support is tested

### On normal development pushes

Hosted CI covers:

- build, lint, race, coverage, and security checks
- installer smoke
- Linux Podman integration
- Linux package-manager smoke in disposable distro containers

That Linux Podman integration path is the source of truth for the managed
runtime floor above.

### Before and around releases

Scheduled and release-time coverage extends to:

- full-host Linux distro journeys on self-hosted runners
- macOS Homebrew plus `podman machine` journeys on self-hosted runners

Some of those flows may still prove that older distro packages install. That
does not expand the support floor unless this policy changes.

The release-qualified workflows live in `.github/workflows/platform-lab.yml`.
For package-manager and host differences, see
[platform-support.md](./platform-support.md).

## Install and rollback expectations

Use [install-and-upgrade.md](./install-and-upgrade.md) for the operational
steps.

The `1.x` rollback promise assumes both versions understand `schema_version: 1`.
If you roll back to an older pre-schema `0.x` build, restore the config backup
you made before the upgrade.

## If the contract needs to change

If a future change would break one of the stable items above, the preferred
order is:

- make it additive instead
- stage it behind a new flag or output mode
- defer it to a new major version
