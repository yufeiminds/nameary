# Release information

## Supported platforms

Prebuilt binaries are produced for:

- macOS: `amd64`, `arm64`
- Linux: `amd64`, `arm64`
- Windows: `amd64`, `arm64`

Building from source requires **Go 1.26+** because of Charm v2.

## Install

Homebrew (macOS/Linux):

```bash
brew install yufeiminds/tap/nameary
```

mise (macOS/Linux/Windows):

```bash
mise use -g github:yufeiminds/nameary@latest
```

Or download the archive for your platform from GitHub Releases and put the
`nameary` binary on `PATH`. Building from source requires Go 1.26+:

```bash
go install github.com/yufeiminds/nameary/cmd/nameary@latest
```

## Modes

| Invocation | Result |
|---|---|
| `nameary acme` on a TTY | interactive TUI (default) |
| `nameary acme --format console` | grouped, progressive console output |
| `nameary acme --format table` | plain aligned table |
| `nameary acme --format json` | one JSON array |
| `nameary acme --format ndjson` | one JSON object per settled check |
| `nameary acme \| cat` | automatic table fallback |

`--style plain` is the one-switch accessibility mode: ASCII glyphs, no color,
no animation.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | checks completed |
| 1 | runtime/config error |
| 2 | usage error or TUI requested without a TTY |
| 3 | `--fail-if-taken` and at least one result is `taken` |

## JSON schema

`json`/`ndjson` records have this stable shape:

```json
{
  "name": "acme",
  "target": "npm",
  "label": "acme",
  "status": "available|taken|unknown|timeout|rate_limit|error|skipped|canceled",
  "confidence": "certain|likely|low",
  "query": "acme",
  "summary": "v3.0.3",
  "url": "https://www.npmjs.com/package/acme",
  "latency_ms": 312,
  "cached": false,
  "fields": [{"key": "registry", "value": "npm"}]
}
```

Fields may be added in any release. Consumers should ignore unknown keys and
not assume that `fields` is ordered across versions.

## Configuration and environment

- Config: `~/.config/nameary/config.toml` or `$NAMEARY_CONFIG`.
- Cache: `$XDG_CACHE_HOME/nameary/cache.db` (platform equivalent).
- `GITHUB_TOKEN` / `GH_TOKEN`: raise the GitHub API quota.
- `NO_COLOR`: disable color.
- `NAMEARY_REDUCED_MOTION=1`: disable non-essential animation.

## Data caveats

- `available` means “no record was found”, not “registrable”.
- Domain availability is not a trademark, price, or legal guarantee.
- A timeout is reported as `timeout`, a rate limit as `rate_limit`, and a server error as `unknown`.
- See [`sources.md`](sources.md) for per-source behavior.

## Verifying a release

```bash
sha256sum -c checksums.txt
nameary version
nameary check acme --format ndjson --cache-mode off
```
