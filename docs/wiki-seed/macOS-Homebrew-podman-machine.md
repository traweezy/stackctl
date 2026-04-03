# macOS + Homebrew + podman machine

`stackctl` supports macOS through Homebrew plus `podman machine`.

## Recommended path

Use the supported bootstrap flow:

```bash
stackctl setup --install
stackctl doctor --fix --yes
```

That is the preferred entry point because it keeps the CLI and the runtime
setup path aligned.

## If Podman is installed but the machine is not ready

Run:

```bash
podman machine init
podman machine start
podman info
stackctl doctor
```

Then try the stack again:

```bash
stackctl start
stackctl services
```

## What Homebrew means here

In the current project state, Homebrew is the runtime bootstrap path for macOS.

It is not yet the official `stackctl` binary distribution channel. GitHub
Releases remain the authoritative install path until the project publishes an
upstream tap or another Homebrew distribution path intentionally.

See [../homebrew.md](../homebrew.md) for the packaging plan.
See [../platform-support.md](../platform-support.md) for the current host
capability matrix.

## Common macOS failure points

- Homebrew is installed, but `podman machine` has never been initialized
- `podman machine` exists, but the VM is stopped
- an older `podman` or compose provider is installed
- Cockpit helpers were left enabled even though stackctl cannot manage Cockpit
  on macOS
- terminal permissions or browser launch behavior differ from Linux defaults

When in doubt:

```bash
stackctl doctor
stackctl health
stackctl open
```
