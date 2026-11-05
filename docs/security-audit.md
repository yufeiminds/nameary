# Security audit — v0.1.0

Date: 2026-11-05

## Scope

The audit covered the CLI, TUI, console/table/JSON renderers, async engine,
registry adapters, domain DNS/RDAP/WHOIS path, cache, config and release
tooling.

## Tool results

| Check | Result |
|---|---|
| `govulncheck ./...` (Go 1.26.10) | no vulnerabilities found |
| `gosec -quiet ./...` | no findings |
| `staticcheck ./...` | no findings |
| `go vet ./...` | pass |
| `go test -race ./...` | pass |
| secret scan (`ghp_`, PAT, private keys, AWS keys) | no matches |
| `go mod verify` | all modules verified |
| `goreleaser check` / snapshot release | pass |

The initial scan ran on Go 1.26.4 and reported 16 standard-library
vulnerabilities. The project now requires **Go 1.26.10** (the patch level where
those advisories are fixed), and the CI/release/smoke workflows pin that
version.

## Issues found and fixed

1. **Standard-library vulnerabilities**
   - Fixed by requiring Go 1.26.10 and updating CI, release and smoke workflows.
2. **NPM/Packagist oversized responses truncated before JSON decode**
   - `registry.npmjs.org/react` is ~7 MB and `packagist.org` package metadata is
     ~7.7 MB; the 4 MB body cap caused decode errors. NPM now requests
     `/{name}/latest` (a few KB) and Packagist uses the official
     `repo.packagist.org/p2` metadata (about 1 MB). The global body cap remains
     4 MB.
3. **Terminal escape injection**
   - User input and registry/WHOIS metadata could contain CSI/OSC sequences,
     newlines or bidi controls. Added `internal/textutil.Clean` and applied it at
     every terminal rendering boundary (TUI, console, table). JSON output keeps
     data intact; JSON encoding escapes controls.
4. **Unsafe external URL handling**
   - The TUI browser opener now accepts only absolute `http`/`https` URLs with a
     host. `javascript:`, `file:`, `ftp:` and relative URLs are rejected.
   - Redirects are capped at five and must remain HTTPS.
5. **Predictable export paths / symlink risk**
   - JSON/Markdown exports now use `os.CreateTemp` with unique names and `0600`
     permissions instead of predictable `os.WriteFile` paths.
6. **Config write-through-symlink risk**
   - Config is written to a `0600` temporary file in the config directory and
     renamed into place.
7. **File permissions**
   - Cache/config directories use `0700`; cache, config, debug log and exports
     use `0600`. Existing cache/debug files are re-chmodded on open.
8. **Unbounded resource inputs**
   - Name length is capped at 512 characters (TUI input 256); TLD count at 64;
     global concurrency at 128; timeout override at 5 minutes.
9. **Weak random source**
   - Retry jitter now uses `crypto/rand` instead of `math/rand`.
10. **Token input hygiene**
    - GitHub tokens are trimmed and rejected if they contain CR/LF/NUL. Tokens
      are never logged.
11. **Unchecked close error**
    - Cache open failure now closes the database and returns a meaningful error.

## Intentional gosec exceptions

Three findings are explicitly documented rather than hidden:

- `G304` for reading the user-selected config file and the debug log under the
  user cache directory. These paths are intentionally user-controlled.
- `G204` for the browser opener. The URL is restricted to absolute `http(s)`
  URLs and the command is invoked without a shell.
- `G101` for the string `GITHUB_TOKEN`: it is the name of an environment
  variable, not a hardcoded credential.

## Residual risks

- Registry metadata, RDAP and WHOIS are untrusted. They are treated as display
  data and sanitized for terminal output, but their factual accuracy is not
  guaranteed.
- External sources can rate-limit or change schemas. The conservative `unknown`
  state avoids false “available” claims.
- No signing/notarization or SBOM is configured yet (tracked as P1/P2).
- A user who can write to the user's cache/config directory can alter local
  cached results; terminal sanitization prevents code execution, and incorrect
  cached facts are still possible.
- Opening a source URL launches the user's browser; the URL scheme is validated,
  but the remote site is outside the tool's trust boundary.

## Recommendations before publishing the tag

1. Run the live smoke workflow once on GitHub Actions for macOS, Linux and
   Windows.
2. Enable Dependabot/security updates and keep the Go patch level current.
3. Add release signing/notarization and SBOM in the next release cycle.
4. Re-run `govulncheck`, `gosec` and the live smoke before each tag.
