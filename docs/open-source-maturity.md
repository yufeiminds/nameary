# Open-source maturity

This page maps the project's practices to OpenSSF expectations (Best Practices
Badge and Scorecard). It is a self-assessment, not a claim that a badge has been
awarded.

## Baseline project hygiene

| Expectation | Evidence |
|---|---|
| Public description and website | `README.md` |
| OSI-approved license | `LICENSE` (Apache-2.0) |
| Change history | `CHANGELOG.md`, git history |
| Roadmap | `ROADMAP.md` |
| Contribution guide | `CONTRIBUTING.md` |
| Code of conduct | `CODE_OF_CONDUCT.md` |
| Governance and maintainers | `GOVERNANCE.md`, `MAINTAINERS.md` |
| Support policy | `SUPPORT.md` |
| Security policy | `SECURITY.md` |
| Release documentation | `docs/release.md`, `docs/release-checklist.md` |

## Secure development

| Expectation | Evidence |
|---|---|
| Threat model | `docs/threat-model.md` |
| Security audit | `docs/security-audit.md` |
| Assurance case | `docs/assurance-case.md` |
| Secure coding guidance | `CONTRIBUTING.md` |
| Dependency vulnerability scan | `govulncheck` in CI |
| SAST | `gosec`, CodeQL |
| Dependency review | `dependency-review` workflow |
| Automated updates | `.github/dependabot.yml` |
| Scorecard | `.github/workflows/scorecard.yml` |
| Private vulnerability reporting | GitHub Security Advisories via `SECURITY.md` |
| Terminal injection defense | `internal/textutil`, renderer tests |
| Safe URL/file handling | `safeExternalURL`, `os.CreateTemp`, temp-file config writes |
| Least-privilege file modes | `0700` dirs, `0600` files |

## Testing and release

| Expectation | Evidence |
|---|---|
| Automated tests | `go test ./...` (61 tests) |
| Race detector | `go test -race ./...` |
| Static checks | `go vet`, `staticcheck`, `gosec` |
| Live integration smoke | `scripts/smoke.sh`, `smoke.yml` |
| Cross-platform build | macOS, Linux, Windows amd64/arm64 via goreleaser |
| Checksums | goreleaser `checksums.txt` |
| Release automation | `release.yml` on `v*` tags |
| Demo assets | Git LFS-tracked GIF/PNGs |
| Review ownership | `CODEOWNERS` |
| Issue/PR templates | `.github/ISSUE_TEMPLATE/`, `.github/PULL_REQUEST_TEMPLATE.md` |

## OpenSSF Scorecard checks

The Scorecard workflow publishes SARIF to GitHub code scanning. It covers:
binary artifacts, branch protection, dangerous workflow patterns, dependency
update tooling, fuzzing, license, maintained status, packaging, pinned
dependencies, security policy, signed releases and token permissions.

The repository must be public and the workflow must have run at least once for
Scorecard results to appear.

## Known gaps

These are deliberately listed rather than overclaimed:

- No OpenSSF Best Practices Badge ID has been registered yet.
- Release artifacts are checksummed but not signed/notarized.
- Builds are not yet bit-for-bit reproducible.
- No SBOM is published yet.
- One maintainer means review independence is limited.
- Branch protection, required checks and maintainer 2FA must be enabled in GitHub settings.
- The live smoke matrix has not yet completed its first GitHub Actions run.

## Next steps

1. Enable branch protection on `main`: require CI, CodeQL, dependency review and
   a pull request review.
2. Register the project for the OpenSSF Best Practices Badge and link the badge.
3. Add release signing/notarization and an SBOM.
4. Add a second maintainer/reviewer.
5. Run the live smoke matrix on all three platforms and record the result.
