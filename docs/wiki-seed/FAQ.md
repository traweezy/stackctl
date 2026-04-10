# FAQ

## Why Podman instead of Docker?

`stackctl` is built around the Podman runtime path and its compose providers.
The supported runtime floor, diagnostics, install helpers, and integration
qualification all assume Podman.

## Is Windows supported?

No. The project currently targets Linux and macOS for local runtime setup.

## Is Homebrew the official install path?

Not yet for the `stackctl` binary itself. Today, GitHub Releases plus the
bootstrap installer are the official install path. Homebrew on macOS is used
for runtime setup. See [../homebrew.md](../homebrew.md).

## Are JSON outputs stable?

The intended stable automation outputs are:

- `stackctl version --json`
- `stackctl env --json`
- `stackctl services --json`
- `stackctl status --json`

See [../output-contract.md](../output-contract.md) for the details.

## Can I run multiple local stacks at the same time?

No. `stackctl` intentionally enforces a one-local-stack-at-a-time safety model.

Named stacks help you switch cleanly between profiles, but only one managed
local stack should be running at once.

## Where do I find the screenshot and demo capture path?

Use [../demos.md](../demos.md) for the checked-in screenshot refresh flow, the
repo-local VHS helper, and the starter tapes under `examples/vhs/`.

## Where is the real command reference?

Use:

- [../cli/stackctl.md](../cli/stackctl.md)
- [../man/man1/stackctl.1](../man/man1/stackctl.1)
- `stackctl --help`
