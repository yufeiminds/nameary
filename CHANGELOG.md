# Changelog

## v0.1.0

Initial release.

- Interactive full-screen TUI as the default on a TTY.
- Grouped, progressive `--format console` output with a live status line.
- `table`, `json` and `ndjson` output modes.
- 20 check targets: domains, npm, PyPI, crates.io, RubyGems, NuGet, Docker Hub,
  Homebrew, Packagist, Hex, pub.dev, GitHub, Maven Central, Conda, CocoaPods,
  AUR, Snap Store, Chocolatey, Hackage and JSR.
- DNS + RDAP + WHOIS domain checks with confidence levels.
- Per-target concurrency, timeouts, retries and bbolt cache.
- `--style`, `--cache-mode`, presets and TOML config.
- Shell completion, debug logging and release packaging.
- Security hardening: Go 1.26.10, terminal-escape sanitization, safe external
  URLs, user-only file permissions, bounded inputs and HTTPS-only redirects.
