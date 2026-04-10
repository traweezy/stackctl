# VHS Examples

These tapes are starter examples for rendering `stackctl` demos with
[Charm VHS](https://github.com/charmbracelet/vhs).

Run them from the repo root so the `Output` paths land in ignored `tmp/`
space:

```bash
bash scripts/render-vhs-demo.sh --tape examples/vhs/help.tape
bash scripts/render-vhs-demo.sh --tape examples/vhs/version-json.tape
```

The helper pins the VHS image, prefers `podman` before `docker`, and builds
`./dist/stackctl` automatically unless you pass `--binary` or `--no-build`.

The current examples intentionally use deterministic commands that do not
require a live Podman stack. They also target `./dist/stackctl` so the render
path stays repo-local.
