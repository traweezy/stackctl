# Documentation

This directory holds the versioned documentation that complements the short
top-level README.

Use the README for the product overview and quick start. Use the docs here when
you need the stable contract, operator procedures, or generated command
reference.

## Start here

- [../README.md](../README.md)
- [install-and-upgrade.md](./install-and-upgrade.md)
- [compatibility.md](./compatibility.md)
- [homebrew.md](./homebrew.md)
- [output-contract.md](./output-contract.md)

## Command reference

- [cli/stackctl.md](./cli/stackctl.md)
- [man/man1/stackctl.1](./man/man1/stackctl.1)

The `docs/cli/` tree contains generated Markdown docs for every command. The
`docs/man/man1/` tree contains the generated man pages shipped in release
archives.

## Shell completions

- [completions/stackctl.bash](./completions/stackctl.bash)
- [completions/stackctl.fish](./completions/stackctl.fish)
- [completions/stackctl.ps1](./completions/stackctl.ps1)
- [completions/_stackctl](./completions/_stackctl)

## Stable automation surfaces

The machine-readable `1.x` contract is intentionally narrow:

- `stackctl version --json`
- `stackctl env --json`
- `stackctl services --json`
- `stackctl status --json`

See [output-contract.md](./output-contract.md) for the documented field-level
expectations.

## Security and release artifacts

Release verification expectations live in:

- [../SECURITY.md](../SECURITY.md)
- [install-and-upgrade.md](./install-and-upgrade.md)

Tagged release archives include this docs tree alongside the binary, license,
changelog, and security policy so operators can keep the local reference set
with the installed release.

## Future wiki seed

If the project later enables a GitHub wiki, starter pages live under
[wiki-seed/README.md](./wiki-seed/README.md).
