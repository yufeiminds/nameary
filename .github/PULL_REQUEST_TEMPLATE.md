## Summary

<!-- What changed and why? -->

## Type

- [ ] Bug fix
- [ ] New check source
- [ ] TUI / console / accessibility
- [ ] Documentation
- [ ] Security hardening
- [ ] Packaging / release
- [ ] Refactor (no behavior change)

## Testing

- [ ] `gofmt -l .`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `./scripts/smoke.sh` (if network behavior changed)
- [ ] Manual TUI/console check at narrow and wide sizes

## Security

- [ ] This change does not add untrusted input to terminal output without
      `textutil.Clean`.
- [ ] New URLs are constructed with escaping and remain `http(s)`.
- [ ] No tokens, secrets, or private names are logged or committed.
- [ ] File paths/permissions remain user-only and symlink-safe.
- [ ] Security impact: <!-- none / describe -->

## Docs

- [ ] README or `docs/` updated for user-visible changes.
- [ ] JSON schema/exit codes unchanged, or documented as a breaking change.

## Related issues

Fixes #
