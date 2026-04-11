# macOS + Homebrew + podman machine

`stackctl` supports macOS through Homebrew plus `podman machine`.

## Recommended flow

Start with:

```bash
stackctl setup --install
stackctl doctor --fix --yes
```

That keeps the CLI and the runtime setup flow aligned.

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

On macOS today:

- Homebrew is the supported runtime bootstrap path
- GitHub Releases are still the official `stackctl` binary channel

See [../homebrew.md](../homebrew.md) for the packaging plan and
[../platform-support.md](../platform-support.md) for the host matrix.

## Common macOS failure points

- Homebrew is installed, but `podman machine` has never been initialized
- the machine exists, but the VM is stopped
- an older `podman` or compose provider is installed
- Cockpit helpers were left enabled even though `stackctl` cannot manage
  Cockpit on macOS
- terminal permissions or browser launch behavior differ from Linux defaults

When in doubt, run:

```bash
stackctl doctor
stackctl health
stackctl open
```
