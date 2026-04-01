# Output Contract

This document defines the machine-readable outputs that `stackctl` intends to
treat as stable in `1.x`.

For automation, prefer these outputs instead of parsing human-oriented tables,
status lines, or TUI text.

## General rules

- prefer `--json` outputs when available
- treat unknown JSON fields as ignorable
- do not parse colored or table-based output
- prefer `stackctl version --json` over `stackctl --version` for automation

## `stackctl version --json`

Shape:

```json
{
  "version": "0.20.1",
  "git_commit": "abc123",
  "build_date": "2026-03-21T00:00:00Z"
}
```

Rules:

- `version` is always present
- `git_commit` is omitted when unavailable
- `build_date` is omitted when unavailable

## `stackctl env --json`

Shape:

- JSON object
- keys are environment variable names
- values are strings

Example:

```json
{
  "DATABASE_URL": "postgres://app:app@localhost:5432/app",
  "REDIS_URL": "redis://localhost:6379"
}
```

Rules:

- this output is for application wiring and may include credentials
- consumers should treat values as sensitive
- absent services simply do not contribute keys

## `stackctl services --json`

Shape:

- JSON array of service objects

Common fields:

- `name`
- `display_name`
- `status`
- `container_name`
- `image`
- `host`
- `external_port`
- `internal_port`
- `port_listening`
- `port_conflict`
- `endpoint`
- `url`
- `dsn`

Service-specific fields may also appear, for example:

- `database`
- `maintenance_database`
- `email`
- `username`
- `access_key`
- `max_connections`
- `shared_buffers`
- `log_min_duration_statement_ms`
- `appendonly`
- `save_policy`
- `maxmemory_policy`
- `volume_size_limit_mb`
- `server_mode`
- `bootstrap_server`
- `bootstrap_server_group`

Secret omission rules:

- `password` is intentionally omitted
- `secret_key` is intentionally omitted
- `token` is intentionally omitted
- `master_key` is intentionally omitted

## `stackctl status --json`

Shape:

- JSON array of container objects

Fields currently emitted:

- `Id`
- `Image`
- `Names`
- `Status`
- `State`
- `Ports`
- `CreatedAt`

`Ports` is an array of objects with:

- `host_port`
- `container_port`
- `protocol`

Notes:

- field casing follows the current Podman-derived container summary shape
- this output is intended to remain available in `1.x`, even if human table
  output changes

## Human-oriented outputs

These outputs are not treated as strict automation contracts:

- default `stackctl version`
- table output from `status`, `services`, `ports`, and similar commands
- progress lines and spinners
- wizard prompts
- TUI rendering

If you need automation stability, use the documented JSON modes above.
