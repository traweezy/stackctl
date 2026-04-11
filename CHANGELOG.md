# Changelog

This file tracks notable project-level changes and release process updates.
Tagged releases continue to use GitHub-generated release notes as the
authoritative per-release summary.

## Unreleased

### Added

- pinned CI and release tool versions for reproducible verification
- a dedicated coverage gate and release-installer smoke path
- signed release checksums, SPDX SBOM generation, and GitHub attestations
- `SECURITY.md` and CODEOWNERS coverage for release-critical files
- committed startup, command, and TUI benchmarks plus local `hyperfine` and
  `pprof` evaluation helpers under `scripts/bench-cli.sh`,
  `scripts/evaluate-pgo.sh`, and `scripts/evaluate-tui-idle.sh`
- a versioned TUI screenshot under `docs/media/` plus a reproducible
  `xterm`-based capture path for README and wiki refreshes
- Homebrew cask scaffolding in GoReleaser, kept reviewable with upload
  intentionally disabled until the tap and signing decisions are made
- a repo-local `scripts/verify-release-asset.sh` helper for checksum,
  attestation, and Sigstore bundle verification of tagged releases
- a pinned `scripts/render-vhs-demo.sh` helper for reproducible CLI and TUI
  demo rendering from the repo's VHS tapes
- a versioned `docs/media/help.gif` CLI help demo rendered from the VHS helper

### Changed

- the coverage baseline is now enforced at `100.0%` in local checks, CI, and
  the tagged release workflow
- TUI rendering now reuses palette-filter and quiet or busy redraw work
  aggressively, cutting steady-state render cost and allocations
- the README, docs index, and wiki seed now align on the versioned docs,
  media, and release entry points expected before `1.0.0`
- the README and wiki landing pages now point directly to the demo-capture
  path and make the one-stack, Podman-first posture more explicit
- release verification docs now distinguish newer hardened tags from older
  `0.x` releases that predate attestations, Sigstore bundles, or wider
  archive coverage
- the bootstrap installer now checks `checksums.txt` before archive download so
  historical tags fail with a clear missing-platform message instead of a raw
  404
