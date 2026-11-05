<div align="center">

<h1>nameary</h1>

<p><strong>Find a name before you commit.</strong></p>

<p>
  <a href="https://github.com/yufeiminds/nameary/actions/workflows/ci.yml"><img src="https://github.com/yufeiminds/nameary/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/yufeiminds/nameary"><img src="https://pkg.go.dev/badge/github.com/yufeiminds/nameary.svg" alt="Go Reference"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/yufeiminds/nameary" alt="Go Version"></a>
  <a href="https://github.com/yufeiminds/nameary/releases"><img src="https://img.shields.io/github/v/release/yufeiminds/nameary?include_prereleases&sort=semver" alt="Latest Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/yufeiminds/nameary" alt="License"></a>
  <img src="https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-2ea44f" alt="Platforms">
</p>

</div>

`nameary` checks a name across **20 sources** — domains, package registries and
handles — in parallel. On a terminal it opens a full-screen interactive TUI.
Use `--format console` for a progressive non-interactive stream: logo and group
summary first, then one line per completed check. Pipe it for a plain table.

```bash
nameary acme
```

![nameary demo](demo/nameary.gif)

| TUI | Console |
|---|---|
| ![TUI](demo/tui.png) | ![Console](demo/console.png) |

> Re-record with `vhs demo/demo.tape` (VHS required).

## Why it feels fast

- **Immediate feedback.** The TUI draws every row as a skeleton before the
  first network request finishes; `--format console` prints the logo and groups,
  then streams one result line per completed check with a live status line.
- **First real result usually < 400ms.** Fast registries start alongside the
  slower domain checks.
- **No surprise sorting.** The TUI keeps rows in a stable order; a settled row
  flashes once instead of jumping.
- **Cancellation is real.** `Ctrl+C` (or `q` in the TUI) cancels in-flight work.
- **Honest results.** A timeout, a 429 and a 404 are different states. Only a
  clear “not found” becomes `available`.

## Install

Requires **Go 1.26+** (Charm v2).

```bash
go install github.com/yufeiminds/nameary/cmd/nameary@latest
```

Or from a checkout:

```bash
git clone https://github.com/yufeiminds/nameary
cd nameary
go build -o nameary ./cmd/nameary
```

Demo images/GIFs are stored with **Git LFS**. Install `git-lfs` and run
`git lfs pull` if you need the demo assets; the binary and tests do not require
them.

Running directly with `go run` (note: no `--` between the package path and
nameary's arguments):

```bash
go run ./cmd/nameary acme
go run ./cmd/nameary acme --format console
```

## Usage

```bash
nameary acme                     # interactive TUI (default on a TTY)
nameary acme --format console    # progressive console stream
nameary acme --format console --style plain
nameary acme --preset all        # every target (slower)
nameary acme --only-available    # hide taken / unknown rows
nameary acme --format table      # plain aligned table
nameary check acme --format ndjson | jq .
nameary check acme --format json --fail-if-taken
nameary acme.dev                 # a full domain checks just that domain
nameary acme.dev --targets npm   # ... or add package targets explicitly
```

### Presets

| Preset | Targets | Domains |
|---|---|---|
| `indie` (default) | domains, npm, pypi, github | com, io, dev, app, ai, co |
| `web` | domains, npm, pypi, github, jsr | com, io, dev, app |
| `go` | domains, github, homebrew | com, io, dev |
| `rust` | domains, crates, github | com, io, dev |
| `jvm` | domains, maven, github | com, io, dev |
| `apple` | domains, cocoapods, github | com, io, dev |
| `linux` | domains, aur, snap, homebrew, github | com, io, dev |
| `data` | domains, conda, pypi, github | com, io, dev |
| `all` | every target | com, io, dev, app, ai, co |

### Flags

| Flag | Description |
|---|---|
| `--preset` | `indie`, `web`, `go`, `rust`, `jvm`, `apple`, `linux`, `data`, `all` |
| `--targets` | comma-separated target list, or `all` |
| `--tlds` | comma-separated TLDs |
| `--concurrency` | global concurrent checks (default 16) |
| `--timeout` | per-request timeout override (most targets default 8s) |
| `--format` | `auto`, `tui`, `console`, `table`, `json`, `ndjson` |
| `--style` | `auto`, `plain`, `ascii`, `mono`, `calm` (comma-separated) |
| `--only-available` | only emit available results |
| `--cache-mode` | `auto`, `off`, `refresh` |
| `--cache-ttl` | override the per-target cache TTL |
| `--cache` | cache database path |
| `--celebrate` | toast when everything is available |
| `--fail-if-taken` | exit 3 if anything is taken |
| `--github-token` | GitHub token (or `GITHUB_TOKEN` / `GH_TOKEN`) |
| `--debug` | write debug logs to the cache directory |

`--style plain` is shorthand for ASCII glyphs + no color + reduced motion;
`--style ascii,calm` combines individual choices. `NO_COLOR` and
`NAMEARY_REDUCED_MOTION=1` are still honored.

Keyboard (TUI): `↑↓` move, `⏎` details, `/` filter, `a` available-only,
`s` sort, `r` rerun, `c` copy, `o` open source, `e`/`E` export JSON/Markdown,
`?` help, `esc` back/cancel, `q` quit.

## Targets

| Target | Source | “Available” means |
|---|---|---|
| `domains` | DNS NS + RDAP (+ WHOIS fallback) | no registration record found |
| `npm` | registry.npmjs.org | 404 |
| `pypi` | pypi.org/pypi | 404 |
| `crates` | crates.io API | 404 |
| `rubygems` | rubygems.org API | 404 |
| `nuget` | NuGet flat container | 404 |
| `dockerhub` | Docker Hub API | no user/org namespace, no official image |
| `homebrew` | formulae.brew.sh | no formula and no cask |
| `packagist` | packagist.org | 404 for `vendor/package` |
| `hex` | hex.pm API | 404 |
| `pub` | pub.dev API | 404 |
| `github` | REST API or public profile | 404 / profile page 404 |
| `maven` | Maven Central Solr search | artifact ID absent across all groups |
| `conda` | Anaconda package search | no exact package name across channels |
| `cocoapods` | cocoapods.org/pods | 404 |
| `aur` | AUR RPC v5 | `resultcount` is 0 |
| `snap` | Snap Store API | 404 |
| `chocolatey` | Chocolatey OData feed | empty Atom feed |
| `hackage` | hackage.haskell.org | 404 |
| `jsr` | JSR search / scoped meta | no exact package name |

### Domain results are deliberately conservative

1. **DNS NS lookup** is a fast “taken” signal. If the domain is delegated, it is
   registered; RDAP is skipped.
2. **RDAP** is the authoritative check. A 404 with a real “object not found”
   body becomes `available` with confidence `likely`.
3. **No RDAP service** (common for some ccTLDs) falls back to **WHOIS** on port
   43. If WHOIS is blocked or inconclusive, the result is `unknown`, never
   `available`.
4. `available` is **not** a guarantee: reserved and premium names can still be
   unavailable or cost more. Confirm with a registrar before relying on it.

## Configuration

`~/.config/nameary/config.toml` (or `$NAMEARY_CONFIG`):

```toml
preset = "indie"
tlds = ["com", "io", "dev", "app", "ai", "co"]
targets = ["domains", "npm", "pypi", "github"]
concurrency = 16
# timeout = "8s"   # optional global override; per-target timeouts apply otherwise
# cache_ttl = "12h"   # optional global override; per-target TTLs apply otherwise
celebrate = false

[github]
token_env = "GITHUB_TOKEN"
```

```bash
nameary config init     # interactive form on a TTY
nameary config path
```

Precedence: flag > environment > config > default.

### GitHub quota

Without a token GitHub allows 60 requests/hour. `nameary` falls back to the
public profile page (`HEAD github.com/<name>`) with 1 request/second and a 12h
cache. Set `GITHUB_TOKEN` for the REST API and richer details.

## Output

```bash
$ nameary check acme --format ndjson
{"name":"acme","target":"npm","label":"acme","status":"taken","confidence":"certain","query":"acme","summary":"v3.0.3","url":"https://www.npmjs.com/package/acme","latency_ms":348}
```

On a TTY the default is the interactive TUI. `--format console` prints the
logo and grouped skeleton first, then fills each row in place as checks settle;
it is still a plain non-interactive program, not the TUI. On terminals that are
too short, or for `--only-available`, it falls back to grouped blocks. `table`
is the default when piped. `json` emits one array after all checks finish;
`ndjson` streams one object per settled check. Table and console output
auto-disable color when piped.

## Architecture

```
Cobra ──► app ──┬─► TUI (default TTY)
                ├─► console stream (--format console)
                ├─► table / json / ndjson
                └─► engine ──► check adapters
                                  ├─ npm / pypi / crates / ...
                                  ├─ domain (DNS + RDAP + WHOIS)
                                  └─ cache (bbolt)
```

The engine is UI-agnostic. It emits `started` / `settled` events over a buffered
channel; the TUI reads them one at a time with a `tea.Cmd` bridge and drops
events from stale runs by `runID`. The console renderer keeps grouped rows and
fills them in place as results settle. Every checker declares its own concurrency, timeout,
priority and cache TTL.

## Development

```bash
go test ./...
go test -race ./internal/...
go test -count=1 ./e2e
go vet ./...
go build -o nameary ./cmd/nameary
```

Test layers, the live smoke matrix and local `act` CI are described in
[`docs/testing.md`](docs/testing.md).

Record the demo (requires [VHS](https://github.com/charmbracelet/vhs)):

```bash
vhs demo/demo.tape
```

Release readiness: see [`docs/release-checklist.md`](docs/release-checklist.md).
Release notes and schema: [`docs/release.md`](docs/release.md).
Per-source endpoints and limits: [`docs/sources.md`](docs/sources.md).
Security policy: [`SECURITY.md`](SECURITY.md); audit:
[`docs/security-audit.md`](docs/security-audit.md).

Project layout:

```
cmd/nameary          entrypoint
internal/app         wiring, planning, summaries
internal/engine      scheduling, retries, events
internal/check       checker contract + HTTP helpers
internal/check/adapters  registry adapters
internal/check/domain    DNS, RDAP, WHOIS
internal/cache       bbolt cache
internal/tui         Bubble Tea model, view, animations
internal/render      table / JSON / NDJSON
internal/cli         Cobra commands and flags
internal/config      TOML config and presets
```

## Project

- [Contributing](CONTRIBUTING.md)
- [Governance](GOVERNANCE.md) and [maintainers](MAINTAINERS.md)
- [Code of conduct](CODE_OF_CONDUCT.md)
- [Security policy](SECURITY.md) and [security audit](docs/security-audit.md)
- [Threat model](docs/threat-model.md) and [assurance case](docs/assurance-case.md)
- [Support](SUPPORT.md)
- [Roadmap](ROADMAP.md)
- [Distribution plan](docs/distribution.md)
- [Open-source maturity](docs/open-source-maturity.md)
- [Changelog](CHANGELOG.md)

## Scope and caveats

- Availability is **not trademark clearance**. Use USPTO/EUIPO search for that.
- Domain prices are not shown: there is no stable free registrar pricing API.
  The detail view links to the source instead.
- Go modules are not checked as bare names (a module is a full path); use the
  GitHub handle target instead.
- Rate limits are respected per host, but registries can still change or block
  traffic. Such cases show as `unknown`, not `available`.

## License

Apache-2.0
