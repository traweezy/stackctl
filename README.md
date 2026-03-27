# stackctl

`stackctl` is a CLI for running and managing a complete local backend stack
with zero setup.

```bash
stackctl start
```

This gives you:

- PostgreSQL
- Redis
- NATS
- pgAdmin
- ready-to-use connection strings

You can also opt into:

- SeaweedFS for S3-compatible local object storage

No manual configuration required. The setup wizard and config editor also let
you enable or disable each service explicitly.

## Why stackctl

Most local dev setups require:

- managing compose files manually
- remembering ports and credentials
- writing custom scripts

stackctl provides:

- interactive setup wizard with service checkboxes, inline hints, and review
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

- `stackctl connect` prints minimal connection strings, URLs, and enabled
  object-storage endpoint credentials
- `stackctl env` prints app-ready environment variables, with shell-export and
  JSON modes
- `stackctl services` prints the full service report with status, ports,
  container names, endpoints, URLs, and DSNs
- `stackctl status` prints raw container state
- `stackctl doctor` checks the environment and expected ports
- `stackctl health` checks whether the configured endpoints are reachable

## Install

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
  bash -s -- --version v0.17.0
```

The installer downloads the release archive plus `checksums.txt` and verifies
the archive checksum before extracting it.

If `~/.local/bin` is not already on your `PATH`, add it:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Manual download

Download release artifacts directly from:

https://github.com/traweezy/stackctl/releases/latest

If you install manually, verify the archive against the release `checksums.txt`
before extracting it.

### Build from source

```bash
git clone https://github.com/traweezy/stackctl.git
cd stackctl
go build -trimpath -o dist/stackctl .
./dist/stackctl --help
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
stackctl env --export
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

NATS
  nats://stackctl@localhost:4222

pgAdmin
  http://localhost:8081

Cockpit
  https://localhost:9090
```

### First run

On first run, `stackctl` can:

- create `~/.config/stackctl/config.yaml` for the default stack or
  `~/.config/stackctl/stacks/<name>.yaml` for a named stack
- scaffold a managed compose file under your XDG data directory
- validate the local environment

Common first-run commands:

```bash
stackctl setup
stackctl config init
stackctl --stack staging config init --non-interactive
stackctl start
```

Common setup variants:

```bash
stackctl setup --non-interactive
stackctl setup --install
stackctl setup --install --yes
```

If you want a true clean slate before walking through setup again, use
`stackctl factory-reset --force`. This is destructive and removes every
stackctl-owned config and managed-stack directory for every stack.

## Defaults

The managed stack uses these default connection values unless you change the
config:

- host: `localhost`
- Postgres database: `app`
- Postgres username: `app`
- Postgres password: `app`
- Redis password: disabled by default
- NATS token: `stackctl`
- SeaweedFS access key: `stackctl`
- SeaweedFS secret key: `stackctlsecret`
- Postgres port: `5432`
- Redis port: `6379`
- NATS port: `4222`
- SeaweedFS endpoint: `http://localhost:8333`
- pgAdmin URL: `http://localhost:8081`
- Cockpit URL: `https://localhost:9090`

Service toggles default to enabled for:

- Postgres
- Redis
- NATS
- pgAdmin
- Cockpit helpers

Disabled by default:

- SeaweedFS

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
- NATS image: `docker.io/library/nats:2.12.5`
- SeaweedFS image:
  `docker.io/chrislusf/seaweedfs:4.17@sha256:186de7ef977a20343ee9a5544073f081976a29e2d29ecf8379891e7bf177fbe9`
- SeaweedFS data volume: `seaweedfs_data`
- SeaweedFS volume size limit: `1024 MB`
- pgAdmin image: `docker.io/dpage/pgadmin4:latest`
- pgAdmin data volume: `pgadmin_data`
- pgAdmin server mode: disabled

Change them with `stackctl config edit` or by editing the current stack config
returned by `stackctl config path`. For named stacks, select the target first
with `--stack <name>`, `STACKCTL_STACK=<name>`, or `stackctl stack use <name>`.

## Where files live

By default, `stackctl` uses standard Linux user directories:

- default stack config file: `~/.config/stackctl/config.yaml`
- named stack config files: `~/.config/stackctl/stacks/<name>.yaml`
- saved current stack selection: `~/.config/stackctl/current-stack`
- managed data root: `~/.local/share/stackctl`
- default managed stack directory: `~/.local/share/stackctl/stacks/dev-stack`
- named managed stack directory: `~/.local/share/stackctl/stacks/<name>`
- managed compose file:
  `~/.local/share/stackctl/stacks/dev-stack/compose.yaml`
- managed NATS config file:
  `~/.local/share/stackctl/stacks/dev-stack/nats.conf`

If `XDG_DATA_HOME` is set, managed stack data is stored under
`$XDG_DATA_HOME/stackctl` instead.

Use `stackctl config path` to print the exact config file for the current stack.
Use `stackctl --stack <name> config path`,
`STACKCTL_STACK=<name> stackctl config path`, or `stackctl stack use <name>`
plus `stackctl config path` for named stacks.

Non-default managed stacks also derive stack-specific container and volume
names, such as `stackctl-staging-postgres` and
`stackctl-staging-postgres-data`, so their managed resources do not collide
with the default stack on disk or in Podman.

## Managed stack vs external stack

`stackctl` supports two stack modes.

### Managed stack

In managed mode:

- `stackctl` owns the stack directory
- `stackctl config scaffold` can create or refresh the managed stack files
- the managed compose file is rendered from your config values, including
  Postgres credentials, optional Redis auth, the NATS token, optional
  SeaweedFS S3 credentials, and pgAdmin login details

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

Example configs:

- managed-stack example:
  [`examples/config.example.yaml`](examples/config.example.yaml)
- external-stack example:
  [`examples/external-stack.example.yaml`](examples/external-stack.example.yaml)

## Typical workflow

Once the stack is configured, the most common day-to-day flow looks like this:

```bash
stackctl start
stackctl tui
stackctl services
stackctl connect
stackctl env --export
stackctl health
stackctl logs --watch
```

If you want separate local environments, create named stacks and either target
them ad hoc with `--stack` or make one the saved default with `stack use`:

```bash
stackctl stack use staging
stackctl config init --non-interactive
stackctl start
stackctl services
stackctl stop
```

Only one local stack is allowed to run at a time. If `staging` is running,
`stackctl start` for the default stack or another named stack will tell you
which stack to stop first.

## Command reference

This section documents the current CLI surface exactly, including flags.

### Top-level usage

```bash
stackctl [flags] [command]
```

Root flags:

| Flag | Meaning |
| --- | --- |
| `--stack` | Select a named stack config for this command only. The default stack uses `~/.config/stackctl/config.yaml`; named stacks use `~/.config/stackctl/stacks/<name>.yaml`. |
| `-h`, `--help` | Show help for `stackctl` |
| `-v`, `--version` | Print the short version string |

Selection precedence is:

1. `--stack`
2. `STACKCTL_STACK`
3. the saved current stack from `stackctl stack use`
4. the default stack (`dev-stack`)

Stack names must use lowercase letters, numbers, hyphens, or underscores.

`stackctl --help` now groups commands into lifecycle, inspect, operate,
setup/config, and utility sections so the command surface is easier to scan.

### `stackctl tui`

Open the interactive terminal dashboard.

The TUI now includes the inspection workflow plus the phase-four config
editor. It gives you a full-screen dashboard for the current stack config and
runtime state, split detail panes for deeper inspection, a first-class config
editing surface, and sidebar actions for `start`, `stop`, `restart`, `doctor`,
`open cockpit`, and `open pgadmin`. In `Services` and `Health`, the selected
stack service also gets direct start, stop, and restart actions ahead of the
global stack actions. Lifecycle and config operations run in the background,
update the header status, and write a session-local action history you can
review inside the dashboard. The header now carries clearer section context,
`Services` includes a quick running/stopped/attention summary, and the detail
panes use explicit subsections so service inspection is easier to scan. The
`Stacks` section gives you a proper profile browser inside the TUI so you can
inspect saved stacks, switch the active stack, start or stop selected stack
profiles, and delete profiles without leaving the dashboard.

Examples:

```bash
stackctl tui
```

Keys:

- `tab`, `l`, `right` to move to the next section
- `shift+tab`, `h`, `left` to move to the previous section
- `j`, `k`, `[`, and `]` to switch the active field, stack profile, service, or health target inside the current pane
- `1` through `9` to run the sidebar actions
- `enter`, `e` to edit or toggle the selected field in `Config`
- `esc` to cancel an in-progress field edit in `Config`
- `ctrl+s`, `A` to save the current config draft and, when it is safe, refresh managed compose and restart running managed services automatically
- `x` to reset the current config draft
- `u` to apply derived defaults to the current config draft
- `p` in `Config` to preview the config diff, or in `Services` and `Health` to pin or unpin the selected service
- `g` in `Config` to save the current draft and scaffold the managed stack when managed scaffolding is enabled and relevant, or elsewhere to jump to a stack profile or service
- `G` to save the current draft and force-refresh the managed stack scaffold when managed scaffolding is enabled and relevant
- `y`, `enter` to confirm a stop or restart action
- `n`, `esc` to cancel a pending confirmation
- `r` to refresh
- `:`, `ctrl+k` to open the command palette
- `/` to open the search and jump picker for the current pane
- `c` to copy a DSN, URL, port, or credential from the selected service
- `e` to open a shell inside the selected stack service
- `d` to open `psql` for the selected Postgres service
- `w` to watch live logs for the selected stack service from `Services` or `Health`
- `a` to toggle auto-refresh for the current TUI session using the configured interval
- `m` to toggle expanded vs compact density
- `s` to show or hide secrets in the dashboard
- `?` to toggle the expanded help footer
- `q`, `esc`, `ctrl+c` to quit

Sections:

- `Overview`: stack paths, mode, stack-managed service counts, and startup behavior
- `Config`: a grouped stack-and-service editor with stack, service, and TUI settings, a slim status strip, a field detail pane, inline validation, allowed-value hints for finite-choice settings, diff preview, save/reset/defaults actions, a key strip under the detail pane, and managed-stack scaffolding
- `Services`: a split service list and detail pane with runtime metadata, lifecycle status, host ports, DSNs, URLs, host-tool handling, pinned-service grouping, real copy actions, shell shortcuts, and a live-log shortcut
- `Health`: a split service-by-service health summary with runtime, reachability, doctor detail rendering, pinned-service grouping, and the same service shortcuts for stack services
- `Stacks`: a split stack-profile browser with per-profile lifecycle, switch, and delete actions
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
- `Services` and `Health` now use a service action strip instead of copy
  placeholders, so copy, logs, shell, db shell, pin, jump, and the command
  palette are all visible where you need them
- live logs are intentionally handed off to the real compose log stream with
  `w` from `Services` and `Health`, so the TUI stays focused on inspection
  instead of embedding a cramped tail viewer
- returning from a live log watch refreshes the snapshot so service state and
  health data stay current
- `c` opens a picker of real values from the selected service, including DSNs,
  URLs, host ports, usernames, passwords, and databases when they exist
- `:` or `ctrl+k` opens a fuzzy command palette that can rerun recent actions,
  jump to sections, jump to services, trigger lifecycle actions, copy
  stack-wide `connect` / `env --export` / `ports` output, and open service-level
  helpers
- `g` or `/` opens the service jump picker directly, with pinned services shown
  first
- `p` pins the selected service for the current session so it stays at the top
  of the `Services` and `Health` target lists
- `e` hands off to an interactive shell inside the selected stack service, and
  `d` jumps straight into `psql` when Postgres is selected
- the config field list intentionally shortens long values such as stack paths;
  the full value always stays visible in the detail pane and diff preview
- long-running actions usually mean image pulls, Podman startup, or service
  readiness waits; leave the TUI open and let the action finish before forcing
  another lifecycle operation

Flags: `-h`, `--help` only.

### TUI productivity workflows

- use `:` or `ctrl+k` to open the command palette, then type a few characters to
  fuzzy-filter sections, service helpers, lifecycle actions, recent actions,
  and stack-wide `connect` / `env --export` / `ports` copy helpers
- use `g` or `/` to jump straight to a service, with pinned services listed
  first
- use `c` from `Services` or `Health` to copy the selected service's real
  values instead of retyping DSNs, URLs, ports, usernames, passwords, or
  database names
- use `e` from `Services` or `Health` to open an interactive shell in the
  selected container
- use `d` when Postgres is selected to jump straight into `psql`
- use `p` to pin the current service for the session so the most-used targets
  stay at the top of the service lists and jump picker
- use `w` to hand off to the full live log stream when you need more than an
  inspection snapshot

### `stackctl setup`

Prepare the local machine and the `stackctl` config.

When setup runs interactively, it opens the full-screen wizard: choose the
stack mode, pick services with a checkbox list, fill only the enabled-service
pages, and see step-by-step progress all the way through the final review
before saving. Set `ACCESSIBLE=1` to use the same wizard in accessible
prompting mode, or `STACKCTL_WIZARD_PLAIN=1` to force the legacy plain prompt
flow.

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

Interactive `config init` uses the same page-based wizard as `stackctl setup`:
stack details first, service selection next, then only the enabled service
pages, followed by a review screen before the config is written.

Examples:

```bash
stackctl config init
stackctl config init --non-interactive
stackctl config init --force
stackctl --stack staging config init --non-interactive
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--force` | Overwrite an existing config without prompting |
| `--non-interactive` | Create the config from defaults without prompts |

Named stacks:

- the default stack resolves to `~/.config/stackctl/config.yaml`
- named stacks resolve to `~/.config/stackctl/stacks/<name>.yaml`
- `stackctl stack use <name>` persists the current stack for later commands
- `--stack <name>` and `STACKCTL_STACK=<name>` both override that saved choice
- stack names use lowercase letters, numbers, hyphens, or underscores
- managed named stacks scaffold under `~/.local/share/stackctl/stacks/<name>`
- only one local stack is allowed to run at a time; stop the current one
  before starting another

### `stackctl stack`

Manage named stack profiles and the saved current stack selection.

Examples:

```bash
stackctl stack list
stackctl stack current
stackctl stack use staging
stackctl stack clone dev-stack demo
stackctl stack rename demo qa
stackctl stack delete qa --purge-data --force
```

Subcommands:

| Subcommand | What it does |
| --- | --- |
| `stack list` | List configured stacks and the active selection |
| `stack current` | Print the active stack selection |
| `stack use` | Persist a stack as the default selection |
| `stack delete` | Delete a stack config and optionally purge managed data |
| `stack rename` | Rename a stack profile |
| `stack clone` | Copy a stack profile into a new stack |

Use `stackctl config edit`, `stackctl config view`, `stackctl config validate`,
or `stackctl tui` after selecting a stack when you want to modify or inspect
that stack in depth.

#### `stackctl stack list`

List configured stack profiles and the active selection.

The table shows which stack is active, whether each profile is managed or
external, whether it looks running, which services are currently running, or
which stack services are configured when the profile is stopped, and which
config path owns it.

#### `stackctl stack current`

Print the active stack selection after applying the same precedence as the rest
of the CLI.

#### `stackctl stack use`

Persist a stack as the default selection for later commands. Choosing
`dev-stack` clears the saved selector file and falls back to the implicit
default.

Examples:

```bash
stackctl stack use staging
stackctl stack use dev-stack
```

#### `stackctl stack delete`

Delete a stack profile config. Add `--purge-data` to also stop and remove the
stackctl-managed local stack directory and volumes for that profile.

Without `--purge-data`, deleting a managed stack only removes the config file.
The command warns when managed local data is still present. If the stack is
currently running, `stack delete` refuses to continue unless you also request
`--purge-data`.

#### `stackctl stack rename`

Rename a stack profile. For managed stacks, this also moves the managed stack
directory, refreshes the generated compose files, and updates the saved current
selection if the renamed stack was active.

#### `stackctl stack clone`

Clone a stack profile into a new stack. For managed stacks, the cloned profile
gets its own managed directory, container names, and volume names so it does
not collide with the source stack.

#### `stackctl config view`

Print the current config in YAML format.

Examples:

```bash
stackctl config view
stackctl --stack staging config view
```

Flags: `-h`, `--help` only.

#### `stackctl config path`

Print the resolved config path.

Examples:

```bash
stackctl config path
stackctl --stack staging config path
```

Flags: `-h`, `--help` only.

#### `stackctl config edit`

Edit the current config using the interactive wizard.

This is the easiest way to change service credentials, optional Redis auth,
the managed NATS token, optional SeaweedFS S3 credentials, managed-stack
ports, Postgres maintenance-db behavior, Redis persistence and memory settings,
SeaweedFS volume sizing, pgAdmin server mode, and service image/data-volume
settings without editing compose files manually. All managed services can also
be enabled or disabled here. The wizard now starts with stack mode and a
checkbox-style service picker, then only shows configuration pages for the
services you selected. Each page includes inline hints so the common fields
read more like the TUI than raw YAML keys.

If you want a full-screen workflow with diff preview, save/reset, and managed
stack scaffolding in one place, use the `Config` section inside `stackctl tui`.
Use `--stack <name>` when you want the wizard to edit a named stack instead of
the default stack.

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

- Postgres: enabled flag, image, data volume, maintenance database, database,
  username, password, container name, and host port
- Redis: enabled flag, image, data volume, password, appendonly persistence,
  save policy, maxmemory policy, container name, and host port
- NATS: enabled flag, image, auth token, container name, and host port
- SeaweedFS: enabled flag, image, data volume, volume size limit, access key,
  secret key, container name, and host port
- pgAdmin: enabled flag, image, data volume, email, password, server mode,
  container name, and host port
- Cockpit: enabled flag, install-on-setup flag, and host port

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
stackctl --stack staging config reset --force
stackctl config reset --delete --force
```

Without `--delete`, reset rewrites the selected stack config back to defaults
and refreshes the managed scaffold when that stack uses managed mode.

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

Start the local development stack or selected stack services. If
`wait_for_services_on_start` is enabled in config, `stackctl` waits for the
core app-facing services (`postgres`, `redis`, `nats`, and `seaweedfs` when
enabled) that are part of the requested start operation before returning.
Disabled services are skipped automatically.

Valid service targets are the enabled stack-managed services: `postgres`,
`redis`, `nats`, `seaweedfs`, and `pgadmin`. Cockpit is a host helper, not a compose
service, so it is never a lifecycle target. `start` also refuses to run when
another local stack is already running and tells you which stack to stop first.

Examples:

```bash
stackctl start
stackctl --stack staging start
stackctl start postgres
stackctl start redis nats
```

Flags: `-h`, `--help` only.

### `stackctl stop`

Stop the local development stack or selected stack services.

Service arguments only target stack-managed services. Cockpit is not part of
the compose lifecycle.

Examples:

```bash
stackctl stop
stackctl --stack staging stop postgres
stackctl stop redis nats
```

Flags: `-h`, `--help` only.

### `stackctl restart`

Restart the local development stack or selected stack services and print the
current connection info when it is ready.

Like `start`, `restart` only targets stack-managed services and refuses to
cross-start a second local stack while another one is running.

Examples:

```bash
stackctl restart
stackctl --stack staging restart postgres
stackctl restart redis nats
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
- which credentials go with it

Examples:

```bash
stackctl services
stackctl services --json
stackctl services --copy postgres
stackctl services --copy nats
stackctl services --copy seaweedfs
stackctl services --copy seaweedfs-secret-key
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--copy <target>` | Copy a common DSN, URL, or credential to the clipboard |
| `-j`, `--json` | Print service details as JSON |

Plaintext secret fields are intentionally omitted from JSON output. Passwords,
tokens, and secret keys stay out of the JSON payload; use the DSNs, URLs,
endpoints, or the human-readable `stackctl services` output when you need the
configured credentials directly.

Supported copy targets:

- `postgres`
- `redis`
- `nats`
- `seaweedfs`
- `pgadmin`
- `cockpit`
- `postgres-user`
- `postgres-password`
- `postgres-database`
- `redis-password`
- `nats-token`
- `seaweedfs-access-key`
- `seaweedfs-secret-key`
- `pgadmin-email`
- `pgadmin-password`

### `stackctl exec`

Run a command inside one of the stack services without looking up container
names manually.

Examples:

```bash
stackctl exec postgres -- psql -U app -d app
stackctl exec redis -- redis-cli -a secret PING
stackctl exec seaweedfs -- weed shell
stackctl exec pgadmin -- printenv PGADMIN_DEFAULT_EMAIL
```

Supported service targets:

- `postgres` or `pg`
- `redis` or `rd`
- `nats`
- `seaweedfs` or `seaweed`
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

Print minimal connection strings, URLs, and object-storage endpoint
credentials. This is intentionally smaller than `stackctl services` and is
meant for copy/paste.

Examples:

```bash
stackctl connect
```

Flags: `-h`, `--help` only.

### `stackctl env`

Print app-ready environment variables from the current stack config. By default
this prints shell-safe `KEY=value` assignments. Use `--export` when you want
`export KEY=value` lines for `eval` or `source` workflows, or `--json` for
tooling.

Examples:

```bash
stackctl env
stackctl env --export
stackctl env postgres redis
stackctl env --json
```

Useful patterns:

```bash
eval "$(stackctl env --export)"
stackctl env postgres > .env.stackctl-postgres
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--export` | Prefix each assignment with `export` |
| `-j`, `--json` | Print environment variables as a JSON object |

### `stackctl logs`

Show recent logs or follow them live.

Examples:

```bash
stackctl logs
stackctl logs --watch
stackctl logs --service postgres
stackctl logs --service seaweedfs --tail 200 --watch
```

Supported service filters:

- `postgres` or `pg`
- `redis` or `rd`
- `nats`
- `seaweedfs` or `seaweed`
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

### `stackctl factory-reset`

DANGEROUS: stop managed stacks discovered under stackctl's local data dir,
remove their volumes, and then delete all stackctl-owned config and local data
directories so you can start again from a true clean slate.

This only removes stackctl-owned paths such as `~/.config/stackctl` and
`~/.local/share/stackctl`. It does not follow an external stack path outside
stackctl's local data root. Named stack configs under
`~/.config/stackctl/stacks/` are removed too.

This is the fastest way to re-run the first-run experience from scratch across
all stacks, but it is intentionally gated behind a DANGEROUS confirmation
prompt unless you pass `--force`.

Examples:

```bash
stackctl factory-reset
stackctl factory-reset --force
```

Flags:

| Flag | Meaning |
| --- | --- |
| `-f`, `--force` | Skip the DANGEROUS confirmation prompt |

### `stackctl version`

Print version information.

Examples:

```bash
stackctl version
```

Flags: `-h`, `--help` only.

### `stackctl completion`

Generate shell completion scripts for bash, zsh, fish, or PowerShell.

Examples:

```bash
stackctl completion bash
stackctl completion zsh
```

Flags: `-h`, `--help` only.

## What the main commands are for

If you only remember a few commands, these are the ones most people will use:

- `stackctl setup`: create config and prepare the machine
- `stackctl start`: bring the stack or selected services up
- `stackctl services`: see the full runtime picture
- `stackctl connect`: copy DSNs, URLs, and endpoint credentials quickly
- `stackctl env`: print shell-ready env vars or JSON for app wiring
- `stackctl stack list`: see every configured stack and which one is active
- `stackctl stack use staging`: switch your saved default stack cleanly
- `stackctl services --copy postgres`: send a ready-to-use value straight to the clipboard
- `stackctl services --copy nats`: send the configured NATS DSN straight to the clipboard
- `stackctl services --copy seaweedfs`: send the configured S3 endpoint straight to the clipboard
- `stackctl db shell`: jump straight into `psql`
- `stackctl db dump`: export the local database as SQL
- `stackctl db restore`: replay a SQL dump into the local database
- `stackctl db reset`: recreate the configured database cleanly
- `stackctl ports`: check host-to-service port mappings quickly
- `stackctl factory-reset`: wipe stackctl's local state and start over cleanly
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

### Starting another stack fails

Only one local stack can be running at a time.

If `stackctl start` or `stackctl restart` tells you another local stack is
already running, stop that stack first:

```bash
stackctl --stack staging stop
```

Then start the stack you actually want to work on.

### I want to start over completely

Use the DANGEROUS clean-slate command:

```bash
stackctl factory-reset --force
```

This removes stackctl-owned config and managed stack data for the default stack
and every named stack.

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
git tag v0.17.0
git push origin v0.17.0
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
- NATS
- SeaweedFS (optional, disabled by default)
- pgAdmin
- Cockpit
- configurable Postgres database, username, password, and ports
- optional Redis auth that flows through generated compose and DSNs
- configurable NATS auth token and port that flow through managed scaffolding,
  DSNs, and helper commands
- optional SeaweedFS S3 endpoint with access key, secret key, volume sizing,
  managed scaffolding, helper output, and TUI coverage
- configurable pgAdmin login details that stay in sync with the managed stack
- explicit enable/disable toggles for Postgres, Redis, NATS, SeaweedFS,
  pgAdmin, and Cockpit helpers in setup, `config edit`, and the TUI config
  editor
- service-level image, data-volume, and tuning settings in `stackctl config`
- configurable Postgres maintenance-database settings for admin helpers
- configurable Redis persistence and maxmemory policy settings
- configurable pgAdmin server-mode settings
- named stacks via `--stack` or `STACKCTL_STACK`, with stack-specific managed
  paths, container names, and volume names
- persisted stack selection plus `stack list/current/use/delete/rename/clone`
  for profile-level stack management
- per-service `start`, `stop`, and `restart` in both the CLI and the TUI for
  stack-managed services
- a one-local-stack-at-a-time safety guard when starting or restarting stacks
- external-stack configs stay valid for non-runtime commands even before a
  compose file is present
- machine-readable service output with `stackctl services --json`
- clipboard-friendly service helpers with `stackctl services --copy <target>`
- `stackctl env` for app-ready environment variables with shell-export and JSON
  output modes
- `stackctl exec <service> -- <command...>` for in-container workflows
- `stackctl db shell` for one-step Postgres access
- `stackctl db dump`, `stackctl db restore`, and `stackctl db reset`
  for repeatable local database workflows
- `stackctl ports` for quick port inspection
- DANGEROUS `stackctl factory-reset` for a full local clean slate
- `stackctl doctor --fix` for supported automatic environment remediation
- live Podman integration coverage for the managed-stack lifecycle

Current CLI surface:

- `setup`
- `doctor`
- `tui`
- `start`, `stop`, `restart`
- `status`
- `services`
- `env`
- `ports`
- `db`
- `exec`
- `logs`
- `health`
- `connect`
- `open`
- `reset`
- `factory-reset`
- `stack`
- `config`
- `version`

### High priority

These are the highest-value additions after the current release line.

#### More local services

- `Meilisearch`
  Why: fast, lightweight search and autocomplete without Elasticsearch

Resulting target stack:

- PostgreSQL
- Redis
- NATS
- SeaweedFS
- Meilisearch
- pgAdmin
- Cockpit

### Next after that

These are strong follow-ups once the high-priority local stack and helper
commands are in place.

- deeper service controls such as Redis ACL users, richer Postgres tuning,
  and pgAdmin server bootstrap helpers
- `stackctl run ...` to launch an app with stack-aware context
- snapshot save and restore commands for dev-state workflows
- broader installer support beyond `apt`-based systems
- more explicit verbosity and quiet controls across runtime commands

### Longer-term UX work

These are appealing, but not ahead of the higher-value local-platform work.

- deeper TUI polish such as resize refinements, richer help surfaces, and
  accessibility work
- a self-update flow such as `stackctl update`
- a plugin model for optional service packs
- non-Linux support once runtime and install behavior are abstracted well

### Not planned right now

These are intentionally out of scope for the near term:

- Kafka
- Elasticsearch
- Kubernetes integration
