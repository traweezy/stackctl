# VHS Examples

These tapes are maintainer examples for local terminal demos. They are not part
of the user-facing product story, and they do not need a live Podman stack.

Run them from the repo root:

```bash
bash scripts/render-vhs-demo.sh --tape examples/vhs/help.tape
bash scripts/render-vhs-demo.sh --tape examples/vhs/version-json.tape
```

The helper pins the VHS image, prefers `podman` before `docker`, and builds
`./dist/stackctl` automatically unless you pass `--binary` or `--no-build`.

The current examples are deliberately simple:

- `help.tape` checks the local `--help` render
- `version-json.tape` checks the JSON version output

They render into ignored `tmp/` paths so you can preview or iterate without
changing tracked assets.
