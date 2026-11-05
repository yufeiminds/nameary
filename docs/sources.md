# Sources and rate limits

`nameary` identifies itself with:

```text
nameary/0.1 (+https://github.com/yufeiminds/nameary)
```

Every request shares an HTTP client, uses a per-target semaphore and a cache
(bbolt, on by default). Retries are limited to 429/5xx and transient network
errors. Timeouts are reported as `unknown`, never `available`.

## Targets

| Target | Endpoint | Concurrency | Timeout | Cache TTL |
|---|---|---:|---:|---:|
| `domains` | DNS NS + IANA RDAP bootstrap + `rdap.org` + WHOIS fallback | 4 | 8s | 6h |
| `npm` | `registry.npmjs.org/{name}` | 6 | 8s | 24h |
| `pypi` | `pypi.org/pypi/{name}/json` | 4 | 8s | 24h |
| `crates` | `crates.io/api/v1/crates/{name}` | 1 | 10s | 24h |
| `rubygems` | `rubygems.org/api/v1/gems/{name}.json` | 4 | 8s | 24h |
| `nuget` | `api.nuget.org/v3-flatcontainer/{id}/index.json` | 4 | 8s | 24h |
| `dockerhub` | `hub.docker.com/v2/users`, `/orgs`, `/repositories` | 4 | 8s | 24h |
| `homebrew` | `formulae.brew.sh/api/formula`, `/cask` | 4 | 8s | 24h |
| `packagist` | `repo.packagist.org/p2/{vendor}/{name}.json` | 2 | 12s | 24h |
| `hex` | `hex.pm/api/packages/{name}` | 4 | 8s | 24h |
| `pub` | `pub.dev/api/packages/{name}` | 4 | 8s | 24h |
| `github` | GitHub REST (token) or public profile HEAD | 1 (4 with token) | 8s | 12h |
| `maven` | `search.maven.org/solrsearch/select` | 4 | 8s | 24h |
| `conda` | `api.anaconda.org/search?name=...&limit=50` | 3 | 10s | 24h |
| `cocoapods` | `cocoapods.org/pods/{name}` HEAD | 2 | 6s | 24h |
| `aur` | `aur.archlinux.org/rpc/v5/info` | 4 | 8s | 24h |
| `snap` | `api.snapcraft.io/v2/snaps/info/{name}` | 4 | 8s | 24h |
| `chocolatey` | Chocolatey OData `Packages` feed | 3 | 10s | 24h |
| `hackage` | `hackage.haskell.org/package/{name}` HEAD | 4 | 8s | 24h |
| `jsr` | `jsr.io/api/packages?query=...` or scoped `meta.json` | 2 | 8s | 24h |

## Notes and caveats

- **GitHub**: anonymous API quota is 60 requests/hour. Without a token the
  checker uses the public profile page at 1 request/second. Set
  `GITHUB_TOKEN`/`GH_TOKEN` (or `--github-token`) for richer metadata and 5000/h.
- **Domains**: an RDAP 404 is reported as `available` with confidence `likely`.
  It does not mean the name is registrable, affordable, or free of trademark
  issues. Reserved/premium names can still be unavailable. WHOIS is used only
  when no public RDAP service exists; blocked port 43 yields `unknown`.
- **Conda**: Anaconda search is fuzzy and capped at 50 results. If 50 results
  arrive without an exact name match, the result is `unknown` rather than a
  false `available`.
- **JSR**: the bare-name search is capped at 100 results; the same truncation
  rule applies. Scoped `@scope/name` uses the exact `meta.json` endpoint.
- **CocoaPods / Hackage**: availability is based on the public package page,
  not registry metadata. Use the source link for details.
- **Maven**: a bare name has no group, so the artifact ID is searched across
  all groups. A hit means the artifact name is already in use.
- **Packagist**: a bare name is interpreted as `vendor/vendor`. Pass
  `vendor/package` for an exact Composer package.
- **Chocolatey**: the feed requires `Accept: application/atom+xml`; the checker
  sends it and parses the Atom XML.
- **Go modules**: not checked as a bare name because a module is a full path.
  Use the GitHub handle target instead.

## Cache and refresh

- Default cache: `$XDG_CACHE_HOME/nameary/cache.db` (or the platform equivalent).
- `--cache-mode off` disables reads and writes.
- `--cache-mode refresh` bypasses reads but still writes fresh results.
- `--cache-ttl` overrides the per-target TTL.

## Terms of service

Respect the source's terms and robots/rate-limit guidance. `nameary` is a
low-volume interactive tool, not a bulk scanner. If you need high-volume
checking, use official APIs and your own credentials, and cache aggressively.
