# Docs Media and Local Demos

Most `stackctl` users do not need to care about VHS, demo tapes, or screenshot
capture. This page is for maintainers who are refreshing README or wiki media,
or who want a deterministic way to record a terminal walkthrough.

## What lives in git

The only checked-in docs image today is:

- `docs/media/tui-services.png`

That image earns its place because it shows the TUI layout at a glance. We do
not currently check in an animated GIF. The old `--help` animation added motion
without teaching anything the README text did not already cover.

## When to use VHS

Use [Charm VHS](https://github.com/charmbracelet/vhs) when you need terminal
media that is:

- deterministic
- reviewable in plain text
- easy to rerender after UI or copy changes

That is useful for maintainers because tapes diff cleanly and can be rerun on a
branch. It is not a product feature, and it should not drive the user-facing
story in the README.

## Choosing the right asset

Prefer:

- a screenshot when one still image explains the interface quickly
- a code block when plain command output is the point
- a local VHS demo when motion or timing actually matters

Only check in a GIF or video if it teaches something a screenshot and text
cannot. A moving `--help` screen usually fails that test.

## Refresh the checked-in screenshot

```bash
bash scripts/capture-docs-media.sh
```

This runs the deterministic docs-only TUI harness, opens it in `xterm`, and
captures a real rendered window into `docs/media/tui-services.png`.

## Render local VHS examples

Run the repo helper:

```bash
bash scripts/render-vhs-demo.sh --tape examples/vhs/help.tape
bash scripts/render-vhs-demo.sh --tape examples/vhs/version-json.tape
```

The helper:

- pins `ghcr.io/charmbracelet/vhs:v0.11.0`
- prefers `podman`, then falls back to `docker`
- builds `./dist/stackctl` unless you pass `--binary` or `--no-build`
- lets you override the tape `Output` path with `--output`

The example tapes render into `tmp/vhs/`, which stays out of git.

## Starter tapes

Starter tapes live in [../examples/vhs](../examples/vhs). They are small,
deterministic examples meant for local preview and iteration:

- `help.tape`
- `version-json.tape`

If you add another tape, keep it deterministic, keep it short, and prefer a
flow that shows actual user value instead of generic help text.
