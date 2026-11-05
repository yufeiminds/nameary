# Governance

## Mission

`nameary` is a small, security-conscious CLI for checking whether a name is
available across domains, package registries, and handles. The project values
correctness, honest uncertainty, accessibility, and a calm terminal experience.

## Roles

### Users

Anyone who runs or evaluates the tool. Users can report bugs, request features,
and participate in discussions under the Code of Conduct.

### Contributors

Anyone who contributes code, docs, tests, translations, design, or issue
triage. Contributors do not need commit access.

### Maintainers

Maintainers review and merge changes, manage releases, respond to security
reports, and set technical direction. The current list is in
[`MAINTAINERS.md`](MAINTAINERS.md).

### Security responders

Maintainers act as security responders. Reports are handled privately through
GitHub Security Advisories as described in [`SECURITY.md`](SECURITY.md).

## Decision making

- Routine changes use **lazy consensus**: a maintainer reviews and approves a
  pull request; no formal vote is needed.
- Changes to the public CLI contract, JSON schema, supported platforms, release
  process, or security model require an issue or design note and explicit
  maintainer agreement.
- Security fixes may be prepared privately and released with minimal
  disclosure to protect users.
- If consensus cannot be reached, the maintainers decide by majority. If there
  is one maintainer, that maintainer decides and documents the rationale.

## Becoming a maintainer

A contributor may be invited after sustained, high-quality contributions and
demonstrated care for security, tests, and review. The current maintainers must
agree. New maintainers get repository triage/merge access appropriate to their
role and are added to `MAINTAINERS.md` and `CODEOWNERS`.

## Releases

- Releases use semantic versioning.
- Tags are `vMAJOR.MINOR.PATCH`.
- A release requires green CI, a completed
  [`release-checklist.md`](docs/release-checklist.md), and a signed or
  checksum-verified build artifact where available.
- Release notes live in the GitHub Release and `CHANGELOG.md`.

## Changes to governance

Governance changes are proposed as a pull request to this file and require
maintainer approval. The rationale is recorded in the pull request.

## Code of conduct

All participation is governed by [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).
