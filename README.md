# stackctl

`stackctl` is a Go CLI for running and inspecting a local Podman-based development stack.

It is designed for local development workflows, with:

- persistent user config
- first-run setup and config bootstrapping
- stack lifecycle commands
- diagnostics, health, logs, and connection helpers

Distribution and release automation are not set up yet. This repo is focused on a solid local CLI foundation.

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

- Linux and macOS: `~/.config/stackctl/config.yaml`

You can create config explicitly:

```bash
go run . config init
```

Or let the CLI help on first run. For example, if you run:

```bash
go run . start
```

and no config exists yet, `stackctl` will explain that and offer to launch the interactive setup flow.

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

## Config

Manage config after first run with:

```bash
go run . config path
go run . config view
go run . config validate
go run . config edit
go run . config reset
```

An example config is bundled at [examples/config.example.yaml](/home/tylers/Dev/go/github.com/traweezy/stackctl/examples/config.example.yaml).

## Local Stack

The bundled development stack lives at [stacks/dev-stack/compose.yaml](/home/tylers/Dev/go/github.com/traweezy/stackctl/stacks/dev-stack/compose.yaml).

Typical commands:

```bash
go run . start
go run . status
go run . stop
go run . restart
go run . reset --volumes --force
```

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
```

This prints DSNs and URLs derived from the current config.

## Build

The repo is intended to work directly with standard Go tooling:

```bash
go mod tidy
go test ./...
go build ./...
go run . --help
```
