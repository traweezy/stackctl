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
  bash -s -- --version v0.11.0
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

When both Docker Compose and `podman-compose` are installed, `stackctl`
prefers `podman-compose` so Podman-managed stacks do not accidentally route
through a Docker-backed compose provider.

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

Managed service defaults also include:

- Postgres image: `docker.io/library/postgres:16`
- Postgres data volume: `postgres_data`
- Postgres maintenance database for admin helpers: `postgres`
- Redis image: `docker.io/library/redis:7`
- Redis data volume: `redis_data`
- Redis appendonly persistence: disabled
- Redis save policy: `3600 1 300 100 60 10000`
- Redis maxmemory policy: `noeviction`
- pgAdmin image: `docker.io/dpage/pgadmin4:latest`
- pgAdmin data volume: `pgadmin_data`
- pgAdmin server mode: disabled

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
- `stackctl` still handles lifecycle, status, logs, diagnostics, and
  config-aware helper output

That means commands like `stackctl connect` still use your configured service
values even before an external compose file is present. Runtime commands such
as `start`, `stop`, `exec`, `logs`, and `db` still require a real compose file
when they need to talk to containers.

Use external mode if you already have a custom Podman Compose setup and want
`stackctl` to be the operator CLI on top of it.

The example config is in
[`examples/config.example.yaml`](examples/config.example.yaml).

## Typical workflow

Once the stack is configured, the most common day-to-day flow looks like this:

```bash
stackctl start
stackctl tui
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

### `stackctl tui`

Open the interactive terminal dashboard.

The TUI now includes the inspection workflow plus the phase-four config
editor. It gives you a full-screen dashboard for the current stack config and
runtime state, split detail panes for deeper inspection, a first-class config
editing surface, and sidebar actions for `start`, `stop`, `restart`, `doctor`,
`open cockpit`, and `open pgadmin`. Lifecycle and config operations run in the
background, update the header status, and write a session-local action history
you can review inside the dashboard.

Examples:

```bash
stackctl tui
```

Keys:

- `tab`, `l`, `right` to move to the next section
- `shift+tab`, `h`, `left` to move to the previous section
- `j`, `k`, `[`, and `]` to switch the active field, service, or health target inside the current pane
- `1` through `6` to run the sidebar actions
- `enter`, `e` to edit or toggle the selected field in `Config`
- `esc` to cancel an in-progress field edit in `Config`
- `ctrl+s`, `A` to save the current config draft and, when it is safe, refresh managed compose and restart running managed services automatically
- `x` to reset the current config draft
- `u` to apply derived defaults to the current config draft
- `p` to preview the config diff
- `g` to save the current draft and scaffold the managed stack when managed scaffolding is enabled and relevant
- `G` to save the current draft and force-refresh the managed stack scaffold when managed scaffolding is enabled and relevant
- `y`, `enter` to confirm a stop or restart action
- `n`, `esc` to cancel a pending confirmation
- `r` to refresh
- `w` to watch live logs for the selected stack service from `Services` or `Health`
- `a` to toggle auto-refresh for the current TUI session using the configured interval
- `m` to toggle expanded vs compact density
- `s` to show or hide secrets in the dashboard
- `?` to toggle the expanded help footer
- `q`, `esc`, `ctrl+c` to quit

Sections:

- `Overview`: stack paths, mode, stack-managed service counts, and startup behavior
- `Config`: a grouped stack-and-service editor with stack, service, and TUI settings, a slim status strip, a field detail pane, inline validation, allowed-value hints for finite-choice settings, diff preview, save/reset/defaults actions, a key strip under the detail pane, and managed-stack scaffolding
- `Services`: a split service list and detail pane with runtime metadata, lifecycle status, host ports, DSNs, URLs, host-tool handling, copy placeholders, and a live-log shortcut
- `Health`: a split service-by-service health summary with runtime, reachability, doctor detail rendering, and a live-log shortcut for stack services
- `History`: the current session’s action log, including cancellations, warnings, and doctor summaries

Notes:

- auto-refresh is on by default, uses the saved `Config -> TUI` interval, and
  can still be toggled off for the current session inside the TUI
- the left sidebar keeps navigation and global stack actions together so the
  active panel stays focused on inspection
- the `Config` section loads the saved config when it exists, otherwise starts
  from defaults so you can recover from a missing or unreadable config without
  leaving the TUI
- config drafts stay local to the TUI session until you save them; refreshes do
  not overwrite dirty edits
- the slim `Config` status strip now makes draft state explicit, so you can
  always tell whether a change is still draft-only or already written to disk
- the `Config` key strip stays under the detail pane so save/edit controls stay
  visible even in tighter terminals
- `ctrl+s` is the main config action: it always writes the draft first, and for
  managed stacks it also refreshes compose and restarts running services
  automatically when the change can be applied safely
- managed-stack scaffold work is shown as a pending config task instead of a
  hard validation failure, so brand-new installs can save and scaffold cleanly
- when a change cannot be applied safely, the status strip explains the exact
  follow-up, such as save-only, manual compose updates, or stack-target changes
- for external stacks, service, port, and credential edits update stackctl
  metadata and helper commands but do not rewrite your compose file
- the services and health panels only split when the terminal is wide enough;
  medium-width terminals fall back to a stacked layout to avoid cramped
  wrapping
- Cockpit is shown as a host tool, not a stack-managed service, so
  `start`/`stop`/`restart` only apply to the compose stack
- confirmations temporarily take over the center panel instead of pushing the
  layout down
- action results now appear briefly in the global status area, then remain in
  `History`, not in the current inspection tab
- compact mode trims less important runtime fields so the dashboard is easier
  to scan on smaller terminals
- while an action is running, the TUI pauses manual and automatic refresh until
  the action completes and then reloads the snapshot when needed
- `stop` and `restart` require an in-TUI confirmation so accidental key presses
  do not interrupt the running stack
- `doctor` runs diagnostics without applying fixes; it stores the summary and
  any warnings or failures in the history panel
- Cockpit and pgAdmin open as separate sidebar actions so you can launch only
  the UI you want
- the `Services` detail pane shows copy placeholders for DSNs and URLs that
  will become real copy actions in a later phase
- live logs are intentionally handed off to the real compose log stream with
  `w` from `Services` and `Health`, so the TUI stays focused on inspection
  instead of embedding a cramped tail viewer
- returning from a live log watch refreshes the snapshot so service state and
  health data stay current
- the config field list intentionally shortens long values such as stack paths;
  the full value always stays visible in the detail pane and diff preview
- long-running actions usually mean image pulls, Podman startup, or service
  readiness waits; leave the TUI open and let the action finish before forcing
  another lifecycle operation

Flags: `-h`, `--help` only.

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
managed-stack ports, Postgres maintenance-db behavior, Redis persistence and
memory settings, pgAdmin server mode, and service image/data-volume settings
without editing compose files manually.

If you want a full-screen workflow with diff preview, save/reset, and managed
stack scaffolding in one place, use the `Config` section inside `stackctl tui`.

Examples:

```bash
stackctl config edit
stackctl config edit --non-interactive
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--non-interactive` | Save the current config after applying derived defaults |

Service settings available in the config and wizard:

- Postgres: image, data volume, maintenance database, database, username,
  password, container name, and host port
- Redis: image, data volume, password, appendonly persistence, save policy,
  maxmemory policy, container name, and host port
- pgAdmin: image, data volume, email, password, server mode, container name,
  and host port

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
in config, `stackctl` waits for the core app-facing services
(`postgres` and `redis`) before returning.

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
stackctl services --copy postgres
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--copy <target>` | Copy a common DSN, URL, or credential to the clipboard |
| `-j`, `--json` | Print service details as JSON |

Plaintext password fields are intentionally omitted from JSON output. Use the
DSNs, URLs, or the human-readable `stackctl services` output when you need the
configured credentials directly.

Supported copy targets:

- `postgres`
- `redis`
- `pgadmin`
- `cockpit`
- `postgres-user`
- `postgres-password`
- `postgres-database`
- `redis-password`
- `pgadmin-email`
- `pgadmin-password`

### `stackctl exec`

Run a command inside one of the stack services without looking up container
names manually.

Examples:

```bash
stackctl exec postgres -- psql -U app -d app
stackctl exec redis -- redis-cli -a secret PING
stackctl exec pgadmin -- printenv PGADMIN_DEFAULT_EMAIL
```

Supported service targets:

- `postgres` or `pg`
- `redis` or `rd`
- `pgadmin`

Flags:

| Flag | Meaning |
| --- | --- |
| `--no-tty` | Disable TTY allocation for the exec session |

### `stackctl db shell`

Open `psql` against the configured Postgres database without remembering the
database name, username, or container details.

Examples:

```bash
stackctl db shell
stackctl db shell -- -c "select version()"
stackctl db shell -- -tAc "select current_user"
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--no-tty` | Disable TTY allocation for the `psql` session |

### `stackctl db dump`

Dump the configured Postgres database as SQL.

Examples:

```bash
stackctl db dump
stackctl db dump dump.sql
stackctl db dump --output dump.sql
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-o`, `--output` | Write the SQL dump to a file instead of stdout |

### `stackctl db restore`

Restore the configured Postgres database from a SQL dump.

Examples:

```bash
stackctl db restore dump.sql --force
stackctl db restore - --force < dump.sql
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-f`, `--force` | Skip confirmation before applying the SQL dump |

### `stackctl db reset`

Drop and recreate the configured Postgres database.

Examples:

```bash
stackctl db reset --force
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-f`, `--force` | Skip confirmation before dropping and recreating the database |

### `stackctl ports`

Show the configured host-to-service port mappings quickly. When runtime data is
available, `stackctl` also fills in the discovered internal service ports.

Examples:

```bash
stackctl ports
```

Flags: `-h`, `--help` only.

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

Run diagnostics for the local stack and surrounding environment. Use
`--fix` when you want `stackctl` to apply the supported automatic fixes.

Examples:

```bash
stackctl doctor
stackctl doctor --fix --yes
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--fix` | Try to apply supported fixes for doctor findings |
| `-y`, `--yes` | Assume yes for automatic fix prompts |

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
- `stackctl services --copy postgres`: send a ready-to-use value straight to the clipboard
- `stackctl db shell`: jump straight into `psql`
- `stackctl db dump`: export the local database as SQL
- `stackctl db restore`: replay a SQL dump into the local database
- `stackctl db reset`: recreate the configured database cleanly
- `stackctl ports`: check host-to-service port mappings quickly
- `stackctl logs --watch`: keep a live log tail open while developing
- `stackctl doctor`: diagnose environment and port problems
- `stackctl doctor --fix`: apply the safe, supported automatic fixes

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

### Clipboard copy fails

`stackctl services --copy ...` needs a clipboard tool on Linux.

Install one of:

- `wl-copy` for Wayland sessions
- `xclip` or `xsel` for X11 sessions

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
git tag v0.11.0
git push origin v0.11.0
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
- service-level image, data-volume, and tuning settings in `stackctl config`
- configurable Postgres maintenance-database settings for admin helpers
- configurable Redis persistence and maxmemory policy settings
- configurable pgAdmin server-mode settings
- external-stack configs stay valid for non-runtime commands even before a
  compose file is present
- machine-readable service output with `stackctl services --json`
- clipboard-friendly service helpers with `stackctl services --copy <target>`
- `stackctl exec <service> -- <command...>` for in-container workflows
- `stackctl db shell` for one-step Postgres access
- `stackctl db dump`, `stackctl db restore`, and `stackctl db reset`
  for repeatable local database workflows
- `stackctl ports` for quick port inspection
- `stackctl doctor --fix` for supported automatic environment remediation
- live Podman integration coverage for the managed-stack lifecycle

Current CLI surface:

- `setup`
- `doctor`
- `start`, `stop`, `restart`
- `status`
- `services`
- `ports`
- `db`
- `exec`
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

### Next after that

These are strong follow-ups once the high-priority local stack and helper
commands are in place.

- deeper service controls such as Redis ACL users, richer Postgres tuning,
  and pgAdmin server bootstrap helpers
- multi-stack support such as `stackctl start dev` and
  `stackctl start staging`
- `stackctl env` to print app-ready environment variables
- `stackctl run ...` to launch an app with stack-aware context
- snapshot save and restore commands for dev-state workflows
- broader installer support beyond `apt`-based systems
- more explicit verbosity and quiet controls across runtime commands

### Longer-term UX work

These are appealing, but not ahead of the higher-value local-platform work.

- TUI power-user workflows such as copy actions, a command palette, and quick jumps
- a self-update flow such as `stackctl update`
- a plugin model for optional service packs
- non-Linux support once runtime and install behavior are abstracted well

### Not planned right now

These are intentionally out of scope for the near term:

- Kafka
- Elasticsearch
- Kubernetes integration
