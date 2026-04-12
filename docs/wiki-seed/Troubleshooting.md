# Troubleshooting

Start with the smallest command that answers the next question.

## First checks

Use these in order:

```bash
stackctl status
stackctl services
stackctl health
stackctl logs --watch
stackctl doctor
stackctl doctor --check-images
```

That sequence answers:

- are the containers present
- what is configured and exposed
- are the endpoints reachable
- what are the services saying
- is the surrounding machine healthy
- are the configured image references still reachable

## No config exists yet

Create one with either:

```bash
stackctl setup
stackctl config init
```

## A managed stack will not start

Common causes:

- `podman` or `podman compose` is missing or below the supported floor
- a required host port is already taken
- a different named stack is already running
- the machine-specific Podman runtime is not initialized

Useful commands:

```bash
stackctl doctor
stackctl health
stackctl stack current
stackctl stack list
```

## A service looks up but is unhealthy

Narrow it to one service:

```bash
stackctl services
stackctl health
stackctl logs --service postgres --watch
stackctl logs --service redis --watch
```

Use the service-specific logs path so the first failing container is obvious.

## I want to start over cleanly

To reset the current stack:

```bash
stackctl reset --volumes --force
```

To wipe all `stackctl`-owned local state:

```bash
stackctl factory-reset --force
```

That command is intentionally destructive. Use it only when you truly want a
clean slate.

## External stack mode problems

If you chose an external stack:

- confirm the compose path in `stackctl config path`
- make sure the configured compose directory and file still exist
- remember that non-runtime helper commands may still work before the compose
  file is present, while lifecycle commands will not

## Clipboard copy does not work

On Linux, install a clipboard helper:

- `wl-copy` for Wayland
- `xclip` or `xsel` for X11

If that is unavailable, use the plain command output instead of `--copy`.
