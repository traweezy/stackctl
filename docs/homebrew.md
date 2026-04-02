# Homebrew Distribution Plan

`stackctl`'s current official install path is GitHub Releases plus the
bootstrap installer in [`scripts/install.sh`](../scripts/install.sh).

This document explains the recommended Homebrew path and what still needs to
exist before releases can publish to a tap automatically.

## Current repo state

Today:

- tagged releases publish through GitHub Releases only
- macOS runtime setup is documented through Homebrew plus `podman machine`
- no release job attempts to push to a Homebrew tap yet

That is intentional until a tap repository and cross-repository token are in
place.

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

## Recommended first implementation step

The safest first repo change is:

- add a `homebrew_casks` block to `.goreleaser.yaml`
- set `skip_upload: true`
- generate the cask into `dist/` during snapshot dry-runs
- review the cask locally before enabling cross-repository publishing

That lets the repo qualify the cask content before any release workflow tries
to write to a tap repository.

## What is still required before publish is enabled

Before Homebrew publishing can be turned on for real, create and wire:

1. A tap repository such as `traweezy/homebrew-stackctl`.
2. A token with content-write access to that tap repository.
3. Release secrets in GitHub Actions for that token.
4. The `repository` block in `.goreleaser.yaml`.
5. macOS validation against the real tap output.

Until those exist, leave `skip_upload: true` in place.

## GitHub Actions token note

Publishing a cask from this repository to a different tap repository cannot
use the default GitHub Actions token.

Use a separate token scoped for the tap repository when the publish step is
enabled.

## Recommended publish cutover

When the tap exists, the cutover should be:

1. Add the tap repository owner/name/token under `homebrew_casks.repository`.
2. Add `homebrew_casks` with `skip_upload: true` and inspect the generated
   cask in `dist/`.
3. Change `skip_upload` from `true` to `false`.
4. Validate on macOS:
   - `brew install --cask ./dist/stackctl.rb`
   - `stackctl version --json`
   - `brew uninstall stackctl`
   - `brew audit --new --cask ./dist/stackctl.rb`
   - `brew style --fix ./dist/stackctl.rb`
5. Only after the tap workflow is solid, decide whether to pursue
   `homebrew/cask` or `homebrew/core`.

## Official docs used for this plan

- <https://docs.brew.sh/Adding-Software-to-Homebrew>
- <https://docs.brew.sh/Cask-Cookbook>
- <https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap>
- <https://goreleaser.com/customization/publish/homebrew_casks/>
