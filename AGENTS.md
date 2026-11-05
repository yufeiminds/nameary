# AGENTS.md

Instructions for AI coding agents working on **nameary**.

## Git: commit locally, never push

- AI agents may create **local commits only**.
- **Never** run `git push`, `git push --tags`, `gh pr create`, `gh release
  create`, or any other command that writes to a remote.
- Pushing is always a human decision. When work is ready, leave a clean local
  commit and summarize exactly what should be pushed.
- Do not push, move, or delete published tags. A local tag move must be called
  out explicitly in the handoff.
- Do not add remotes or rewrite published history.

## Before handoff

- Remove intermediate and temporary documents before the commit that is meant
  for release. This includes design scratch files and `docs/plan.md`; keep only
  durable user-facing documentation.
- Keep the working tree clean and never commit `dist/`, built binaries, caches,
  exported `nameary-*.json` / `nameary-*.md` files, or editor state.
- Update `README.md`, `CHANGELOG.md`, or `docs/` when behavior, flags, output or
  installation changes.
- Run the relevant quality gates:
  - `gofmt -l .`
  - `go vet ./...`
  - `go test -race ./internal/...`
  - `go test -count=1 ./e2e`
  - `act -W .github/workflows/ci.yml -j test --pull=false` for workflow changes
  - `goreleaser check` for release or packaging changes
- Never make pull-request CI depend on live registries. Use mocks and fixtures;
  live checks belong in `scripts/smoke.sh`.

## Demo recording

- VHS recordings must use `--preset all`:
  `./nameary acme --preset all --cache-mode off`.
- Re-record with `vhs demo/demo.tape` and commit the LFS-tracked GIF.
- Do not commit a recording that was made with the default preset.

## README and presentation

- Keep the project title and badges centered together at the top of
  `README.md`.
- Badges should reflect real, verifiable project facts (CI, Go version,
  license, supported platforms, release).
- Keep command examples copy-pasteable and consistent with the CLI.

## Security and releases

- Preserve terminal-text sanitization, validated external URLs, HTTPS-only
  redirects, user-only file permissions and bounded input sizes.
- Never add secrets, tokens or real credentials to fixtures or logs.
- Release tags, GitHub Releases and repository visibility changes are human
  decisions.
