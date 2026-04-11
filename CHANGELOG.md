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
- committed startup, command, and TUI benchmarks plus documented local
  `hyperfine`, `pprof`, and idle-render evaluation flows
- a versioned TUI screenshot under `docs/media/` plus contributor guidance for
  refreshing it from a real rendered terminal window
- Homebrew cask scaffolding in GoReleaser, kept reviewable with upload
  intentionally disabled until the tap and signing decisions are made
- documented manual checksum, attestation, and Sigstore verification steps for
  tagged releases
- committed Go fuzz targets for the markdown render path so the
  `glamour`/`bluemonday` dependency chain is exercised directly in-repo

### Changed

- the coverage baseline is now enforced at `100.0%` in local checks, CI, and
  the tagged release workflow
- TUI rendering now reuses palette-filter and quiet or busy redraw work
  aggressively, cutting steady-state render cost and allocations
- the README, docs index, and wiki seed now align on the versioned docs,
  media, role-based entry points, and release guidance expected before `1.0.0`
- the README and wiki landing pages now point directly to versioned docs and
  make the one-stack, Podman-first posture more explicit
- the main human-authored docs now lead with user tasks and operational
  decisions instead of maintainer process language, while keeping local
  maintainer scratch workflows out of the tracked repo surface
- release verification docs now distinguish newer hardened tags from older
  `0.x` releases that predate attestations, Sigstore bundles, or wider
  archive coverage
- the bootstrap installer now checks `checksums.txt` before archive download so
  historical tags fail with a clear missing-platform message instead of a raw
  404
- the markdown-output dependency floor now tracks current `golang.org/x/net`
  releases instead of the older vulnerable `v0.44.0` line
