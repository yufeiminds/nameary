#!/usr/bin/env python3
"""Render the Homebrew formula from a GoReleaser checksums.txt file."""

from __future__ import annotations

import argparse
import pathlib
import sys

ASSETS = {
    "SHA_DARWIN_ARM64": "darwin_arm64",
    "SHA_DARWIN_AMD64": "darwin_amd64",
    "SHA_LINUX_ARM64": "linux_arm64",
    "SHA_LINUX_AMD64": "linux_amd64",
}


def read_checksums(path: pathlib.Path) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        parts = line.split()
        if len(parts) >= 2:
            out[parts[1].lstrip("*")] = parts[0]
    return out


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True, help="version without the v prefix")
    parser.add_argument("--checksums", required=True, type=pathlib.Path)
    parser.add_argument("--template", required=True, type=pathlib.Path)
    parser.add_argument("--output", required=True, type=pathlib.Path)
    parser.add_argument(
        "--base-url",
        default="",
        help="release asset base URL (default: GitHub release for this version)",
    )
    args = parser.parse_args()

    version = args.version.removeprefix("v")
    base = args.base_url.strip().rstrip("/")
    if not base:
        base = f"https://github.com/yufeiminds/nameary/releases/download/v{version}"

    sums = read_checksums(args.checksums)
    values = {"VERSION": version, "BASE_URL": base}
    missing: list[str] = []
    for placeholder, platform in ASSETS.items():
        asset = f"nameary_{version}_{platform}.tar.gz"
        digest = sums.get(asset)
        if not digest:
            missing.append(asset)
        else:
            values[placeholder] = digest
    if missing:
        print("missing checksums for: " + ", ".join(missing), file=sys.stderr)
        return 1

    text = args.template.read_text(encoding="utf-8")
    for key, value in values.items():
        text = text.replace("{{" + key + "}}", value)
    if "{{" in text:
        print("unresolved placeholders remain in the formula", file=sys.stderr)
        return 1

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
