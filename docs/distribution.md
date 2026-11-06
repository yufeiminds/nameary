# Distribution plan

This document plans first-class installs through Homebrew, asdf and mise. It is
deliberately separate from the release workflow: the **GitHub release is the
single source of truth**, and every installer consumes the same archives and
`checksums.txt`.

## Prerequisites

1. **Make the repository public.** Homebrew, asdf and mise download release
   assets without credentials. A private repository also breaks
   `go install ...@latest`, badges, CodeQL, Scorecard and dependency review.
2. **Publish a real semver release.** Tag `v0.1.0`, attach the six archives
   (`darwin`, `linux`, `windows` × `amd64`, `arm64`) and `checksums.txt`.
3. **Freeze the asset naming contract.** Installers rely on
   `nameary_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) and the `nameary`
   binary inside. Do not rename assets without updating every installer.
4. **Verify from a clean machine** that each asset downloads, passes SHA-256,
   and `nameary version` reports the tag.
5. **Optional, recommended:** publish GitHub artifact attestations with
   `actions/attest-build-provenance`. mise's `github:` backend then verifies
   provenance in addition to the checksum; Homebrew and asdf keep using
   `checksums.txt`.

## Homebrew

Homebrew is not a mise backend; it is a separate tap.

### Target UX

```bash
brew install yufeiminds/tap/nameary
nameary version
```

### Implementation

1. The public repository `yufeiminds/homebrew-tap` is initialized with
   `brew tap-new`.
2. `packaging/homebrew/nameary.rb.tmpl` defines the formula with per-OS/arch
   URLs and SHA-256 values from the release `checksums.txt`.
3. `scripts/render-homebrew-formula.py` renders the template from
   `dist/checksums.txt`; it fails if any of the four platform checksums is
   missing.
4. `scripts/update-homebrew-tap.sh` clones the tap, renders
   `Formula/nameary.rb`, commits it, and pushes only when the formula changed.
   The release workflow runs it after GoReleaser with the
   `HOMEBREW_TAP_GITHUB_TOKEN` secret.
5. The updater bootstraps the tap when it has no `main` branch, so the first
   release does not require a manual push to `homebrew-tap`.
6. Local validation: render a snapshot, publish it to a temporary empty bare
   tap, `brew tap` + `brew install`, and run `nameary version`.
7. CI validates the renderer and the empty-tap bootstrap path. The release
   workflow also installs from the real tap on macOS after publishing.
8. Verify on a real release:
   - `brew install yufeiminds/tap/nameary`
   - `brew test yufeiminds/tap/nameary`
   - `brew audit --strict --online yufeiminds/tap/nameary`
9. Submit to `homebrew-core` only after the project meets Homebrew's notability
   bar. A tap is the correct first step.

### Formula versus cask

The formula is the right first target for a CLI: it works on macOS and Linux
and does not put the binary through the Cask quarantine/Gatekeeper path. An
unsigned cask can make macOS block or warn on first run, so adopting a cask
later would require an Apple Developer ID and notarization to keep user trust.
If the project later moves to a cask, use `tap_migrations.json`, delete the
formula in the same tap update, and test the migration with an existing
formula installation.

## asdf

asdf is a separate plugin ecosystem. mise can consume an asdf plugin through its
legacy `asdf:` backend, but GitHub releases are the preferred mise path.

### Target UX

```bash
asdf plugin add nameary https://github.com/yufeiminds/asdf-nameary.git
asdf install nameary latest
asdf set nameary latest
```

### Plan

1. Create `yufeiminds/asdf-nameary` from the asdf plugin template. The plugin
   name must be `asdf-nameary`.
2. Implement the required scripts:
   - `bin/list-all`: list GitHub release tags, newest last, without `sort -V`.
   - `bin/download`: pick the asset for `darwin`/`linux` and `amd64`/`arm64`,
     download it into `ASDF_DOWNLOAD_PATH`, and verify SHA-256 against
     `checksums.txt`.
   - `bin/install`: extract into `ASDF_INSTALL_PATH/bin/nameary`.
   - `bin/latest-stable` and the help scripts for a complete plugin.
3. Test with `asdf plugin test nameary <url> 'nameary version'` on Linux and
   macOS. asdf does not support Windows; Windows users should use mise or the
   ZIP archive.
4. Add the plugin to the asdf short-name index
   (`asdf-vm/asdf-plugins`) so `asdf plugin add nameary` also works.
5. Keep the plugin release-agnostic: it queries GitHub at install time, so no
   per-release update is needed.

## mise

mise's own guidance is to prefer the publisher's release assets. The right
backend for nameary is therefore:

```bash
mise use -g github:yufeiminds/nameary@latest
mise exec -- nameary version
```

No registry entry is required for this; the explicit identifier works as soon
as the repository and release are public.

### Backend assessment

| Backend | Use for nameary? | Work required | Decision |
|---|---|---|---|
| `github:` | **Yes, primary** | None beyond GitHub release assets | Ship now via explicit `github:` identifier |
| `aqua:` | Optional | PR a recipe to `aquaproj/aqua-registry` | Recommended after the first stable release; improves verification and gives a registry-friendly backend |
| `packslip:` | Future | Publisher must ship signed manifests | Defer until signing is in scope |
| `http:` | No | Stable URLs plus `version_list_url` | GitHub releases already solve this |
| `s3:` | No | Hosting and credentials | Not needed |
| `go:` | Fallback only | None; module already installable | Document `go install`; do not make it the primary backend |
| `asdf:` | For asdf users, not mise | Separate `asdf-nameary` plugin | Build the plugin, but prefer `github:` for mise |
| `gitlab:` / `forgejo:` | No | Mirror | Repository lives on GitHub |
| `npm:`, `pypi:`, `cargo:`, `gem:`, `dotnet:`, `conda:`, `spm:`, `spinel:` | No | Publish to another language ecosystem | Wrong ecosystem; adds a runtime/toolchain |
| `vfox:` | No | Lua plugin | Unnecessary; `github:` works |
| `ubi:` | No | Deprecated; removed in mise 2027.1 | Do not use |
| `core:` | No | Reserved for mise's built-in tools | Not eligible |

### mise registry short name

A short name such as `mise use nameary` requires a `registry/nameary.toml`
entry in `jdx/mise`. The registry has a popularity bar (normally thousands of
GitHub stars and real external use). nameary should **not** block v0.1 on this.
Once the project meets the bar, submit an entry like:

```toml
backends = [
  "aqua:yufeiminds/nameary",
  "github:yufeiminds/nameary",
]
bins = ["nameary"]
description = "Find a name before you commit"
os = ["linux", "macos", "windows"]
test = { cmd = "nameary version", expected = "nameary {{version}}" }
version_order = "semver"
```

If an aqua recipe is not ready, a `github:`-only entry is acceptable. Do not
submit an `asdf:`, `vfox:` or `ubi:` entry; mise does not accept those for new
registry tools.

## What to upload

Only these distribution artifacts are necessary:

1. **GitHub Releases** — already produced by GoReleaser. This feeds Homebrew,
   asdf and mise.
2. **`yufeiminds/homebrew-tap`** — formula (or cask) repository.
3. **`yufeiminds/asdf-nameary`** — asdf plugin repository.
4. **Optional:** an aqua-registry recipe and, later, a mise registry entry.

Do **not** publish npm, PyPI, crates.io, RubyGems, NuGet, Cargo, .NET, conda,
S3 or a custom HTTP endpoint just to satisfy mise. They add a second supply
chain without improving installation for this Go CLI.

## Delivery order

1. Public repository + signed-off GitHub release.
2. mise explicit `github:` usage in the README (zero new repos).
3. Homebrew tap; verify install, test and audit.
4. asdf plugin and short-name index.
5. aqua recipe.
6. mise registry entry after the popularity bar is met.

## Verification matrix

| Installer | macOS | Linux | Windows |
|---|---|---|---|
| GitHub release archive | yes | yes | yes |
| Homebrew tap | yes | Linuxbrew (formula only) | no |
| asdf plugin | yes | yes | no |
| mise `github:` | yes | yes | yes |
| mise `aqua:` | yes | yes | yes |

Every installer must run `nameary version` and compare it with the release tag.
