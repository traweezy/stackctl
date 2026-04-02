# Upgrade and rollback

The safest `stackctl` upgrade flow is deterministic and reversible.

## Before upgrade

Back up the whole config directory:

```bash
stackctl_config_root="${XDG_CONFIG_HOME:-$HOME/.config}/stackctl"
stackctl_backup_root="${stackctl_config_root}-backup-$(date +%Y%m%d%H%M%S)"
cp -R "${stackctl_config_root}" "${stackctl_backup_root}"
```

Then verify the current binary:

```bash
stackctl version --json
```

## Upgrade to a specific release

Pin the script URL and the version to the same tag:

```bash
STACKCTL_VERSION=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/traweezy/stackctl/${STACKCTL_VERSION}/scripts/install.sh" | \
  bash -s -- --version "${STACKCTL_VERSION}"
stackctl version --json
```

## Roll back

Use the same pinned flow with the previous version:

```bash
STACKCTL_VERSION=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/traweezy/stackctl/${STACKCTL_VERSION}/scripts/install.sh" | \
  bash -s -- --version "${STACKCTL_VERSION}"
stackctl version --json
```

For the intended `1.x` contract, rollback assumes both versions understand the
same config schema.

If you intentionally go back to an older pre-schema `0.x` build, restore the
config backup you made before upgrade instead of assuming backward config
compatibility.

See the versioned guide in [../install-and-upgrade.md](../install-and-upgrade.md).
