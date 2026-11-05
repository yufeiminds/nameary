# Assurance case

This document records the project's security and quality claims and the
evidence supporting them. It is intentionally conservative: gaps are listed.

## Claim 1 — The release build is reproducible from source

**Evidence**

- `go.mod`/`go.sum` pin every module; `go mod verify` passes.
- CI builds with Go 1.26.10 and runs `goreleaser` from a clean checkout.
- `goreleaser` produces cross-platform archives and checksums.
- `Makefile` uses the same version/commit/date ldflags as goreleaser.

**Gap**: builds are not yet bit-for-bit reproducible, and artifacts are not
signed/notarized.

## Claim 2 — Untrusted terminal text cannot control the terminal

**Evidence**

- `internal/textutil.Clean` strips C0/C1 controls, ESC, and bidi overrides.
- TUI, console and table rendering sanitize user names and registry metadata.
- Tests (`TestTableSanitizesUntrustedText`, `TestConsoleSanitizesUntrustedText`,
  `TestViewSanitizesUntrustedText`) assert that escapes do not survive.

**Gap**: a compromised terminal emulator is out of scope.

## Claim 3 — Availability errors are conservative

**Evidence**

- Adapters return `available` only on an explicit 404 / empty registry record.
- Timeouts, 429, parse errors and 5xx map to `unknown`/`error`.
- Domain checks use RDAP before claiming availability; WHOIS gaps return
  `unknown`.
- Smoke tests include known-taken packages for every target.

**Gap**: external data can be stale or wrong; confidence levels and source links
communicate this.

## Claim 4 — Local files and credentials are protected

**Evidence**

- Cache/config/debug directories are `0700`; cache/log/exports are `0600`.
- Exports use unpredictable `os.CreateTemp` names.
- Config writes use a temp file + rename.
- Tokens are never logged; token control characters are rejected.
- Browser opening is restricted to validated `http(s)` URLs.

**Gap**: a same-user attacker can read or modify local state.

## Claim 5 — Changes are reviewed and tested

**Evidence**

- `CONTRIBUTING.md` documents the review and test process.
- CI runs gofmt, vet, race tests, build and govulncheck.
- CodeQL, Dependency Review, Dependabot and OpenSSF Scorecard are configured.
- `CODEOWNERS` assigns review ownership.
- Releases require a green tag build and the release checklist.

**Gap**: the project currently has one maintainer, so independent review is
limited. Adding a second reviewer is a maturity goal.

## Claim 6 — Vulnerabilities are handled responsibly

**Evidence**

- `SECURITY.md` defines private reporting through GitHub Security Advisories.
- Supported versions and response expectations are documented.
- `GOVERNANCE.md` delegates security response to maintainers.
- Security fixes can be prepared privately and released from a patch tag.

**Gap**: no formal SLA or bug bounty.

## Claim 7 — The codebase is free of known dependency vulnerabilities

**Evidence**

- `govulncheck` reports no vulnerabilities with Go 1.26.10.
- `gosec` and `staticcheck` report no findings.
- Dependabot and dependency review are enabled.

**Gap**: new advisories can appear at any time; CI must be kept green and the
Go patch level current.

## Claim 8 — The project is openly governed

**Evidence**

- Apache-2.0 license.
- `GOVERNANCE.md`, `MAINTAINERS.md`, `CONTRIBUTING.md`,
  `CODE_OF_CONDUCT.md`, `SUPPORT.md`, `ROADMAP.md` and `CHANGELOG.md`.
- Public issue/PR templates and a documented decision process.

**Gap**: a second maintainer and a formal security review are future goals.
