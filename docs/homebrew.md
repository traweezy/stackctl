# Homebrew Distribution Plan

Homebrew already matters for `stackctl` on macOS because it installs the local
runtime pieces around Podman. It is not yet the official distribution channel
for the `stackctl` binary itself.

This page explains the intended Homebrew path and what still has to be true
before releases can publish to a tap.

## Current state

Today:

- tagged releases publish through GitHub Releases
- macOS runtime setup is documented through Homebrew plus `podman machine`
- GoReleaser generates a tap-ready cask into `dist/` during snapshot and tagged
  release runs
- no release job pushes that cask to a Homebrew tap yet

Publishing stays disabled until the tap, token, and macOS trust posture are
settled.

## Planned Homebrew shape

The pragmatic first Homebrew path is:

- an upstream tap such as `traweezy/homebrew-stackctl`
- a binary-installing cask named `stackctl`

Why this path:

- it fits the current GitHub Releases archive model
- GoReleaser supports Homebrew cask generation directly
- it avoids the extra review and packaging constraints of `homebrew/core`
- it keeps the release archives as the source of truth

The longer-term `homebrew/core` option would be a separate build-from-source
effort.

## What the cask should install

When publishing is enabled, the cask should install:

- the `stackctl` binary from the Darwin release archive
- the root `stackctl` man page
- generated shell completions by running `stackctl completion <shell>` at
  install time

## What the repo already does

The current safe groundwork is in place:

- `.goreleaser.yaml` includes a `homebrew_casks` block
- `skip_upload: true` keeps publish disabled
- `goreleaser release --snapshot --clean --skip=sign` generates a reviewable
  cask in
  `dist/`
- the generated cask installs the binary, root man page, and shell completions

That lets releases qualify the cask content before any workflow writes to a tap
repository.

## What must exist before publish

Before Homebrew publishing can be enabled, create and wire:

1. A tap repository such as `traweezy/homebrew-stackctl`.
2. A token with content-write access to that tap repository.
3. GitHub Actions secrets for that token.
4. The `repository` block in `.goreleaser.yaml`.
5. A deliberate macOS signing and notarization decision.
6. macOS validation against the real tap output.

Until those exist, leave `skip_upload: true` in place.

## Token note

Publishing from this repository to a different tap repository cannot use the
default GitHub Actions token. Use a separate token scoped for the tap repo.

## Recommended cutover checklist

When the tap exists:

1. Add the tap owner, repo, and token under `homebrew_casks.repository`.
2. Inspect the generated cask in `dist/` and confirm the archive paths, man
   page install, completions, and caveats still match the release archive.
3. Change `skip_upload` from `true` to `false`.
4. Validate on macOS:
   - `brew install --cask ./dist/stackctl.rb`
   - `stackctl version --json`
   - `stackctl setup`
   - `brew uninstall stackctl`
   - `brew audit --new --cask ./dist/stackctl.rb`
   - `brew style --fix ./dist/stackctl.rb`
5. Only after the tap workflow is solid, decide whether to pursue
   `homebrew/cask` or `homebrew/core`.

## macOS signing and notarization

Unsigned, non-notarized binaries can still trip Gatekeeper after Homebrew
install. Sigstore improves release verification, but it does not replace Apple
code signing and notarization.

Before turning on Homebrew publishing, pick one of these paths:

- add Apple signing and notarization for the macOS release artifacts
- accept the user friction of manual quarantine-clearing steps

The first option is the cleaner `1.0.x` path. The second should be an explicit
product decision, not an accident.

## Reference docs

- <https://docs.brew.sh/Adding-Software-to-Homebrew>
- <https://docs.brew.sh/Cask-Cookbook>
- <https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap>
- <https://goreleaser.com/customization/publish/homebrew_casks/>
