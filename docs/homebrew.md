# Homebrew Distribution Plan

`stackctl`'s current official install path is GitHub Releases plus the
bootstrap installer in [`scripts/install.sh`](../scripts/install.sh).

This document explains the recommended Homebrew path and what still needs to
exist before releases can publish to a tap automatically.

## Current repo state

Today:

- tagged releases publish through GitHub Releases only
- macOS runtime setup is documented through Homebrew plus `podman machine`
- GoReleaser now generates a tap-ready `stackctl` cask into `dist/` during
  snapshot and tagged release runs
- no release job attempts to push that cask to a Homebrew tap yet

That is intentional until a tap repository, publish token, and macOS
distribution posture are finalized.

## Why the planned path is tap plus cask

For `stackctl`, the pragmatic first Homebrew path is:

- an upstream tap such as `traweezy/homebrew-stackctl`
- a binary-installing cask named `stackctl`

Reasons:

- it fits the current GitHub release archive model
- GoReleaser supports Homebrew cask generation directly
- it avoids prematurely optimizing for `homebrew/core`
- it keeps the official release artifacts as the source of truth

The longer-term `homebrew/core` path would be a separate effort with a
build-from-source formula and Homebrew-core-specific review constraints.

## What the planned cask should include

When the cask is wired, it should install:

- the `stackctl` binary from the Darwin release archive
- the root `stackctl` man page
- generated shell completions by running `stackctl completion <shell>` at
  install time

The repo keeps the full generated docs, man pages, and completions in the
release archive regardless of whether the cask is published yet.

## What the repo now does

The repo now takes the first safe step from the original plan:

- `.goreleaser.yaml` includes a `homebrew_casks` block
- `skip_upload: true` keeps publish disabled
- `goreleaser release --snapshot --clean` generates a reviewable cask in
  `dist/`
- the generated cask installs the `stackctl` binary, the root man page, and
  shell completions generated from `stackctl completion`

This lets the release pipeline qualify the cask content before any workflow is
allowed to write to a tap repository.

## What is still required before publish is enabled

Before Homebrew publishing can be turned on for real, create and wire:

1. A tap repository such as `traweezy/homebrew-stackctl`.
2. A token with content-write access to that tap repository.
3. Release secrets in GitHub Actions for that token.
4. The `repository` block in `.goreleaser.yaml`.
5. A final decision on macOS signing and notarization posture for the cask
   payload.
6. macOS validation against the real tap output.

Until those exist, leave `skip_upload: true` in place.

## GitHub Actions token note

Publishing a cask from this repository to a different tap repository cannot
use the default GitHub Actions token.

Use a separate token scoped for the tap repository when the publish step is
enabled.

## Recommended publish cutover

When the tap exists, the cutover should be:

1. Add the tap repository owner/name/token under `homebrew_casks.repository`.
2. Inspect the generated cask in `dist/` and confirm the archive paths, man
   page install, shell completions, and caveats still match the release
   archive content.
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

## macOS signing note

GoReleaser's current Homebrew cask guidance explicitly warns that unsigned,
non-notarized binaries may trip Gatekeeper after install. `stackctl` already
uses Sigstore for release verification, but that is not the same thing as
Apple code signing and notarization.

That means the Homebrew publish cutover should stay disabled until one of these
is chosen intentionally:

- add Apple signing and notarization for the macOS release artifacts
- or accept the operator friction of a manual quarantine-clearing workaround

The first option is the cleaner `1.0.x` path. The second should not become the
default without an explicit product decision because it weakens the normal
macOS trust flow.

## Official docs used for this plan

- <https://docs.brew.sh/Adding-Software-to-Homebrew>
- <https://docs.brew.sh/Cask-Cookbook>
- <https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap>
- <https://goreleaser.com/customization/publish/homebrew_casks/>
