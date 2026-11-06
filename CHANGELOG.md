# Changelog

## v0.1.1

Distribution and CLI polish.

- Homebrew and mise are the primary installation paths.
- The release workflow renders `Formula/nameary.rb` from the release
  `checksums.txt` and publishes it to `yufeiminds/homebrew-tap` automatically.
  An empty tap repository is bootstrapped on the first release.
- Homebrew formula template is validated in CI and supports macOS/Linux on
  amd64 and arm64; a post-release macOS job verifies the tap install.
- Added `nameary --version`.
- Added `nameary cache path` and `nameary cache clear`.
- Added distinct `timeout` and `rate_limit` result statuses; they are no
  longer collapsed into `unknown`.
- Default preset is now `all`; default request timeout is `3s`. Use
  `--preset indie` for the previous fast subset.
- Release checks run the race detector and the mock-registry e2e suite.

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
