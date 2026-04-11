# FAQ

## Why Podman instead of Docker?

`stackctl` is built around Podman. The runtime checks, setup helpers, docs, and
qualification coverage all target Podman and a Podman compose provider.

## Is Windows supported?

No. `stackctl` currently supports Linux and macOS for local runtime setup.

## Is Homebrew the official install path?

Not for the `stackctl` binary yet. Today, the official binary channel is GitHub
Releases plus the install script. On macOS, Homebrew is used to install and
update the local runtime pieces around Podman. See [../homebrew.md](../homebrew.md).

## Which JSON outputs can I automate against?

These are the documented JSON outputs for `1.x`:

- `stackctl version --json`
- `stackctl env --json`
- `stackctl services --json`
- `stackctl status --json`

See [../output-contract.md](../output-contract.md) for field-level details.

## Can I run multiple local stacks at the same time?

No. `stackctl` keeps the managed runtime model simple and safe: one managed
local stack at a time. Named stacks let you switch between profiles cleanly,
but only one managed stack should be running.

## Where is the command reference?

Use:

- [../cli/stackctl.md](../cli/stackctl.md)
- [../man/man1/stackctl.1](../man/man1/stackctl.1)
- `stackctl --help`

## How do I refresh README screenshots or local demos?

Use [../demos.md](../demos.md). That page is for maintainers working on docs
media, not for everyday `stackctl` use.
