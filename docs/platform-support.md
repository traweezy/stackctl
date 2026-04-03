# Platform Support Matrix

This document explains what `stackctl` currently expects from each supported
host path.

Use it together with [compatibility.md](./compatibility.md):

- `compatibility.md` defines the `1.x` contract and verification policy
- this document explains the package-manager and host-service differences

## Runtime baseline

The supported managed-runtime floor is:

- `podman` `4.9.3+`
- a `podman compose` provider `1.0.6+`

Those minimums apply across Linux and macOS. Older distro packages may still
install, but installability alone does not expand the supported runtime floor.

## Verification tiers

Hosted CI continuously verifies:

- build, lint, vet, race, coverage, and security checks
- Linux Podman integration
- Linux package-manager smoke in disposable distro containers
- installer and journey smoke

Release-qualified `platform-lab` verifies:

- full-host Linux distro journeys
- macOS Homebrew plus `podman machine`

See [compatibility.md](./compatibility.md) for the formal support policy.

## Host matrix

### Debian and Ubuntu family: `apt`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`

Cockpit behavior:

- Cockpit helpers are supported when the host uses `systemd`
- `stackctl` does not auto-install Cockpit on the `apt` path
- if you want Cockpit here, install it manually and let `stackctl` keep the
  helper URLs, open actions, and diagnostics aligned

### Fedora and RHEL family: `dnf`, `yum`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`
- `cockpit`
- `cockpit-podman`

Cockpit behavior:

- Cockpit helpers are supported when the host uses `systemd`
- `stackctl setup --install` and `stackctl doctor --fix` can install and enable
  Cockpit automatically on this path

### Arch family: `pacman`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`
- `cockpit`
- `cockpit-podman`

Cockpit behavior:

- Cockpit helpers are supported when the host uses `systemd`
- automatic Cockpit install and enablement are supported on this path

### openSUSE family: `zypper`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`
- `cockpit`
- `cockpit-podman`

Cockpit behavior:

- Cockpit helpers are supported when the host uses `systemd`
- automatic Cockpit install and enablement are supported on this path
- the smoke harness retries `zypper refresh` and install flows because mirror
  metadata failures are common enough to be worth handling explicitly

### Alpine: `apk`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`

Cockpit behavior:

- Cockpit is not supported through `stackctl` on Alpine
- `stackctl` should not expect `systemctl` or other `systemd`-specific behavior
- Cockpit helper and install settings should stay off on this host path

### macOS: `brew`

Automatic package install support:

- `podman`
- `podman-compose`
- `skopeo`

Not automatically installed by `stackctl`:

- `buildah`
- Cockpit

Extra readiness requirement:

- `podman machine` must be initialized and running before managed runtime
  commands can succeed

Operational meaning:

- Homebrew is the runtime bootstrap path for macOS
- GitHub Releases remain the official `stackctl` binary distribution path until
  a Homebrew tap is published intentionally

See [homebrew.md](./homebrew.md) for the distribution plan.

## Important config nuance

`system.package_manager` chooses the package backend for setup and doctor fix
flows, but it does not redefine the actual host.

That means:

- changing `system.package_manager` to `brew` on Linux does not make Linux use
  `podman machine`
- changing `system.package_manager` to `apt` on macOS does not make macOS gain
  `systemd` or Cockpit support
- host OS and service-manager behavior still come from the real machine

The CLI, wizard, and TUI try to describe this as accurately as possible from
the current host plus the current config draft.
