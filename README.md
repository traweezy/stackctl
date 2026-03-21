# stackctl

`stackctl` is a Linux CLI for bringing up and managing a local
Podman-based development stack.

It is aimed at the "get my local services running quickly" path:

- bootstrap config on first run
- scaffold a managed local stack under standard XDG paths
- start, stop, restart, and reset the stack
- inspect status, health, diagnostics, and logs
- print connection details for local services

The bundled managed stack currently includes:

- PostgreSQL
- Redis
- pgAdmin

Cockpit is also supported as a local management UI when it is installed on
the host.

> [!IMPORTANT]
> `stackctl` is Linux-only right now.
>
> - GitHub release binaries are published for Linux `x86_64` and `arm64`
> - the install script only supports Linux
> - `setup --install` currently targets `apt`-based systems
> - macOS and Windows are not supported yet

## What You Get

With `stackctl`, you can:

- create and validate a persistent config
- use a CLI-managed stack directory or point at your own compose project
- start the local stack and wait for services to come up
- tail all logs or a single service
- inspect health and run read-only diagnostics
- print ready-to-use URLs and DSNs

## Install

### Quick install from GitHub Releases

Install the latest release to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | bash
```

Install to `/usr/local/bin` instead:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | bash -s -- --system
```

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | \
  bash -s -- --version v0.1.0
```

Manual downloads are available from:

https://github.com/traweezy/stackctl/releases/latest

### Build from source

If you already have a recent Go toolchain:

```bash
git clone https://github.com/traweezy/stackctl.git
cd stackctl
go build ./...
go run . --help
```

## Prerequisites

`stackctl` expects a Linux machine with Podman available.

For the managed local stack, the practical requirements are:

- `podman`
- `podman compose`

If you want `stackctl` to try installing missing packages for you, use:

```bash
stackctl setup --install
```

That installer flow is currently intended for `apt`-based Linux systems.

## Getting Started

The fastest path is:

```bash
stackctl setup
stackctl start
stackctl status
stackctl connect
```

If you are working from source instead of an installed binary, replace
`stackctl` with `go run .`.

### First run behavior

On first run, `stackctl` can create config for you and scaffold a managed
stack into your user data directory.

Common first-run commands:

```bash
stackctl setup
stackctl config init
stackctl start
```

Useful setup variants:

```bash
stackctl setup --non-interactive
stackctl setup --install
stackctl setup --install --yes
```

## Where stackctl stores files

By default, `stackctl` uses standard Linux user directories:

- config: `~/.config/stackctl/config.yaml`
- managed data root: `~/.local/share/stackctl`
- managed stack directory: `~/.local/share/stackctl/stacks/dev-stack`
- managed compose file:
  `~/.local/share/stackctl/stacks/dev-stack/compose.yaml`

If `XDG_DATA_HOME` is set, managed stack data is stored under
`$XDG_DATA_HOME/stackctl` instead.

## Managed stack vs external stack

`stackctl` supports two modes:

### Managed stack

In managed mode, `stackctl` owns the stack directory and can scaffold the
compose file from the embedded template.

This is the default and the easiest path to get started.

### External stack

In external mode, your config points at an existing compose directory that
you manage yourself.

Use that if you already have a custom stack and just want `stackctl` to act
as the operator CLI.

## Common commands

### Setup and config

```bash
stackctl setup
stackctl config path
stackctl config view
stackctl config validate
stackctl config edit
stackctl config scaffold
stackctl config reset
```

The example config is in
[`examples/config.example.yaml`](examples/config.example.yaml).

### Stack lifecycle

```bash
stackctl start
stackctl stop
stackctl restart
stackctl status
stackctl reset --volumes --force
```

`start` and `restart` print connection info after the stack is ready. They
do not automatically open browser windows.

### Logs

```bash
stackctl logs
stackctl logs -n 50
stackctl logs -w
stackctl logs -s postgres -n 50
stackctl logs -s redis -w
```

By default, `stackctl logs` prints the last 100 lines and exits.

Supported service filters are:

- `postgres` or `pg`
- `redis` or `rd`
- `pgadmin`

### Diagnostics and health

```bash
stackctl doctor
stackctl health
stackctl health -w -i 2
```

Use `doctor` when something looks wrong before changing anything. It is a
read-only diagnostic command.

### URLs and connection details

```bash
stackctl connect
stackctl open
stackctl open pgadmin
stackctl open all
```

`connect` prints the configured DSNs and URLs.

`open` is the explicit browser-opening command. If browser launch is not
available, `stackctl` prints the URL instead of failing.

## Suggested first workflow

If you are starting from a clean Linux machine:

1. Install `stackctl`
2. Run `stackctl setup`
3. Run `stackctl start`
4. Check `stackctl status`
5. Check `stackctl health`
6. Run `stackctl connect`
7. Use `stackctl logs -w` while bringing up your app

## Troubleshooting

### No config exists yet

Run one of:

```bash
stackctl setup
stackctl config init
```

### Managed stack files are missing

Recreate them with:

```bash
stackctl config scaffold
```

Then validate the config:

```bash
stackctl config validate
```

### Something is already using one of the expected ports

Run:

```bash
stackctl doctor
```

This will tell you whether the port is owned by an expected local service or
by something unrelated.

### Browser launch fails

Use:

```bash
stackctl open
```

If `stackctl` cannot open the browser automatically, it prints the URL so
you can open it yourself.

## Next steps

After the initial setup is working, the usual next commands are:

- `stackctl logs -w` while developing
- `stackctl status` to confirm container state
- `stackctl connect` to copy DSNs and URLs into your app config
- `stackctl doctor` when the stack behaves unexpectedly
- `stackctl config edit` if you want to switch to an external stack path

## Development

For local development on the CLI itself:

```bash
go test ./...
go build ./...
go run . --help
```

## Release flow

Releases are created from tags that match `v*`.

Example:

```bash
git tag v0.1.0
git push origin v0.1.0
```
