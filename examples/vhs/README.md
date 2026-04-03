# VHS Examples

These tapes are starter examples for rendering `stackctl` demos with
[Charm VHS](https://github.com/charmbracelet/vhs).

Run them from the repo root so the `Output` paths land in ignored `tmp/`
space:

```bash
go build -trimpath -o dist/stackctl .
mkdir -p tmp/vhs
vhs examples/vhs/help.tape
vhs examples/vhs/version-json.tape
```

The current examples intentionally use deterministic commands that do not
require a live Podman stack. They also target `./dist/stackctl` so the render
path stays repo-local.
