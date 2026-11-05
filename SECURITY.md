# Security policy

## Reporting a vulnerability

Please report security issues privately through GitHub's **Security → Report a
vulnerability** flow for this repository. Do not open a public issue for a
suspected vulnerability.

Include:

- affected version or commit
- a minimal reproduction
- the impact and any known workaround
- whether the issue is already public

Do not include real GitHub tokens, private keys, or other secrets in the report.
If you must share a token, revoke it first and describe the format instead.

## Response process

1. A maintainer acknowledges the report privately.
2. The issue is reproduced and triaged; severity and affected versions are
   documented.
3. A fix is prepared privately when practical, with tests and a regression
   check.
4. A patch release is published, followed by a coordinated advisory if users
   need to take action.
5. Credit is given unless the reporter asks to remain anonymous.

This is a volunteer project without a guaranteed SLA. We aim to acknowledge
valid reports promptly and to avoid public disclosure before a fix or mitigation
is available.

## Scope

In scope:

- terminal escape injection through names, metadata or errors
- unsafe URL/protocol handling or browser launch
- SSRF, redirect downgrade, or TLS verification bugs in our HTTP layer
- path traversal, symlink/clobber, or permission bugs in config/cache/export
- secret leakage through logs, errors, or release artifacts
- dependency or workflow compromise affecting released binaries
- incorrect `available` claims caused by our error mapping

Out of scope:

- factual errors or outages in third-party registries, RDAP, DNS or WHOIS
- legal, trademark, pricing, or registrability questions
- a local attacker with the same OS user privileges
- vulnerabilities in the user's browser or terminal after opening a source URL
- social engineering or physical access

## Supported versions

| Version | Supported |
|---|---|
| `0.1.x` | yes |
| older | no |

Security fixes target the latest release. Users should upgrade to the newest
patch release.

## Security model

- `nameary` performs read-only HTTP, DNS and WHOIS queries against public
  registries and sends no telemetry.
- It does not execute downloaded content, install packages, or run shell
  commands. The TUI can open a validated `http(s)` source URL in the user's
  browser.
- The release build is CGO-free and uses Go's standard-library TLS
  verification. Redirects are capped and must remain HTTPS.
- User-controlled and registry-provided text is stripped of terminal control
  characters before it is rendered.
- Cache, config, debug logs and exports use user-only permissions and
  symlink-safe creation.
- `GITHUB_TOKEN`/`GH_TOKEN` is used only in the GitHub `Authorization` header
  and is never logged.

See [`docs/threat-model.md`](docs/threat-model.md),
[`docs/security-audit.md`](docs/security-audit.md) and
[`docs/assurance-case.md`](docs/assurance-case.md) for more detail.

## Safe harbor

We will not pursue legal action against researchers who make a good-faith
effort to follow this policy, avoid privacy violations and service disruption,
and give us reasonable time to fix the issue before disclosure.
