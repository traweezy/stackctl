# Platform Support Matrix

Use this matrix when you need to know what `stackctl` can install or manage on
each supported host family.

Pair it with [compatibility.md](./compatibility.md):

- `compatibility.md` defines the support policy and runtime floor
- this page explains package-manager and host-service differences

## Runtime baseline

The supported managed-runtime floor is:

- `podman` `4.9.3+`
- a `podman compose` provider `1.0.6+`

Those minimums apply across Linux and macOS. The fact that an older package
still installs does not expand the support floor.

## What gets tested where

Hosted CI continuously covers:

- build, lint, vet, race, coverage, and security checks
- Linux Podman integration
- Linux package-manager smoke in disposable distro containers
- installer and journey smoke

Release-time and scheduled `platform-lab` coverage extends that to:

- full-host Linux distro journeys
- macOS Homebrew plus `podman machine`

See [compatibility.md](./compatibility.md) for the policy details.

## Host matrix

### Debian and Ubuntu family: `apt`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`

Cockpit notes:

- supported when the host uses `systemd`
- not auto-installed by `stackctl`
- if you want Cockpit here, install it yourself and let `stackctl` keep the
  helper URLs, open actions, and diagnostics aligned

### Fedora and RHEL family: `dnf`, `yum`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`
- `cockpit`
- `cockpit-podman`

Cockpit notes:

- supported when the host uses `systemd`
- `stackctl setup --install` and `stackctl doctor --fix` can install and enable
  Cockpit automatically

### Arch family: `pacman`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`
- `cockpit`
- `cockpit-podman`

Cockpit notes:

- supported when the host uses `systemd`
- automatic Cockpit install and enablement are supported

### openSUSE family: `zypper`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`
- `cockpit`
- `cockpit-podman`

Cockpit notes:

- supported when the host uses `systemd`
- automatic Cockpit install and enablement are supported
- the smoke harness retries `zypper refresh` and install flows because mirror
  metadata failures are common enough to handle explicitly

### Alpine: `apk`

Automatic package install support:

- `podman`
- `podman-compose`
- `buildah`
- `skopeo`

Cockpit notes:

- Cockpit is not supported through `stackctl` on Alpine
- `stackctl` does not assume `systemd`
- Cockpit helper and install settings should stay off on this host

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
- GitHub Releases remain the official `stackctl` binary distribution channel
  until a Homebrew tap is published

See [homebrew.md](./homebrew.md) for the distribution plan.

## Important config detail

`system.package_manager` chooses the package backend for setup and doctor fix
flows, but it does not redefine the real host.

That means:

- setting `system.package_manager: brew` on Linux does not make Linux use
  `podman machine`
- setting `system.package_manager: apt` on macOS does not make macOS gain
  `systemd` or Cockpit support
- the actual host OS and service manager still come from the machine you are on
