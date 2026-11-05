#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PY=python3
command -v python3 >/dev/null 2>&1 || PY=python

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

BIN="$TMP/nameary"
case "$(go env GOOS)" in
  windows*) BIN="$TMP/nameary.exe" ;;
esac
go build -o "$BIN" ./cmd/nameary

status_of() {
  "$PY" -c 'import json,sys; rows=[json.loads(l) for l in sys.stdin if l.strip()]; print(rows[0]["status"] if rows else "none")'
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# Known-taken cases across every target. This catches endpoint/schema drift.
cases=(
  "domains|google.com"
  "npm|react"
  "pypi|requests"
  "crates|serde"
  "rubygems|rails"
  "nuget|newtonsoft.json"
  "dockerhub|nginx"
  "homebrew|wget"
  "packagist|laravel/framework"
  "hex|phoenix"
  "pub|http"
  "github|torvalds"
  "maven|guava"
  "conda|numpy"
  "cocoapods|AFNetworking"
  "aur|yay"
  "snap|hello"
  "chocolatey|git"
  "hackage|aeson"
  "jsr|@std/async"
)

for entry in "${cases[@]}"; do
  target="${entry%%|*}"
  name="${entry#*|}"
  out="$("$BIN" check "$name" --targets "$target" --format ndjson --cache-mode off --timeout 25s 2>"$TMP/stderr")" || {
    cat "$TMP/stderr" >&2
    fail "$target/$name command failed"
  }
  status="$(printf '%s\n' "$out" | status_of)"
  if [ "$status" != "taken" ]; then
    printf '%s\n' "$out" >&2
    fail "$target/$name: expected taken, got $status"
  fi
  printf 'ok  %-13s %s\n' "$target" "$name"
done

# A random name must fan out to every target without engine errors.
random="zzqxjvsmoke$(date +%s)"
out="$("$BIN" check "$random" --preset all --format ndjson --cache-mode off --timeout 25s 2>"$TMP/stderr")" || {
  cat "$TMP/stderr" >&2
  fail "random-name fan-out failed"
}
printf '%s\n' "$out" | "$PY" -c '
import json, sys
rows = [json.loads(l) for l in sys.stdin if l.strip()]
assert len(rows) == 25, f"expected 25 rows, got {len(rows)}"
bad = [r for r in rows if r["status"] in ("error", "skipped")]
assert not bad, f"engine errors: {bad}"
print("ok  fan-out      %d targets, %d available, %d taken, %d unknown" % (
    len(rows),
    sum(r["status"] == "available" for r in rows),
    sum(r["status"] == "taken" for r in rows),
    sum(r["status"] == "unknown" for r in rows),
))
'

# JSON output must be a valid array.
"$BIN" check "$random" --targets npm --format json --cache-mode off 2>/dev/null |
  "$PY" -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d,list) and len(d)==1; print("ok  json         valid array")'

# Grouped console output must keep the section headings.
console="$("$BIN" check "$random" --targets domains,npm --tlds com --format console --style plain --cache-mode off 2>/dev/null)"
printf '%s\n' "$console" | grep -q "DOMAINS" || fail "console output missing DOMAINS"
printf '%s\n' "$console" | grep -q "PACKAGES" || fail "console output missing PACKAGES"
printf 'ok  console      grouped output\n'

echo "smoke: PASS"
