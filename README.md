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
  <a href="#install"><img src="https://img.shields.io/badge/Homebrew-install-FBB040?logo=homebrew&logoColor=white" alt="Homebrew"></a>
  <a href="#install"><img src="https://img.shields.io/badge/mise-install-6E56CF" alt="mise"></a>
</p>

</div>

`nameary` checks a name across **20 sources** — domains, package registries and
handles — in parallel, streaming each result as it arrives. On a terminal it
opens a full-screen interactive TUI; piped, it prints a plain table.

```bash
nameary acme
```

![nameary demo](demo/nameary.gif)

| TUI | Console |
|---|---|
| ![TUI](demo/tui.png) | ![Console](demo/console.png) |

## Why it feels fast

- **Immediate feedback.** The TUI draws every row as a skeleton before the
  first network request finishes; `--format console` prints the logo and
  groups, then streams one result line per completed check.
- **First real result usually < 400ms.** Fast registries start alongside the
  slower domain checks.
- **No surprise sorting.** The TUI keeps rows in a stable order; a settled row
  flashes once instead of jumping.
- **Cancellation is real.** `Ctrl+C` (or `q` in the TUI) cancels in-flight work.
- **Honest results.** A timeout, a 429 and a 404 are different states. Only a
  clear “not found” becomes `available`.

## Install

### Homebrew (recommended)

```bash
brew install yufeiminds/tap/nameary
```

### mise (recommended)

```bash
mise use -g github:yufeiminds/nameary@latest
```

Both install a prebuilt binary; no Go toolchain is required.

### Other options

Download the archive for your platform from
[GitHub Releases](https://github.com/yufeiminds/nameary/releases), or build
from source with **Go 1.26+**:

```bash
go install github.com/yufeiminds/nameary/cmd/nameary@latest
```

Or from a checkout:

```bash
git clone https://github.com/yufeiminds/nameary
cd nameary
go build -o nameary ./cmd/nameary
```

## First run

```bash
nameary acme
```

Every planned check appears immediately as a skeleton row and fills in as each
source answers — the first results usually arrive within a few hundred
milliseconds. Press `?` for help, `q` to quit.

### Reading the results

Every row is one source, with a status and a confidence:

| Status | Meaning |
|---|---|
| `available` | A clear “not found” from the source. |
| `taken` | The name exists at that source. |
| `timeout` | The source did not answer within the timeout. |
| `rate_limit` | The source returned 429 or its quota is exhausted. |
| `unknown` | Inconclusive: a blocked WHOIS, truncated results or an unclassified response. Never reported as `available`. |
| `error` / `skipped` / `canceled` | No verdict: the source failed, the check could not run (for example the name is not valid there), or the run was interrupted. |

Confidence weighs an `available` result: `certain` — the source's own API
answered; `likely` — a strong signal (for example RDAP has no registration
record for the domain); `low`.

> `available` is **not** a guarantee. Reserved and premium names can still be
> unavailable or cost more — confirm with a registrar before relying on it.
> Availability is also not trademark clearance (see
> [Scope and caveats](#scope-and-caveats)).

## Recipes

### Fail CI when the name is taken

```bash
nameary check acme --format json --fail-if-taken
```

`--fail-if-taken` exits with code 3 when any checked source reports `taken`,
so it drops straight into a script or workflow step. All exit codes are listed
in the [reference](#exit-codes).

### Check a full domain

A name that parses as a domain checks just that domain:

```bash
nameary acme.dev                 # only the domain acme.dev
nameary acme.dev --targets npm   # ... plus package targets explicitly
```

An explicit `--targets` or `--preset` always wins over the domain shortcut.

### Check for a specific ecosystem

Presets bundle targets and TLDs for a stack — see the
[presets reference](#presets) for the full table:

```bash
nameary myname --preset go       # domains, GitHub, Homebrew + .com/.io/.dev
nameary myname --preset rust
nameary myname --preset indie    # the fast, four-source subset
```

### Pick the sources to check

```bash
nameary acme --targets npm,pypi,github
nameary acme --targets all --tlds com,dev
```

### Avoid GitHub rate limits

Without a token, GitHub allows 60 requests/hour and `nameary` falls back to
the public profile page with a 12h cache. Set a token for the REST API and
richer details:

```bash
export GITHUB_TOKEN=ghp_…        # GH_TOKEN also works
nameary acme
```

A token can also come from `--github-token` or the `[github]` config section.

### Get machine-readable output

`table` is the default when piped, `json` emits one array after all checks
finish, and `ndjson` streams one object per settled check:

```bash
nameary check acme --format ndjson | jq .
```

The formats are described in [Output formats](#output-formats).

### Control the cache

Results are cached per target. Re-check from scratch, or find and clear the
database:

```bash
nameary acme --cache-mode off      # skip the cache
nameary acme --cache-mode refresh  # ignore cached rows, re-check
nameary cache path
nameary cache clear
```

### Make output plain and accessible

```bash
nameary acme --style plain
```

`plain` is shorthand for ASCII glyphs + no color + reduced motion;
`--style ascii,calm` combines individual choices. `NO_COLOR`, `CLICOLOR=0`,
`NAMEARY_REDUCED_MOTION=1` and `TERM=dumb` are honored automatically (see
[environment variables](#environment-variables)).

## Reference

### Commands

| Command | Purpose |
|---|---|
| `nameary [name]` | Check a name (interactive TUI on a TTY) |
| `nameary check <name>` | Check exactly one name; same flags and formats |
| `nameary version` / `nameary --version` | Print version information |
| `nameary cache path` | Print the cache database path |
| `nameary cache clear` | Remove the cache database |
| `nameary config` / `nameary config path` | Print the config file path |
| `nameary config init` | Create a config file (interactive on a TTY; `--force`, `--preset`, `--targets`, `--tlds` for non-interactive use) |

### Flags

| Flag | Description |
|---|---|
| `--preset` | `all` (default), `indie`, `web`, `go`, `rust`, `jvm`, `apple`, `linux`, `data` |
| `--targets` | comma-separated target list, or `all` |
| `--tlds` | comma-separated TLDs |
| `--concurrency` | global concurrent checks (default 16) |
| `--timeout` | per-request timeout override (default 3s) |
| `--format` | `auto`, `tui`, `console`, `table`, `json`, `ndjson` |
| `--style` | `auto`, `plain`, `ascii`, `mono`, `calm` (comma-separated) |
| `--only-available` | only emit available results |
| `--cache-mode` | `auto`, `off`, `refresh` |
| `--cache-ttl` | override the per-target cache TTL |
| `--cache` | cache database path |
| `--celebrate` | toast when everything is available |
| `--fail-if-taken` | exit 3 if anything is taken |
| `--github-token` | GitHub token (or `GITHUB_TOKEN` / `GH_TOKEN`) |
| `--debug` | write `debug.log` to the cache directory |

### Presets

| Preset | Targets | TLDs |
|---|---|---|
| `all` (default) | every supported target | com, io, dev, app, ai, co |
| `indie` | domains, npm, pypi, github | com, io, dev, app, ai, co |
| `web` | domains, npm, pypi, github, jsr | com, io, dev, app |
| `go` | domains, github, homebrew | com, io, dev |
| `rust` | domains, crates, github | com, io, dev |
| `jvm` | domains, maven, github | com, io, dev |
| `apple` | domains, cocoapods, github | com, io, dev |
| `linux` | domains, aur, snap, homebrew, github | com, io, dev |
| `data` | domains, conda, pypi, github | com, io, dev |
| `all` | every target | com, io, dev, app, ai, co |

### Targets

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

Per-source endpoints, limits and cache TTLs:
[`docs/sources.md`](docs/sources.md).

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Run completed; nothing was taken (or `--fail-if-taken` was not set) |
| `1` | Invalid options (unknown preset, style or cache mode), config problems, or the run failed |
| `2` | Usage error: more than one name, flags placed after the name, a missing name when not on a TTY, TUI requested without a TTY, or an unknown `--format` value |
| `3` | `--fail-if-taken` was set and at least one result is `taken` |

### Environment variables

| Variable | Effect |
|---|---|
| `GITHUB_TOKEN`, `GH_TOKEN` | GitHub token for the REST API (also configurable via `[github] token_env`) |
| `NAMEARY_CONFIG` | Path to the config file |
| `NO_COLOR` | Disable color (color is also disabled when piped) |
| `CLICOLOR=0` | Disable color |
| `NAMEARY_REDUCED_MOTION=1` | Reduce animation |
| `TERM=dumb` | Force plain, non-interactive behavior |

### Configuration

The config file lives in your user configuration directory — `nameary config
path` prints it (`~/.config/nameary/config.toml` on Linux,
`~/Library/Application Support/nameary/config.toml` on macOS);
`NAMEARY_CONFIG` overrides the location.

| Key | Default | Description |
|---|---|---|
| `preset` | `"all"` | Preset: sets `targets` and `tlds` |
| `targets` | from preset | Sources to check |
| `tlds` | from preset | TLDs to check |
| `concurrency` | `16` | Global concurrent checks |
| `timeout` | `"3s"` | Global per-request timeout override |
| `cache_ttl` | per-target | Global cache TTL override, e.g. `"12h"` |
| `celebrate` | `false` | Toast when everything is available |
| `[github] token_env` | `"GITHUB_TOKEN"` | Environment variable to read the GitHub token from |

```toml
preset = "all"
tlds = ["com", "io", "dev", "app", "ai", "co"]
targets = ["domains", "npm", "pypi", "github"]
concurrency = 16
timeout = "3s"
# cache_ttl = "12h"   # optional global override; per-target TTLs apply otherwise
celebrate = false

[github]
token_env = "GITHUB_TOKEN"
```

Precedence: flag > environment > config > default.

### Output formats

| Format | Behavior |
|---|---|
| `auto` | `tui` on a full TTY; `console` when stdout is a terminal but stdin is not; `table` when piped |
| `tui` | full-screen interactive TUI (requires a TTY) |
| `console` | progressive non-interactive stream: logo and group summary first, then one line per completed check; falls back to grouped blocks when the terminal is too short or with `--only-available` |
| `table` | plain aligned table |
| `json` | one JSON array after all checks finish |
| `ndjson` | one JSON object per settled check |

Color is disabled automatically when output is piped. `ndjson` lines look like
this (an npm check and a domain check):

```json
{"name":"acme","target":"npm","label":"acme","status":"taken","confidence":"certain","query":"acme","summary":"v3.0.3","url":"https://www.npmjs.com/package/acme","latency_ms":436}
{"name":"zzqxvfqk.dev","target":"domains","label":"zzqxvfqk.dev","status":"available","confidence":"likely","query":"zzqxvfqk.dev","summary":"no registration record","url":"https://pubapi.registry.google/rdap/domain/zzqxvfqk.dev","latency_ms":726}
```

### Keyboard (TUI)

| Key | Action |
|---|---|
| `↑`/`↓` or `k`/`j` | move selection |
| `pgup` / `pgdown` | page |
| `g` / `G` | jump to top / bottom |
| `enter` | open row details |
| `/` | filter rows |
| `a` | toggle available-only |
| `s` | cycle sort: group order → status → latency |
| `tab` | next group |
| `r` | rerun the check |
| `e` / `E` | export results as JSON / Markdown |
| `c` | copy the source URL |
| `o` | open the source in a browser |
| `?` | help |
| `esc` | back / cancel |
| `q` or `ctrl+c` | quit |

## How results are determined

### Domains are deliberately conservative

1. **DNS NS lookup** is a fast “taken” signal. If the domain is delegated, it is
   registered; RDAP is skipped.
2. **RDAP** is the authoritative check. A 404 with a real “object not found”
   body becomes `available` with confidence `likely`.
3. **No RDAP service** (common for some ccTLDs) falls back to **WHOIS** on port
   43. If WHOIS is blocked or inconclusive, the result is `unknown`, never
   `available`.
4. `available` is **not** a guarantee: reserved and premium names can still be
   unavailable or cost more. Confirm with a registrar before relying on it.

### GitHub without a token

GitHub allows 60 anonymous requests/hour, so without a token `nameary` checks
the public profile page (`HEAD github.com/<name>`) instead — 1 request/second
with a 12h cache — and a single run does not exhaust the quota. With a token it
uses the REST API and returns richer details.

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

When running from a checkout with `go run`, note there is no `--` between the
package path and nameary's arguments:

```bash
go run ./cmd/nameary acme
go run ./cmd/nameary acme --format console
```

Demo images/GIFs are stored with **Git LFS**. Install `git-lfs` and run
`git lfs pull` if you need the demo assets; the binary and tests do not require
them. Re-record the demo (requires [VHS](https://github.com/charmbracelet/vhs)):

```bash
vhs demo/demo.tape
```

Release readiness: see [`docs/release-checklist.md`](docs/release-checklist.md).
Release notes and schema: [`docs/release.md`](docs/release.md).
Per-source endpoints and limits: [`docs/sources.md`](docs/sources.md).
Security policy: [`SECURITY.md`](SECURITY.md); audit:
[`docs/security-audit.md`](docs/security-audit.md).

### Architecture

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
