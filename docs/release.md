# Release And Distribution

Bare Systems Installer is distributed as a verifiable GitHub Release surface with archives, checksums, Linux packages, artifact attestations, and a checksum-verifying install script. Windows is not a supported edge runtime in the first release milestone.

## Release Channels

| Channel | Tag Pattern | Audience | Stability |
| --- | --- | --- | --- |
| `canary` | `vX.Y.Z-canary.N` | internal and design-partner validation | built from early release candidates |
| `beta` | `vX.Y.Z-beta.N` | customer preview environments | feature complete, still under validation |
| `stable` | `vX.Y.Z` | production edge devices | approved production release |

All public release tags use the `v` prefix. Stable tags must not include prerelease suffixes.

## Artifacts

Tagged releases publish:

- `bare-systems_linux_amd64.tar.gz`
- `bare-systems_linux_arm64.tar.gz`
- `bare-systems_darwin_amd64.tar.gz`
- `bare-systems_darwin_arm64.tar.gz`
- `.deb` packages for Linux amd64 and arm64
- `.rpm` packages for Linux amd64 and arm64
- `checksums.txt`
- GitHub artifact attestations for release artifacts

Archives include the `bare-systems` binary plus the README and release/runtime docs. Linux packages install the binary to `/usr/bin/bare-systems`.

## Install Script

The install script detects OS and architecture, downloads the matching Bare Systems archive and `checksums.txt`, verifies the SHA-256 checksum, and installs the binary.

It also installs the Tardigrade host CLI from the `Bare-Systems/Tardigrade` GitHub release before deployment runtime commands are used. The current bootstrap path is intentionally hard-coded to the Linux x86_64 Tardigrade archive, `tardigrade-linux-x86_64.tar.gz`, and installs both `tardigrade` and the `tardi` alias into the same install directory.

```sh
curl -fsSL https://raw.githubusercontent.com/Bare-Systems/Bare-Systems-Installer/main/scripts/install.sh | sh
```

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/Bare-Systems/Bare-Systems-Installer/main/scripts/install.sh | \
  BARE_SYSTEMS_VERSION=v0.1.0 sh
```

Install to a non-default directory:

```sh
BARE_SYSTEMS_INSTALL_DIR="$HOME/.local/bin" \
  sh scripts/install.sh
```

Skip the Tardigrade install when only refreshing the Bare Systems CLI:

```sh
BARE_SYSTEMS_SKIP_TARDIGRADE=1 sh scripts/install.sh
```

Install a specific Tardigrade release or override the temporary hard-coded asset:

```sh
BARE_SYSTEMS_TARDIGRADE_VERSION=v0.1.0 \
BARE_SYSTEMS_TARDIGRADE_ASSET=tardigrade-linux-x86_64.tar.gz \
  sh scripts/install.sh
```

The `.deb` and `.rpm` packages currently install only `bare-systems`; use the install script on hosts that need the installer to provision Tardigrade automatically.

## Verification

Manual archive verification:

```sh
curl -fsSLO https://github.com/Bare-Systems/Bare-Systems-Installer/releases/download/v0.1.0/bare-systems_linux_amd64.tar.gz
curl -fsSLO https://github.com/Bare-Systems/Bare-Systems-Installer/releases/download/v0.1.0/checksums.txt
grep ' bare-systems_linux_amd64.tar.gz$' checksums.txt | sha256sum -c -
```

On macOS, replace `sha256sum -c -` with `shasum -a 256 -c -`.

GitHub artifact attestations are published by the release workflow. Operators can verify provenance with the GitHub CLI:

```sh
gh attestation verify bare-systems_linux_amd64.tar.gz \
  --repo Bare-Systems/Bare-Systems-Installer
```

## Release Workflow

The CI workflow builds Linux and macOS binaries for amd64 and arm64 on every push and pull request.

The release workflow runs when a `v*` tag is pushed:

1. Run the Go test suite.
2. Build release binaries with version, commit, and build date metadata.
3. Publish archives, `.deb`, `.rpm`, and `checksums.txt` through GoReleaser.
4. Publish GitHub artifact attestations for the files in `dist/`.

## Homebrew

Homebrew tap automation is planned for the first public distribution path, but the tap repository is intentionally not hard-coded until ownership and naming are finalized. The planned formula should install the macOS archive for the matching architecture, verify the release checksum, and expose `bare-systems` on `PATH`.

## Updates

Operators can update by installing a newer release with the install script, package manager, or Homebrew once the tap is active. Runtime deployment updates remain a separate CLI concern handled by `bare-systems update`; updating the installer binary does not automatically update customer workloads. When using the install script, Tardigrade is refreshed during the CLI install step rather than during deployment start/update.
