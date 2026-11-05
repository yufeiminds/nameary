# Roadmap

This is a direction document, not a promise. Priorities can change when a
security issue or a registry API change requires attention.

## v0.1 — current

- 20 availability sources: domains, package registries, handles.
- Interactive TUI by default; grouped progressive `console` output.
- Async engine, per-target limits, retries, bbolt cache.
- DNS + RDAP + WHOIS domain checks with confidence levels.
- JSON/NDJSON/table output, presets, config, shell completion.
- Security hardening, live smoke script, release workflow and LFS demo assets.

## v0.2 — next

- More registry adapters where an official, documented API exists.
- Richer detail panes and comparison filters.
- Better rate-limit telemetry and per-source status in the UI.
- Cache inspection/pruning commands and config migration.
- Homebrew tap, asdf plugin, mise installation, Scoop/winget, signed
  artifacts and SBOM.

## v1.0 — stability

- Freeze the JSON schema and CLI flags with a documented compatibility policy.
- Reproducible builds and signed/notarized release artifacts.
- Full accessibility pass for `--style plain`, narrow terminals and screen
  readers.
- Documented threat model and external security review.
- Stable documentation site with examples and source policy pages.

## Non-goals

- Guaranteeing that a domain is registrable, affordable, or free of trademark
  conflicts.
- High-volume scraping or bulk surveillance of registries.
- Replacing legal, trademark, or registrar due diligence.
- Installing packages or executing downloaded code.

## How priorities are set

1. Security and correctness.
2. Registry/API breakage that causes wrong results.
3. Accessibility and terminal compatibility.
4. User-requested sources and workflow improvements.
5. Packaging and distribution.

See [`GOVERNANCE.md`](GOVERNANCE.md) for decision making.
