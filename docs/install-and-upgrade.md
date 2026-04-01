# Install, Upgrade, and Roll Back

This guide covers the supported install paths for `stackctl` plus the release
qualification expectations around upgrades and rollbacks.

## Choose the install path

- Use the default-branch bootstrap script when you want the convenience path to
  the latest GitHub release.
- Pin both the raw script URL and the `--version` flag when you need a
  deterministic install, upgrade, or rollback.
- Tagged release archives are always verified against `checksums.txt` before
  extraction. See the README for optional Sigstore and attestation verification.

## Latest install

Install the latest release to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | bash
```

Install to `/usr/local/bin` instead:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | \
  bash -s -- --system
```

This is the convenience path. The bootstrap script comes from the default
branch, then downloads the latest tagged release archive for your platform.

## Deterministic install of a specific release

Pin the same tag in both places:

```bash
STACKCTL_VERSION=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/traweezy/stackctl/${STACKCTL_VERSION}/scripts/install.sh" | \
  bash -s -- --version "${STACKCTL_VERSION}"
```

That keeps the bootstrap script and the installed archive on the same release
boundary.

## Verify the installed binary

```bash
stackctl version --json
```

The machine-readable output is part of the documented compatibility contract in
[output-contract.md](./output-contract.md).

## Upgrade

Upgrade to the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | bash
stackctl version --json
```

Upgrade to a specific release:

```bash
STACKCTL_VERSION=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/traweezy/stackctl/${STACKCTL_VERSION}/scripts/install.sh" | \
  bash -s -- --version "${STACKCTL_VERSION}"
stackctl version --json
```

Before upgrade-sensitive transitions, back up the whole config directory:

```bash
stackctl_config_root="${XDG_CONFIG_HOME:-$HOME/.config}/stackctl"
stackctl_backup_root="${stackctl_config_root}-backup-$(date +%Y%m%d%H%M%S)"
cp -R "${stackctl_config_root}" "${stackctl_backup_root}"
```

If you use named stacks, back up the whole `stackctl` config directory instead
of a single file.

## Roll back

Roll back to a previous tagged release the same way:

```bash
STACKCTL_VERSION=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/traweezy/stackctl/${STACKCTL_VERSION}/scripts/install.sh" | \
  bash -s -- --version "${STACKCTL_VERSION}"
stackctl version --json
```

For the `1.x` contract, rollback is expected between releases that understand
`schema_version: 1`.

If you intentionally roll back to an older pre-schema `0.x` build, restore the
config backup you made before the upgrade instead of assuming backward config
compatibility.
