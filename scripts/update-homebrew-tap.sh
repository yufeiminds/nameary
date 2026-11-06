#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-${GITHUB_REF_NAME:-}}"
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
  echo "VERSION or GITHUB_REF_NAME must be set" >&2
  exit 1
fi

CHECKSUMS="${CHECKSUMS:-dist/checksums.txt}"
TAP_REPO_URL="${HOMEBREW_TAP_REPO_URL:-https://github.com/yufeiminds/homebrew-tap.git}"
TAP_BRANCH="${HOMEBREW_TAP_BRANCH:-main}"
TAP_TOKEN="${HOMEBREW_TAP_GITHUB_TOKEN:-}"
BASE_URL="${HOMEBREW_TAP_BASE_URL:-}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "$TAP_TOKEN" ]; then
  export GH_TOKEN="$TAP_TOKEN"
  gh auth setup-git
fi

if git ls-remote --exit-code --heads "$TAP_REPO_URL" "$TAP_BRANCH" >/dev/null 2>&1; then
  git clone --depth 1 --branch "$TAP_BRANCH" "$TAP_REPO_URL" "$TMP/tap"
else
  # Bootstrap an empty tap so the release workflow never needs a manual push.
  git init -q -b "$TAP_BRANCH" "$TMP/tap"
  git -C "$TMP/tap" remote add origin "$TAP_REPO_URL"
  cp packaging/homebrew/tap-README.md "$TMP/tap/README.md"
fi

python3 scripts/render-homebrew-formula.py \
  --version "$VERSION" \
  --checksums "$CHECKSUMS" \
  --template packaging/homebrew/nameary.rb.tmpl \
  --output "$TMP/tap/Formula/nameary.rb" \
  ${BASE_URL:+--base-url "$BASE_URL"}

git -C "$TMP/tap" config user.name "nameary release bot"
git -C "$TMP/tap" config user.email "release-bot@users.noreply.github.com"

if [ -z "$(git -C "$TMP/tap" status --porcelain -- Formula/nameary.rb)" ]; then
  echo "homebrew-tap already has nameary ${VERSION}"
  exit 0
fi

git -C "$TMP/tap" add Formula/nameary.rb
if [ -f "$TMP/tap/README.md" ]; then
  git -C "$TMP/tap" add README.md
fi
git -C "$TMP/tap" commit -m "nameary ${VERSION}"
git -C "$TMP/tap" push origin "HEAD:${TAP_BRANCH}"
echo "updated homebrew-tap to nameary ${VERSION}"
