# Contributing to nameary

Thanks for helping make name discovery faster and more pleasant. This project
follows the [OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/)
spirit: small changes, tests, clear reviews, and a documented security process.

## Before you start

- Read [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) and [`GOVERNANCE.md`](GOVERNANCE.md).
- For a security issue, **do not open a public issue**. Follow
  [`SECURITY.md`](SECURITY.md) instead.
- For usage questions, see [`SUPPORT.md`](SUPPORT.md).
- Check existing issues and pull requests before starting large work. For a
  behavioral or API change, open an issue first so we can agree on the shape.

## Requirements

- **Go 1.26.10** or newer (Charm v2 requires Go 1.26+; releases are built with
  the patched 1.26.10 toolchain).
- `git-lfs` only if you need the demo GIF/screenshots. The binary and tests do
  not require LFS assets.
- `golangci-lint`, `gosec`, `govulncheck` and `staticcheck` are recommended for
  security-sensitive changes. CI runs `go vet`, tests, CodeQL, Scorecard,
  dependency review and `govulncheck`.

## Local setup

```bash
git clone https://github.com/yufeiminds/nameary
cd nameary
git lfs install        # optional: demo assets
git lfs pull           # optional
go build -o nameary ./cmd/nameary
```

## Build, test, and checks

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...

# optional but recommended
gosec ./...
govulncheck ./...
staticcheck ./...
```

Live network smoke tests are separate because they call public registries:

```bash
./scripts/smoke.sh
```

Do not make pull-request CI depend on live registries. The scheduled/manual
`Live smoke` workflow covers that.

## Code guidelines

- Keep the engine independent of the TUI and renderers.
- Every checker implements `check.Checker` and declares its own policy
  (priority, concurrency, timeout, cache TTL).
- User-controlled and registry-provided strings must pass through
  `textutil.Clean` before terminal rendering.
- Use `url.PathEscape`/`url.QueryEscape` for names in URLs. Never concatenate
  raw input into a URL or command.
- Do not open arbitrary URL schemes. The TUI only opens absolute `http(s)` URLs.
- Back off on 429/5xx; never turn a timeout or rate limit into `available`.
- Add tests for new adapters with `httptest` fixtures. Do not use real network
  calls in unit tests.
- Keep JSON field names stable. Add fields, do not rename or remove them without
  a major-version discussion.

## Tests

- Adapter tests belong next to the adapter and should cover 200, 404, 429,
  malformed bodies and timeouts.
- Engine tests should cover retry, cancellation, cache and event ordering.
- TUI/render tests should cover width, `--ascii`, `--style plain`, and
  terminal-escape sanitization.
- Run `go test -race ./...` before opening a pull request.

## Pull requests

1. Fork and create a topic branch from `main`.
2. Keep the change focused; split unrelated refactors.
3. Include tests and documentation for behavior changes.
4. Run the checks above.
5. Open a PR using the template. Describe the user-visible change and any
   security or compatibility impact.

Maintainers review for correctness, security, accessibility, and consistency.
A PR needs at least one maintainer approval before merge. We do not require a
CLA; contributions are accepted under the project's Apache-2.0 license.

## Commit messages

Use a short imperative subject, for example:

```text
check: handle conda search truncation
tui: keep footer anchored
docs: document source rate limits
```

Reference issues with `Fixes #123` when applicable.

## Security-sensitive changes

Changes touching HTTP, URL handling, subprocesses, file paths, permissions,
cache/config parsing, or release workflows need an explicit security note in
the PR. At minimum:

- What input is untrusted?
- What is the trust boundary?
- Which tests or scans cover it?
- Does it change redirects, URL schemes, file modes, or logging?

## Release process

Releases are cut from `v*` tags with goreleaser. See
[`docs/release.md`](docs/release.md) and
[`docs/release-checklist.md`](docs/release-checklist.md). Do not push a tag
without green CI and a completed checklist.

## Recognition

Contributors are credited in release notes and the changelog. Thank you for
helping.
