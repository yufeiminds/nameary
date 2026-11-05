# Threat model

## Assets

- The user's terminal and shell session.
- Local config, cache, debug log and exported reports.
- GitHub tokens and any future credentials.
- The correctness of availability results.
- Release artifacts and the tag/release pipeline.
- User privacy: the names checked and the source URLs fetched.

## Trust boundaries

```
user input ──► CLI/TUI ──► engine ──► HTTP / DNS / WHOIS ──► untrusted data
                    │           │
                    │           └──► bbolt cache ──► local files
                    └──► terminal renderers ──► terminal
                                      └──► browser (validated URL)
```

The registries, RDAP servers, WHOIS servers and DNS answers are **untrusted
data**. The local machine and the user's browser are outside the tool's trust
boundary once a URL is handed off.

## Threats and mitigations

| Threat | Mitigation |
|---|---|
| Terminal escape/OSC/bidi injection from names or registry metadata | `textutil.Clean` at every terminal rendering boundary; tests cover TUI, table and console |
| Malicious `file://`/`javascript:` source links | Only absolute `http(s)` URLs with a host are opened; `exec.Command` is used without a shell |
| Redirect to a downgraded or unexpected scheme | At most five redirects, HTTPS-only redirects |
| Oversized or malicious registry response | 4 MB body cap; JSON/XML parsers; conservative `unknown` on decode errors |
| Cache poisoning through malicious strings | Cached facts are display-only and sanitized; URL opening validates schemes |
| Symlink/clobber on export | `os.CreateTemp` unique names and `0600` files |
| Write-through-symlink on config | Temp file plus rename in the config directory; `0600` |
| World-readable local state | Cache/config/debug dirs `0700`; files `0600` |
| Secret leakage through logs | Tokens are never logged; query strings are sanitized; debug log is user-only |
| Command injection | No shell invocation; browser opener only accepts validated http(s) URLs |
| Path traversal | Export names are restricted to a safe character set and generated in the CWD |
| Resource exhaustion | Caps on name length, TLD count, concurrency and timeout; bounded response bodies |
| Dependency compromise | Pinned modules, Dependabot, dependency review, CodeQL, govulncheck |
| Wrong “available” result | 404-only availability; timeouts/429/5xx become `unknown`; confidence levels |
| GitHub quota exhaustion | Token guidance, small concurrency, 12h cache, HTML fallback |
| WHOIS/DNS privacy | Only public registry endpoints; no telemetry; cache is local |
| Release tampering | Checksums, goreleaser, CI tests, Scorecard; signing is a future hardening item |

## Out of scope

- A local attacker with the same OS user privileges.
- A compromised browser or remote website after the user opens a source URL.
- Registry/RDAP/WHOIS operators returning false data.
- Legal, trademark, pricing, or registrability determinations.
- Nation-state interference with DNS or TLS.

## Review cadence

Review this model before each minor release, after adding a new network source,
or when changing URL/file handling, release automation, or authentication. The
current audit is in [`security-audit.md`](security-audit.md).
