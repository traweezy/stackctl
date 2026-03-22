# stackctl

`stackctl` is a CLI for running and managing a complete local backend stack
with zero setup.

```bash
stackctl start
```

This gives you:

- PostgreSQL
- Redis
- pgAdmin
- ready-to-use connection strings

No manual configuration required.

## Why stackctl

Most local dev setups require:

- managing compose files manually
- remembering ports and credentials
- writing custom scripts

stackctl provides:

- interactive setup
- consistent service management
- built-in connection helpers
- diagnostics and health checks

All in a single CLI.

> [!IMPORTANT]
> `stackctl` is Linux-only right now.
>
> - release binaries are published for Linux `x86_64` and `arm64`
> - the installer script only supports Linux
> - `setup --install` currently targets `apt`-based systems
> - macOS and Windows are not supported yet

## What stackctl is for

Use `stackctl` if you want one CLI that answers:

- is my local stack configured
- is it running
- how do I connect to it
- which service is broken
- where are the stack files on disk

## Who this is for

stackctl is useful if you:

- want a ready-to-use local backend stack
- are tired of managing docker-compose files manually
- need consistent local infrastructure across projects
- want quick access to connection details and logs

The runtime inspection commands are intentionally split:

- `stackctl connect` prints minimal connection strings and URLs
- `stackctl services` prints the full service report with status, ports,
  container names, URLs, and DSNs
- `stackctl status` prints raw container state
- `stackctl doctor` checks the environment and expected ports
- `stackctl health` checks whether the configured endpoints are reachable

## Install

### Quick install (recommended)

### Quick install from GitHub Releases

Install the latest release to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | bash
```

Install to `/usr/local/bin` instead:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | \
  bash -s -- --system
```

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | \
  bash -s -- --version v0.3.0
```

If `~/.local/bin` is not already on your `PATH`, add it:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Manual download

Download release artifacts directly from:

https://github.com/traweezy/stackctl/releases/latest

### Build from source

```bash
git clone https://github.com/traweezy/stackctl.git
cd stackctl
go build ./...
go run . --help
```

## Requirements

For normal runtime usage, `stackctl` expects:

- Linux
- `podman`
- `podman compose`

If you want the CLI to install supported dependencies for you:

```bash
stackctl setup --install
```

That flow is currently aimed at `apt`-based systems.

## Quick start

If you want the shortest path from a clean Linux machine to a running local
stack:

```bash
stackctl setup
stackctl start
stackctl services
stackctl logs --watch
```

If you are running from source instead of an installed binary, replace
`stackctl` with `go run .`.

## Example

```bash
stackctl start
stackctl connect
```

```text
Postgres
  postgres://app:app@localhost:5432/app

Redis
  redis://localhost:6379
```

### First run

On first run, `stackctl` can:

- create `~/.config/stackctl/config.yaml`
- scaffold a managed compose file under your XDG data directory
- validate the local environment

Common first-run commands:

```bash
stackctl setup
stackctl config init
stackctl start
```

Common setup variants:

```bash
stackctl setup --non-interactive
stackctl setup --install
stackctl setup --install --yes
```

## Defaults

The managed stack uses these default connection values unless you change the
config:

- host: `localhost`
- Postgres database: `app`
- Postgres username: `app`
- Postgres password: `app`
- Redis password: disabled by default
- Postgres port: `5432`
- Redis port: `6379`
- pgAdmin URL: `http://localhost:8081`
- Cockpit URL: `https://localhost:9090`

The managed pgAdmin service also ships with these default credentials:

- email: `admin@example.com`
- password: `admin`

Change them with `stackctl config edit` or by editing
`~/.config/stackctl/config.yaml` directly.

## Where files live

By default, `stackctl` uses standard Linux user directories:

- config file: `~/.config/stackctl/config.yaml`
- managed data root: `~/.local/share/stackctl`
- managed stack directory: `~/.local/share/stackctl/stacks/dev-stack`
- managed compose file:
  `~/.local/share/stackctl/stacks/dev-stack/compose.yaml`

If `XDG_DATA_HOME` is set, managed stack data is stored under
`$XDG_DATA_HOME/stackctl` instead.

## Managed stack vs external stack

`stackctl` supports two stack modes.

### Managed stack

In managed mode:

- `stackctl` owns the stack directory
- `stackctl config scaffold` can create or refresh the compose file
- the managed compose file is rendered from your config values, including
  Postgres credentials, optional Redis auth, and pgAdmin login details

This is the default and the easiest way to get started.

### External stack

In external mode:

- your config points at an existing compose directory
- you manage the compose file yourself
- `stackctl` still handles lifecycle, status, logs, and diagnostics

Use external mode if you already have a custom Podman Compose setup and want
`stackctl` to be the operator CLI on top of it.

The example config is in
[`examples/config.example.yaml`](examples/config.example.yaml).

## Typical workflow

Once the stack is configured, the most common day-to-day flow looks like this:

```bash
stackctl start
stackctl services
stackctl connect
stackctl logs --watch
stackctl health
```

## Command reference

This section documents the current CLI surface exactly, including flags.

### Top-level usage

```bash
stackctl [command]
```

Root flags:

| Flag | Meaning |
| --- | --- |
| `-h`, `--help` | Show help for `stackctl` |
| `-v`, `--version` | Print the short version string |

### `stackctl setup`

Prepare the local machine and the `stackctl` config.

Examples:

```bash
stackctl setup
stackctl setup --non-interactive
stackctl setup --install --yes
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--install` | Install supported missing dependencies |
| `--interactive` | Force interactive config setup |
| `--non-interactive` | Skip prompts and use defaults where possible |
| `--yes` | Assume yes for installation prompts |

### `stackctl config`

Manage persistent stack configuration.

Examples:

```bash
stackctl config view
stackctl config validate
stackctl config scaffold --force
```

Subcommands:

| Subcommand | What it does |
| --- | --- |
| `config init` | Create a new config |
| `config view` | Print the current config as YAML |
| `config path` | Print the resolved config path |
| `config edit` | Edit the current config using the wizard |
| `config validate` | Validate the current config |
| `config reset` | Reset the config to defaults or delete it |
| `config scaffold` | Create or refresh managed stack files |

#### `stackctl config init`

Create a new config file.

Examples:

```bash
stackctl config init
stackctl config init --non-interactive
stackctl config init --force
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--force` | Overwrite an existing config without prompting |
| `--non-interactive` | Create the config from defaults without prompts |

#### `stackctl config view`

Print the current config in YAML format.

Examples:

```bash
stackctl config view
```

Flags: `-h`, `--help` only.

#### `stackctl config path`

Print the resolved config path.

Examples:

```bash
stackctl config path
```

Flags: `-h`, `--help` only.

#### `stackctl config edit`

Edit the current config using the interactive wizard.

This is the easiest way to change service credentials, optional Redis auth,
and managed-stack ports without editing compose files manually.

Examples:

```bash
stackctl config edit
stackctl config edit --non-interactive
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--non-interactive` | Save the current config after applying derived defaults |

#### `stackctl config validate`

Validate the current config.

Examples:

```bash
stackctl config validate
```

Flags: `-h`, `--help` only.

#### `stackctl config reset`

Reset the config to defaults or delete it.

Examples:

```bash
stackctl config reset
stackctl config reset --delete --force
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--delete` | Delete the config file instead of resetting it |
| `--force` | Skip confirmation |
| `--yes` | Assume yes for confirmation prompts |

#### `stackctl config scaffold`

Create or refresh the managed stack files from embedded templates.

Examples:

```bash
stackctl config scaffold
stackctl config scaffold --force
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--force` | Overwrite managed stack files from embedded templates |

### `stackctl start`

Start the local development stack. If `wait_for_services_on_start` is enabled
in config, `stackctl` waits for the configured ports before returning.

Examples:

```bash
stackctl start
```

Flags: `-h`, `--help` only.

### `stackctl stop`

Stop the local development stack.

Examples:

```bash
stackctl stop
```

Flags: `-h`, `--help` only.

### `stackctl restart`

Restart the local development stack and print the current connection info when
it is ready.

Examples:

```bash
stackctl restart
```

Flags: `-h`, `--help` only.

### `stackctl status`

Show container status for this stack. Use this when you want the raw container
view.

Examples:

```bash
stackctl status
stackctl status --verbose
stackctl status --json
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-j`, `--json` | Print container status as JSON |
| `-v`, `--verbose` | Show extra container details |

### `stackctl services`

Show the full service report for the configured stack.

This is the command to use when you want to answer all of these at once:

- what is running
- what container is backing it
- what host and port it is using
- how to connect to it

Examples:

```bash
stackctl services
stackctl services --json
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-j`, `--json` | Print service details as JSON |

### `stackctl connect`

Print minimal connection strings and URLs. This is intentionally smaller than
`stackctl services` and is meant for copy/paste.

Examples:

```bash
stackctl connect
```

Flags: `-h`, `--help` only.

### `stackctl logs`

Show recent logs or follow them live.

Examples:

```bash
stackctl logs
stackctl logs --watch
stackctl logs --service postgres
stackctl logs --service pg --tail 200 --watch
```

Supported service filters:

- `postgres` or `pg`
- `redis` or `rd`
- `pgadmin`

Flags:

| Flag | Meaning |
| --- | --- |
| `-s`, `--service` | Filter logs to a single service |
| `--since` | Show logs since a relative time or timestamp |
| `-n`, `--tail` | Number of log lines to show. Default: `100` |
| `-w`, `--watch` | Follow logs |

### `stackctl health`

Check whether the configured stack endpoints are reachable.

Examples:

```bash
stackctl health
stackctl health --watch --interval 2
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-i`, `--interval` | Watch interval in seconds. Default: `5` |
| `-w`, `--watch` | Continuously rerun health checks |

### `stackctl doctor`

Run read-only diagnostics for the local stack and surrounding environment.

Examples:

```bash
stackctl doctor
```

Flags: `-h`, `--help` only.

### `stackctl open`

Open configured web UIs. If browser launch is unavailable, `stackctl` prints
the URL instead.

Examples:

```bash
stackctl open
stackctl open cockpit
stackctl open pgadmin
stackctl open all
```

Flags: `-h`, `--help` only.

Arguments:

| Argument | Meaning |
| --- | --- |
| `cockpit` | Open Cockpit |
| `pgadmin` | Open pgAdmin |
| `all` | Open every enabled web UI |

### `stackctl reset`

Bring the stack down and optionally remove volumes.

Examples:

```bash
stackctl reset
stackctl reset --volumes --force
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-f`, `--force` | Skip confirmation for destructive reset |
| `-v`, `--volumes` | Remove volumes while stopping the stack |

### `stackctl version`

Print version information.

Examples:

```bash
stackctl version
```

Flags: `-h`, `--help` only.

## What the main commands are for

If you only remember a few commands, these are the ones most people will use:

- `stackctl setup`: create config and prepare the machine
- `stackctl start`: bring the stack up
- `stackctl services`: see the full runtime picture
- `stackctl connect`: copy DSNs and URLs quickly
- `stackctl logs --watch`: keep a live log tail open while developing
- `stackctl doctor`: diagnose environment and port problems

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

If you want to force a refresh of the managed compose file:

```bash
stackctl config scaffold --force
```

Then validate the config:

```bash
stackctl config validate
```

### A service port is already in use

Run:

```bash
stackctl doctor
```

That will tell you whether the port belongs to the expected local service or
to some unrelated process.

### The stack looks up but a service is not healthy

Use these in order:

```bash
stackctl status
stackctl services
stackctl health
stackctl logs --watch
```

### Browser launch fails

Run:

```bash
stackctl open
```

If browser launch is unavailable, `stackctl` prints the URL instead of
failing.

## Development

For local development on `stackctl` itself:

```bash
go test ./...
go test ./integration -tags=integration -count=1
go build ./...
go run . --help
```

The default test suite includes unit tests, script-driven CLI tests, and
interactive PTY coverage for the config wizard. The integration suite is
Linux-only and runs the real binary against real Podman-managed services in
isolated temp XDG directories.

## Release flow

Releases are created from tags that match `v*`.

Example:

```bash
git tag v0.3.0
git push origin v0.3.0
```

## Roadmap

stackctl is moving toward:

> a local backend platform for developers

The direction is pragmatic:

- real local-development usefulness first
- lightweight services with fast startup
- strong developer experience without turning the tool into a giant platform

### Current platform

Available today:

- PostgreSQL
- Redis
- pgAdmin
- Cockpit
- configurable Postgres database, username, password, and ports
- optional Redis auth that flows through generated compose and DSNs
- configurable pgAdmin login details that stay in sync with the managed stack
- machine-readable service output with `stackctl services --json`
- live Podman integration coverage for the managed-stack lifecycle

Current CLI surface:

- `setup`
- `doctor`
- `start`, `stop`, `restart`
- `status`
- `services`
- `logs`
- `health`
- `connect`
- `reset`
- `config`

### High priority

These are the highest-value additions after the current release line.

#### More local services

- `NATS`
  Why: lightweight messaging for event-driven systems, workers, and
  real-time backends
- `MinIO`
  Why: S3-compatible object storage for uploads and cloud-like local dev
- `Meilisearch`
  Why: fast, lightweight search and autocomplete without Elasticsearch

Resulting target stack:

- PostgreSQL
- Redis
- NATS
- MinIO
- Meilisearch
- pgAdmin
- Cockpit

#### More day-to-day CLI helpers

- `stackctl services --copy <target>`
  Why: quick copy helpers for DSNs and URLs
- `stackctl exec <service> ...`
  Why: run commands inside containers without remembering container names
- `stackctl db shell`, `stackctl db reset`, `stackctl db dump`,
  `stackctl db restore`
  Why: streamline common Postgres workflows
- `stackctl ports`
  Why: show host-to-service port mappings quickly
- `stackctl doctor --fix`
  Why: automate safe fixes for common local-environment issues

### Next after that

These are strong follow-ups once the high-priority local stack and helper
commands are in place.

#### Broader service reconfiguration

- first-class per-service settings beyond the current credential fields
  Why: support richer Postgres, Redis, and pgAdmin customization without
  forcing users back into manual compose edits
- richer Redis configuration such as ACLs, named users, persistence, and
  memory-policy defaults
  Why: local environments often need more than a single optional password
- broader Postgres and pgAdmin configuration controls
  Why: users will need database names, credentials, ports, admin logins,
  and common service-level tuning knobs to stay inside `stackctl config`
- better support for non-managed custom stacks
  Why: external compose users should be able to keep using `stackctl`
  without losing as much configuration awareness

- multi-stack support such as `stackctl start dev` and
  `stackctl start staging`
- `stackctl env` to print app-ready environment variables
- `stackctl run ...` to launch an app with stack-aware context
- snapshot save and restore commands for dev-state workflows
- broader installer support beyond `apt`-based systems
- more explicit verbosity and quiet controls across runtime commands

### Longer-term UX work

These are appealing, but not ahead of the higher-value local-platform work.

- a TUI dashboard such as `stackctl ui`
- a self-update flow such as `stackctl update`
- a plugin model for optional service packs
- non-Linux support once runtime and install behavior are abstracted well

### Not planned right now

These are intentionally out of scope for the near term:

- Kafka
- Elasticsearch
- Kubernetes integration
