# Install, Upgrade, and Roll Back

Use this guide when you need to install `stackctl`, pin a specific release, or
roll back safely.

## Pick an install method

- Use the default-branch install script when you want the latest release with
  the least typing.
- Pin both the raw script URL and the `--version` flag when you need a
  deterministic install, upgrade, or rollback.
- If you download archives manually, verify `checksums.txt` before extraction.
- Newer tags may also ship `checksums.txt.sigstore.json`, per-archive SPDX
  SBOMs, `stackctl-release.intoto.jsonl`, and GitHub artifact attestations.
- Older `0.x` tags may predate those extra assets and may only publish Linux
  archives.

## Install the latest release

Install to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | bash
```

Install to `/usr/local/bin` instead:

```bash
curl -fsSL https://raw.githubusercontent.com/traweezy/stackctl/master/scripts/install.sh | \
  bash -s -- --system
```

This is the convenience path: the script comes from the default branch and then
downloads the latest tagged release archive for your platform.

## Install a specific release

Pin the same tag in both places:

```bash
STACKCTL_VERSION=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/traweezy/stackctl/${STACKCTL_VERSION}/scripts/install.sh" | \
  bash -s -- --version "${STACKCTL_VERSION}"
```

That keeps the bootstrap script and the installed archive on the same release.

## Verify a release archive manually

The install script already checks `checksums.txt`. Use the manual path when you
want to inspect the archive yourself before extraction.

Linux example:

```bash
STACKCTL_VERSION=vX.Y.Z
mkdir -p /tmp/stackctl-verify
cd /tmp/stackctl-verify

gh release download "${STACKCTL_VERSION}" --repo traweezy/stackctl \
  -p 'checksums.txt' \
  -p 'stackctl-release.intoto.jsonl' \
  -p 'stackctl_Linux_x86_64.tar.gz'

sha256sum -c --ignore-missing checksums.txt
```

If the release also publishes GitHub artifact attestations:

```bash
gh release verify-asset "${STACKCTL_VERSION}" ./stackctl_Linux_x86_64.tar.gz \
  --repo traweezy/stackctl
```

If the release also publishes `checksums.txt.sigstore.json`:

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/traweezy/stackctl/.github/workflows/release.yml@refs/tags/${STACKCTL_VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

If the release also publishes `stackctl-release.intoto.jsonl`:

```bash
go install github.com/slsa-framework/slsa-verifier/v2/cli/slsa-verifier@v2.7.1

slsa-verifier verify-artifact \
  --provenance-path stackctl-release.intoto.jsonl \
  --source-uri github.com/traweezy/stackctl \
  --source-tag "${STACKCTL_VERSION}" \
  stackctl_Linux_x86_64.tar.gz
```

The same provenance file can be reused for any archive or SBOM listed inside
that release-wide statement.

For historical `0.x` tags that do not ship attestations or a Sigstore bundle,
checksum verification is the supported baseline. Likewise, only verify
`stackctl-release.intoto.jsonl` when that file is present on the release.

Even from a repo checkout, prefer this same manual flow. The published release
artifacts are the contract, and local wrapper helpers are intentionally kept
out of the tracked repo surface.

## Verify the installed binary

```bash
stackctl version --json
```

That JSON output is part of the documented automation contract in
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

Before upgrade-sensitive transitions, back up the config directory:

```bash
stackctl_config_root="${XDG_CONFIG_HOME:-$HOME/.config}/stackctl"
stackctl_backup_root="${stackctl_config_root}-backup-$(date +%Y%m%d%H%M%S)"
cp -R "${stackctl_config_root}" "${stackctl_backup_root}"
```

If you use named stacks, back up the whole `stackctl` config directory instead
of a single file.

## Roll back

Rollback uses the same pinned-install flow:

```bash
STACKCTL_VERSION=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/traweezy/stackctl/${STACKCTL_VERSION}/scripts/install.sh" | \
  bash -s -- --version "${STACKCTL_VERSION}"
stackctl version --json
```

For the planned `1.x` contract, rollback is expected between releases that
understand `schema_version: 1`.

If you intentionally roll back to an older pre-schema `0.x` build, restore the
config backup you made before the upgrade instead of assuming backward config
compatibility.
