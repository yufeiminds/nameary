# Testing layers

nameary has three deliberately different test layers. They are complementary;
none of them replaces the others.

| Layer | Command | Network | What it proves |
|---|---|---|---|
| Unit / integration | `go test -race ./internal/...` | none | Adapter schemas, engine retries, cache, renderers and TUI with mocked HTTP/servers. |
| End-to-end | `go test -count=1 ./e2e` | none | The compiled CLI, every adapter, all output formats, exit codes and cache against a deterministic mock registry. |
| Live smoke | `bash scripts/smoke.sh` | real registries | The real endpoints still answer with the expected schemas, statuses and redirects. |

## End-to-end suite

`e2e/e2e_test.go` builds the real `cmd/nameary` binary with `-tags e2e`. That
tag enables `NAMEARY_TEST_BASE_URL`, which points every adapter at a local
`httptest` registry. The suite runs:

- every adapter in both `taken` and `available` mode;
- the full `--preset all` fan-out (25 jobs over 20 targets);
- NDJSON, grouped console and JSON rendering;
- exit codes (`0`, `1`, `2`, `3`);
- rate-limit handling;
- a cache round trip across two separate binary executions.

Nothing in the production binary changes: the base-URL hook only exists in
builds compiled with `-tags e2e`.

```bash
go test -count=1 ./e2e -v
```

## Live smoke matrix

`scripts/smoke.sh` checks one known-taken name per target against the real
internet, then fans a random name out to every target and verifies JSON and
console output. `.github/workflows/smoke.yml` runs it weekly and on demand on
Linux, macOS and Windows.

It is the only layer that can catch upstream API or schema drift, changed
rate-limit policies, TLS/redirect behaviour and DNS/RDAP quirks. It is also
slower, flaky by nature and cannot force an "available" or error result.

## Is live smoke equivalent to e2e?

No. The live smoke matrix covers **reachability and schema drift**, while the
e2e suite covers **deterministic CLI behaviour and every adapter branch**.

- Live smoke has one known-taken case per target and no deterministic
  available/429/cache cases. It cannot exercise `--fail-if-taken`, error exit
  codes or cache hits.
- E2E uses a mock registry, so it cannot detect a removed field, a changed
  endpoint, a new auth requirement or a real rate limit.
- Live smoke is validated on three operating systems; e2e in CI runs on Linux
  (the `act` container is Linux-only too).

Run both before a release. Keep the live smoke suite scheduled so drift is
found even when the code has not changed.

## Local CI with act

The repository ships `.actrc`, which maps `ubuntu-latest` to the small
`catthehacker/ubuntu:act-latest` image used by act:

```bash
act -W .github/workflows/ci.yml
```

Act runs the Linux CI job (format, vet, govulncheck, race tests, e2e and
build). It cannot emulate the macOS or Windows legs of the live smoke matrix;
run those on GitHub Actions or directly with `bash scripts/smoke.sh`.
