# Demo Capture

`stackctl` can use [Charm VHS](https://github.com/charmbracelet/vhs) for
reproducible CLI and TUI demos.

This is an optional docs and release tool, not part of the required CI path.

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

## Sample tapes

The repo keeps starter tapes under [../examples/vhs](../examples/vhs).

Run them from the repo root:

```bash
go build -trimpath -o dist/stackctl .
mkdir -p tmp/vhs
vhs examples/vhs/help.tape
vhs examples/vhs/version-json.tape
```

Both example tapes render into `tmp/vhs/`, which stays out of git.

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
release assets directly from source control.

The starter tapes intentionally drive `./dist/stackctl` so they work from a
repo-local build instead of depending on a globally installed binary.
