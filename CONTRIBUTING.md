# Contributing

Thanks for working on `stackctl`.

This repo aims to keep changes small, reviewable, and release-safe. The CLI,
wizard, TUI, docs, and release pipeline are all part of the product contract,
so even small changes should keep those surfaces aligned.

## Before you start

- read the top-level [README.md](./README.md)
- use the versioned docs in [docs/README.md](./docs/README.md) for the stable
  contract and operator-facing guides
- check [SECURITY.md](./SECURITY.md) before reporting security-sensitive issues

## Development setup

The project uses the Go version declared in `go.mod`.

Typical local setup:

```bash
git clone https://github.com/traweezy/stackctl.git
cd stackctl
go test ./... -count=1
go run . --help
```

If you are working on real runtime behavior, you will also want a supported
Podman environment available locally.

## Expected local verification

For most changes, run:

```bash
bash scripts/check-workflows.sh
bash scripts/check-shell-scripts.sh
go test ./... -count=1
go test ./... -race -count=1
go vet ./...
bash scripts/check-coverage.sh
```

When the change touches installer, release, or runtime flows, also run the
relevant deeper checks:

```bash
bash scripts/install-smoke.sh
bash scripts/journey-smoke.sh
go test ./integration -tags=integration -count=1
```

If you change release packaging, also qualify the snapshot release path:

```bash
goreleaser release --snapshot --clean
```

If you change README or docs links, also run:

```bash
bash scripts/check-links.sh
```

If GitHub link checks get rate-limited locally, export `GITHUB_TOKEN` first or
make sure `gh auth` is available so the script can reuse that token.

Pull requests also run GitHub-hosted dependency review, and the default branch
has a separate Scorecard workflow for ongoing supply-chain posture tracking.

## Generated assets

The generated CLI docs, man pages, and shell completions are part of the repo.

If your change affects command help, flags, or command shape, regenerate them:

```bash
bash scripts/generate-cli-assets.sh
```

CI verifies that these generated assets are up to date.

## Docs expectations

Update docs when behavior changes.

Common cases:

- README for first-visit or install-flow changes
- `docs/compatibility.md` for `1.x` contract changes
- `docs/output-contract.md` for machine-readable output changes
- `docs/install-and-upgrade.md` for install, upgrade, rollback, or release
  verification changes
- `docs/homebrew.md` for Homebrew distribution planning changes
- `docs/wiki-seed/` for narrative operator guidance that should not become part
  of the stable contract

## Commit style

Keep commits small and use Conventional Commit style with the repo's existing
emoji prefix convention.

Recent examples:

- `🐛 fix(ci): warm managed images before Podman smoke`
- `📝 docs(readme): tighten landing page and docs index`
- `📝 docs(homebrew): document tap and cask rollout`

Prefer one logical change per commit.

## Pull requests

When opening a PR:

- explain the operator impact
- call out any contract or compatibility implications
- mention which local verification commands you ran
- include screenshots when the change affects rendered TUI layout

For visual TUI work, validate with a real screenshot rather than only text
render output or snapshot tests.

## Security

Do not open public issues for suspected vulnerabilities. Follow
[SECURITY.md](./SECURITY.md) instead.
