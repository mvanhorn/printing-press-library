#!/usr/bin/env python3
"""Publish checksums.txt on <slug>-current releases from GitHub's asset digests.

Digests come from the release API rather than local files, so the list covers
every asset actually on the release, including ones a partial rebuild left
behind, and the backfill needs no downloads.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
import time
from pathlib import Path


CHECKSUMS_NAME = "checksums.txt"
SHA256_DIGEST_RE = re.compile(r"^sha256:([0-9a-f]{64})$")


def checksums_text(assets: list[dict]) -> str:
    lines = []
    for asset in sorted(assets, key=lambda a: a["name"]):
        name = asset["name"]
        if name == CHECKSUMS_NAME:
            continue
        match = SHA256_DIGEST_RE.match(asset.get("digest") or "")
        if not match:
            raise ValueError(f"asset {name} has no sha256 digest: {asset.get('digest')!r}")
        lines.append(f"{match.group(1)}  {name}\n")
    if not lines:
        raise ValueError("release has no assets to checksum")
    return "".join(lines)


def releases_missing_checksums(releases: list[dict]) -> list[dict]:
    return [
        r
        for r in releases
        if r["tag_name"].endswith("-current")
        and not any(a["name"] == CHECKSUMS_NAME for a in r["assets"])
    ]


def gh_json(*args: str):
    out = subprocess.run(["gh", "api", *args], check=True, text=True, stdout=subprocess.PIPE).stdout
    return json.loads(out)


def fetch_assets(repo: str, tag: str) -> list[dict]:
    attempts = 5
    for attempt in range(attempts):
        assets = gh_json(f"repos/{repo}/releases/tags/{tag}")["assets"]
        if all(a.get("digest") for a in assets if a["name"] != CHECKSUMS_NAME):
            break
        if attempt < attempts - 1:
            time.sleep(2 * (attempt + 1))
    return assets


def publish(repo: str, tag: str, assets: list[dict], dry_run: bool) -> None:
    text = checksums_text(assets)
    count = len(text.splitlines())
    if dry_run:
        print(f"[dry-run] {tag}: {count} assets")
        return
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / CHECKSUMS_NAME
        path.write_text(text)
        subprocess.run(
            ["gh", "release", "upload", tag, str(path), "--clobber", "--repo", repo],
            check=True,
        )
    print(f"{tag}: published {CHECKSUMS_NAME} for {count} assets")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True)
    target = parser.add_mutually_exclusive_group(required=True)
    target.add_argument("--tag")
    target.add_argument("--all-missing", action="store_true")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    if args.tag:
        publish(args.repo, args.tag, fetch_assets(args.repo, args.tag), args.dry_run)
        return 0

    pages = gh_json("--paginate", "--slurp", f"repos/{args.repo}/releases?per_page=100")
    releases = releases_missing_checksums([r for page in pages for r in page])
    print(f"{len(releases)} release(s) missing {CHECKSUMS_NAME}")
    failed = []
    for release in releases:
        tag = release["tag_name"]
        try:
            # Re-read right before writing: a build may have published a fresh
            # checksums.txt since the listing, and clobbering it would go stale.
            assets = fetch_assets(args.repo, tag)
            if any(a["name"] == CHECKSUMS_NAME for a in assets):
                print(f"{tag}: already has {CHECKSUMS_NAME}, skipping")
                continue
            publish(args.repo, tag, assets, args.dry_run)
        except (ValueError, subprocess.CalledProcessError) as err:
            print(f"::error::{tag}: {err}", file=sys.stderr)
            failed.append(tag)
    if failed:
        print(f"{len(failed)} release(s) failed: {' '.join(failed)}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
