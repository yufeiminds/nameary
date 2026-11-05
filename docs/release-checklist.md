# Release checklist

## Current status

| Area | Status |
|---|---|
| Build / tests | `go build ./...`, `go test ./internal/...`, `go test -race ./internal/...`, `go test -count=1 ./e2e`, `go vet ./...`, `gofmt` pass |
| Security | govulncheck/gosec/staticcheck clean; audit in `security-audit.md` |
| OpenSSF | governance, contribution, support, threat model, CodeQL, Scorecard, Dependabot, dependency review |
| Targets | 20 live sources; unit + e2e coverage; live smoke tested |
| CLI modes | TUI by default on a TTY; `--format console` progressive; `table/json/ndjson` headless |
| Packaging | goreleaser builds + deb/rpm/apk; release workflow on `v*` tags |
| CI | Linux test/race/e2e/vet/build + manual/scheduled live smoke matrix |
| Docs | README + GIF/screenshots + source and release docs |
| Distribution | no Homebrew tap, Scoop/winget, signing, SBOM, or docs site |
| Repo | committed and tagged `v0.1.0` (not pushed) |

## P0 — completed

- [x] Commit and tag `v0.1.0`; release workflow on `v*` tags.
- [x] Live smoke script for all 20 targets; local macOS run passed. The
  `smoke.yml` workflow runs the same script on macOS/Linux/Windows on demand and
  weekly (first run still needs to happen in GitHub Actions).
- [x] User-Agent/contact, per-host concurrency, cache, GitHub token guidance and
  source notes documented in [`sources.md`](sources.md).
- [x] VHS demo recorded; GIF and TUI/console screenshots added to README.
- [x] Release docs: supported platforms, exit codes, JSON schema, domain caveats
  in [`release.md`](release.md).
- [x] Apache-2.0 + third-party notices; license/notices included in archives
  and packages.
- [x] Security audit: Go 1.26.10, govulncheck/gosec/staticcheck clean, terminal
  sanitization and URL/file-permission hardening applied. See `security-audit.md`.

## P1 — distribution and confidence

- Homebrew tap, asdf plugin and mise installs; Scoop/winget manifests;
  verify deb/rpm/apk installs.
- Shell completions (Cobra already generates them) and a man page.
- `golangci-lint`, `govulncheck`, SBOM (syft or goreleaser SBOM), and
  dependency update automation.
- CI matrix: macOS + Windows in addition to Linux; separate network-free tests
  from live smoke tests so PRs never depend on external APIs.
- CLI integration tests with a PTY for TUI startup/quit and console animation.
- Explicit version injection (`-X .../version.Version`) so `go build` does not
  report `(devel)` in local builds.

## P2 — hardening

- macOS signing/notarization and Windows Authenticode (optional but reduces
  friction).
- Docker image / GitHub Container Registry if there is demand.
- HTTP proxy support, custom CA support, and offline mode documentation.
- Config migration/versioning, cache format versioning, and cache pruning.
- Coverage reporting and benchmarks for the engine and renderers.

## Release command sequence

```bash
gofmt -l .
go vet ./...
go test -race ./internal/...
go test -count=1 ./e2e
goreleaser check
goreleaser release --clean   # on a clean v* tag
```

## Main risks

- External APIs change or rate-limit without notice; the conservative
  `unknown` state limits false positives.
- CocoaPods/Hackage use page-existence checks, not registry metadata.
- Conda's fuzzy search can be truncated; it returns `unknown` instead of
  guessing.
- Default TUI must be disabled automatically in non-TTY/CI contexts; this is
  already handled, but keep an integration test for it.
- WHOIS port 43 can be blocked; the domain checker reports `unknown`, not
  `available`.
