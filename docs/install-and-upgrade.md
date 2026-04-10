# Install, Upgrade, and Roll Back

This guide covers the supported install paths for `stackctl` plus the release
qualification expectations around upgrades and rollbacks.

## Choose the install path

- Use the default-branch bootstrap script when you want the convenience path to
  the latest GitHub release.
- Pin both the raw script URL and the `--version` flag when you need a
  deterministic install, upgrade, or rollback.
- Always verify `checksums.txt` before extracting a release archive manually.
- Newer tags cut from the current tagged-release workflow may also include
  `checksums.txt.sigstore.json`, per-archive SPDX SBOMs, and GitHub artifact
  attestations.
- Older `0.x` tags may predate those extra release assets and may only publish
  Linux archives.

## Verify a release archive before extraction

Manual installs should verify the downloaded archive before extraction. The
bootstrap installer in [`../scripts/install.sh`](../scripts/install.sh) already
does the checksum step automatically.

Linux example:

```bash
STACKCTL_VERSION=vX.Y.Z
mkdir -p /tmp/stackctl-verify
cd /tmp/stackctl-verify

gh release download "${STACKCTL_VERSION}" --repo traweezy/stackctl \
  -p 'checksums.txt' \
  -p 'stackctl_Linux_x86_64.tar.gz'

sha256sum -c --ignore-missing checksums.txt
```

For newer tags that also publish GitHub artifact attestations, you can verify
the archive against the release metadata directly:

```bash
gh release verify-asset "${STACKCTL_VERSION}" ./stackctl_Linux_x86_64.tar.gz \
  --repo traweezy/stackctl
```

For newer tags that also publish `checksums.txt.sigstore.json`, you can verify
the checksum manifest itself with `cosign`:

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/traweezy/stackctl/.github/workflows/release.yml@refs/tags/${STACKCTL_VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

If a historical `0.x` tag does not ship attestations or a Sigstore bundle,
`gh release verify-asset` will report that no attestations were found and the
bundle file will be absent. In that case, treat `checksums.txt` verification as
the supported baseline.

Maintainers working from a repo checkout can use
[`../scripts/verify-release-asset.sh`](../scripts/verify-release-asset.sh) to
bundle these checks into one step.

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
