# Demo Capture and Media

`stackctl` can use [Charm VHS](https://github.com/charmbracelet/vhs) for
reproducible CLI and TUI demos.

This is an optional docs and release tool, not part of the required CI path.

The repo now also carries a versioned still asset for the README and wiki seed:

- `docs/media/tui-services.png`

## Why VHS

VHS makes terminal demos reviewable as code:

- tapes are plain text and easy to diff
- rendered media is reproducible from the same commands
- operator flows can be documented without hand-recorded screencasts

## Prerequisites

Use one of the supported installation paths from the upstream project:

- `brew install vhs`
- `go install github.com/charmbracelet/vhs@latest`
- `docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs <tape>.tape`

If you run `vhs` directly on the host, make sure `ttyd` and `ffmpeg` are also
installed and available on `PATH`.

## Repo helper

The supported repo-local path is the helper script:

```bash
bash scripts/render-vhs-demo.sh --tape examples/vhs/help.tape
bash scripts/render-vhs-demo.sh --tape examples/vhs/version-json.tape
```

The helper:

- pins the container image to `ghcr.io/charmbracelet/vhs:v0.11.0`
- prefers `podman`, then falls back to `docker`
- builds `./dist/stackctl` automatically unless you pass `--binary` or
  `--no-build`
- can rewrite the tape `Output` path with `--output`

The first run may pull the pinned VHS image if it is not already present
locally.

## Sample tapes

The repo keeps starter tapes under [../examples/vhs](../examples/vhs).

Run them from the repo root:

```bash
bash scripts/render-vhs-demo.sh --tape examples/vhs/help.tape
bash scripts/render-vhs-demo.sh --tape examples/vhs/version-json.tape
```

Both example tapes render into `tmp/vhs/`, which stays out of git.

## Versioned screenshot capture

To refresh the checked-in TUI screenshot, run:

```bash
bash scripts/capture-docs-media.sh
```

This uses a deterministic docs-only TUI harness, launches it in `xterm`, and
captures a real rendered window into `docs/media/tui-services.png`.

If you intentionally want to refresh a versioned motion asset as well, render
it explicitly:

```bash
bash scripts/render-vhs-demo.sh --tape examples/vhs/help.tape --output docs/media/help.gif
```

## Adding a new demo

Start by recording or hand-authoring a tape:

```bash
vhs record > examples/vhs/my-flow.tape
```

Then edit it down so the flow stays deterministic.

Good demo candidates:

- `stackctl --help`
- `stackctl version --json`
- static docs or config inspection flows
- carefully scripted TUI paths that do not depend on host-local secrets

Avoid committing generated GIFs or videos unless the repo later decides to ship
release assets directly from source control. Still screenshots are acceptable
when they are intentional, reproducible, and clearly support the landing-page
docs.

The starter tapes intentionally drive `./dist/stackctl` so they work from a
repo-local build instead of depending on a globally installed binary.
