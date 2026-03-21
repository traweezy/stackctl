# stackctl

`stackctl` is a Go CLI for running and inspecting a local Podman-based development stack.

It is designed for local development workflows, with:

- persistent user config
- managed stack data under standard user directories
- first-run setup and config bootstrapping
- stack lifecycle commands
- diagnostics, health, logs, and connection helpers

GitHub Releases now publish Linux tarballs, and a one-command installer is available for the latest release.

## Install (Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctrl/master/scripts/install.sh | bash
```

Install to `/usr/local/bin` instead:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctrl/master/scripts/install.sh | bash -s -- --system
```

## Manual Download

Download the latest release assets from:

https://github.com/traweezy/stackctrl/releases/latest

## Commands

The CLI currently provides:

- `stackctl setup`
- `stackctl doctor`
- `stackctl config`
- `stackctl start`
- `stackctl stop`
- `stackctl restart`
- `stackctl status`
- `stackctl logs`
- `stackctl open`
- `stackctl health`
- `stackctl connect`
- `stackctl reset`
- `stackctl version`

## First Run

The CLI stores user config at:

- Linux: `~/.config/stackctl/config.yaml`

When you use the managed stack flow, `stackctl` stores runtime stack files at:

- Linux: `~/.local/share/stackctl/stacks/dev-stack/compose.yaml`

You can create config explicitly:

```bash
go run . config init
```

Or let the CLI help on first run. For example, if you run:

```bash
go run . start
```

and no config exists yet, `stackctl` will explain that, offer interactive setup, and offer to scaffold a managed stack into the standard user data directory.

## Setup

Use setup to prepare the environment and config:

```bash
go run . setup
go run . setup --non-interactive
go run . setup --install
```

`setup --install` supports `apt`-based systems and installs the packages described in the spec:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`
- `cockpit`
- `cockpit-podman`

If the current config is using a managed stack and the managed files are missing, `setup` can scaffold them from the embedded default template.

## Config

Manage config after first run with:

```bash
go run . config path
go run . config view
go run . config validate
go run . config edit
go run . config scaffold
go run . config reset
```

An example config is bundled at [examples/config.example.yaml](/home/tylers/Dev/go/github.com/traweezy/stackctl/examples/config.example.yaml).

`managed: true` means the CLI owns the stack path under `~/.local/share/stackctl/...` and can scaffold the embedded default compose file there. When `managed: false`, the config points at a custom external stack directory that you manage yourself.

The embedded source template for the managed stack lives at [templates/dev-stack/compose.yaml](/home/tylers/Dev/go/github.com/traweezy/stackctl/templates/dev-stack/compose.yaml).

## Custom Stack Paths

The interactive config flow lets you switch between:

- a managed stack under `~/.local/share/stackctl/stacks/dev-stack`
- an external custom stack directory

If you choose an external stack, `stackctl` saves that path in config and does not treat the stack files as CLI-managed.

## Runtime

Typical commands:

```bash
go run . start
go run . status
go run . stop
go run . restart
go run . reset --volumes --force
```

`start` and `restart` print the configured DSNs and URLs after the stack is ready. They do not try to open browser tabs automatically.

## Logs And Health

Useful examples:

```bash
go run . logs
go run . logs -s postgres -w
go run . logs -s redis -n 50
go run . health
go run . health -w -i 2
```

## Connection Info

Print ready-to-use local endpoints with:

```bash
go run . connect
go run . open
go run . open pgadmin
go run . open all
```

`connect` prints DSNs and URLs derived from the current config. `open` is the explicit browser action; if launching a browser is unavailable, it prints the URL instead.

## Manual Testing

Useful manual checks for the managed stack model:

- delete `~/.config/stackctl/config.yaml`, run `go run . start`, and verify the config is recreated under `~/.config/stackctl`
- verify the managed stack is scaffolded under `~/.local/share/stackctl/stacks/dev-stack/compose.yaml`
- delete only the config file, keep the managed stack data, rerun setup, and verify the existing managed stack is reused cleanly
- delete only the managed stack data, keep the config file, and verify `go run . doctor` and `go run . config validate` fail clearly
- run `go run . config scaffold` to recreate the managed compose file from the embedded template

## Build

The repo is intended to work directly with standard Go tooling:

```bash
go mod tidy
go test ./...
go build ./...
go run . --help
```

## Release

Tagged releases are published by the GitHub Actions release workflow using GoReleaser. To create a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```
