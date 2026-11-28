#!/usr/bin/env python3
"""Publish checksums.txt on <slug>-current releases from GitHub's asset digests.

Digests come from the release API rather than local files, so the list covers
every asset actually on the release, including ones a partial rebuild left
behind, and the backfill needs no downloads.

A build also records where its binaries came from, as two comment lines that
sha256sum and shasum skip:

    # source-commit <commit the release was built at>
    # source-tree <git tree of the CLI directory at that commit> <directory>

The registry entry carries the same tree as release.source_tree. Each CLI is its
own Go module, so an installer can trust a binary only when the two trees match.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timedelta
from pathlib import Path
from typing import NamedTuple


CHECKSUMS_NAME = "checksums.txt"
SHA256_DIGEST_RE = re.compile(r"^sha256:([0-9a-f]{64})$")
OBJECT_ID_RE = re.compile(r"^(?:[0-9a-f]{40}|[0-9a-f]{64})$")
SOURCE_DIR_RE = re.compile(r"^library/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$")
SOURCE_COMMIT_RE = re.compile(r"^# source-commit (\S+)$", re.M)
SOURCE_TREE_RE = re.compile(r"^# source-tree (\S+) (\S+)$", re.M)


class Source(NamedTuple):
    directory: str
    commit: str
    tree: str


def source_lines(source: Source) -> list[str]:
    if not OBJECT_ID_RE.match(source.commit):
        raise ValueError(f"source commit is not a full object id: {source.commit!r}")
    if not OBJECT_ID_RE.match(source.tree):
        raise ValueError(f"source tree is not a full object id: {source.tree!r}")
    if not SOURCE_DIR_RE.match(source.directory):
        raise ValueError(f"source directory is not library/<category>/<slug>: {source.directory!r}")
    return [f"# source-commit {source.commit}\n", f"# source-tree {source.tree} {source.directory}\n"]


def retained_assets(assets: list[dict], uploaded: set[str]) -> list[str]:
    """Release assets this build did not upload, left over from an earlier one."""
    return sorted(a["name"] for a in assets if a["name"] != CHECKSUMS_NAME and a["name"] not in uploaded)


# One build uploads every asset within minutes; rebuilds that left an older
# asset behind are hours or days apart.
SINGLE_UPLOAD_WINDOW = timedelta(minutes=30)


def single_upload(assets: list[dict]) -> bool:
    """Whether every asset other than checksums.txt was uploaded in one build."""
    times = [a.get("updated_at") for a in assets if a["name"] != CHECKSUMS_NAME]
    if not times or not all(times):
        return False
    parsed = [datetime.fromisoformat(t.replace("Z", "+00:00")) for t in times]
    return max(parsed) - min(parsed) <= SINGLE_UPLOAD_WINDOW


def parse_source(text: str) -> Source | None:
    commit, tree = SOURCE_COMMIT_RE.search(text), SOURCE_TREE_RE.search(text)
    if not commit or not tree:
        return None
    return Source(tree.group(2), commit.group(1), tree.group(1))


def checksums_text(assets: list[dict], source: Source | None = None) -> str:
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
    if source:
        lines = source_lines(source) + lines
    return "".join(lines)


def has_checksums(assets: list[dict]) -> bool:
    return any(a["name"] == CHECKSUMS_NAME for a in assets)


def releases_missing_checksums(releases: list[dict]) -> list[dict]:
    return [
        r
        for r in releases
        if r["tag_name"].endswith("-current") and not has_checksums(r["assets"])
    ]


def gh_json(*args: str):
    out = subprocess.run(["gh", "api", *args], check=True, text=True, stdout=subprocess.PIPE).stdout
    return json.loads(out)


def fetch_release(repo: str, tag: str) -> dict:
    attempts = 5
    for attempt in range(attempts):
        release = gh_json(f"repos/{repo}/releases/tags/{tag}")
        if all(a.get("digest") for a in release["assets"] if a["name"] != CHECKSUMS_NAME):
            break
        if attempt < attempts - 1:
            time.sleep(2 * (attempt + 1))
    return release


def fetch_assets(repo: str, tag: str) -> list[dict]:
    return fetch_release(repo, tag)["assets"]


def published_checksums(repo: str, tag: str) -> str:
    return subprocess.run(
        ["gh", "release", "download", tag, "--repo", repo, "--pattern", CHECKSUMS_NAME, "--output", "-"],
        check=True, text=True, stdout=subprocess.PIPE,
    ).stdout


def tree_at(commit: str, directory: str) -> str | None:
    out = subprocess.run(
        ["git", "rev-parse", "--verify", "--quiet", f"{commit}:{directory}"],
        text=True, stdout=subprocess.PIPE,
    )
    return out.stdout.strip() if out.returncode == 0 else None


def publish(repo: str, tag: str, assets: list[dict], dry_run: bool, clobber: bool, source: Source | None = None) -> None:
    text = checksums_text(assets, source)
    count = len(text.splitlines())
    count -= 2 if source else 0
    if dry_run:
        print(f"[dry-run] {tag}: {count} assets" + (f", tree {source.tree}" if source else ""))
        return
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / CHECKSUMS_NAME
        path.write_text(text)
        cmd = ["gh", "release", "upload", tag, str(path), "--repo", repo]
        if clobber:
            cmd.append("--clobber")
        subprocess.run(cmd, check=True)
    print(f"{tag}: published {CHECKSUMS_NAME} for {count} assets")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True)
    target = parser.add_mutually_exclusive_group(required=True)
    target.add_argument("--tag")
    target.add_argument("--all-missing", action="store_true")
    target.add_argument(
        "--all-missing-source", action="store_true",
        help="add source lines to checksums.txt files that lack them; run from a full clone",
    )
    parser.add_argument("--source-dir", help="CLI directory, library/<category>/<slug> (with --tag)")
    parser.add_argument("--source-commit", help="commit the release was built at (with --tag)")
    parser.add_argument("--source-tree", help="git tree of --source-dir at --source-commit (with --tag)")
    parser.add_argument("--built-dir", help="directory of the files this build uploaded (required with the source flags)")
    parser.add_argument("--registry", default="registry.json", help="slug to directory map (with --all-missing-source)")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    source_args = (args.source_dir, args.source_commit, args.source_tree, args.built_dir)
    if any(source_args) and (not all(source_args) or not args.tag):
        parser.error("--source-dir, --source-commit, --source-tree and --built-dir go together, with --tag")

    if args.tag:
        assets = fetch_assets(args.repo, args.tag)
        source = Source(*source_args[:3]) if all(source_args) else None
        if source:
            # A failed platform leaves the previous build's asset on the release.
            # The source lines would then vouch for a binary this tree did not build.
            retained = retained_assets(assets, {p.name for p in Path(args.built_dir).iterdir() if p.is_file()})
            if retained:
                print(f"::warning::{args.tag}: not recording source; assets from an earlier build remain: {' '.join(retained)}")
                source = None
        publish(args.repo, args.tag, assets, args.dry_run, clobber=True, source=source)
        return 0

    if args.all_missing_source:
        return backfill_source(args.repo, args.registry, args.dry_run)

    pages = gh_json("--paginate", "--slurp", f"repos/{args.repo}/releases?per_page=100")
    releases = releases_missing_checksums([r for page in pages for r in page])
    print(f"{len(releases)} release(s) missing {CHECKSUMS_NAME}")
    failed = []
    for release in releases:
        tag = release["tag_name"]
        try:
            # A build may publish a fresher list at any point, so the backfill
            # never overwrites: re-read first, and upload without --clobber.
            assets = fetch_assets(args.repo, tag)
            if has_checksums(assets):
                print(f"{tag}: already has {CHECKSUMS_NAME}, skipping")
                continue
            try:
                publish(args.repo, tag, assets, args.dry_run, clobber=False)
            except subprocess.CalledProcessError:
                if not has_checksums(fetch_assets(args.repo, tag)):
                    raise
                print(f"{tag}: a build published {CHECKSUMS_NAME} first, skipping")
        except (ValueError, subprocess.CalledProcessError) as err:
            print(f"::error::{tag}: {err}", file=sys.stderr)
            failed.append(tag)
    if failed:
        print(f"{len(failed)} release(s) failed: {' '.join(failed)}", file=sys.stderr)
        return 1
    return 0


def backfill_source(repo: str, registry_path: str, dry_run: bool) -> int:
    """Add source lines to every -current checksums.txt that lacks them.

    The source commit is the release's target, which build-mcpb.yml sets to the
    commit it built. The directory comes from registry.json, and the tree is
    read from this clone, so a CLI moved since its last build is skipped rather
    than guessed. A release whose assets were not uploaded together may hold an
    earlier build's binary, so it is skipped too.

    This writes checksums.txt outside the release job's concurrency group, so
    run it only while no build is queued or running.

    Accepted gap: release metadata cannot prove which build produced each asset.
    A publish canceled between its upload and its release edit, or a partial
    rebuild landing within the window of a full one, would be labeled with the
    wrong tree. Builds record the source themselves; this backfill exists to
    cover releases published before they did.
    """
    directories = {e["name"]: e["path"] for e in json.loads(Path(registry_path).read_text())["entries"]}
    pages = gh_json("--paginate", "--slurp", f"repos/{repo}/releases?per_page=100")
    releases = [r for page in pages for r in page if r["tag_name"].endswith("-current") and has_checksums(r["assets"])]
    print(f"{len(releases)} release(s) with {CHECKSUMS_NAME}")
    failed, skipped = [], []
    for listed in releases:
        tag = listed["tag_name"]
        try:
            if parse_source(published_checksums(repo, tag)):
                continue
            directory = directories.get(tag.removesuffix("-current"))
            # Read the release fresh so its target and digests describe one build.
            release = fetch_release(repo, tag)
            commit = release["target_commitish"]
            if not single_upload(release["assets"]):
                print(f"::warning::{tag}: assets were not uploaded together, skipping")
                skipped.append(tag)
                continue
            tree = tree_at(commit, directory) if directory and OBJECT_ID_RE.match(commit) else None
            if not tree:
                print(f"::warning::{tag}: no tree for {directory!r} at {commit!r}, skipping")
                skipped.append(tag)
                continue
            publish(repo, tag, release["assets"], dry_run, clobber=True, source=Source(directory, commit, tree))
        except (ValueError, subprocess.CalledProcessError) as err:
            print(f"::error::{tag}: {err}", file=sys.stderr)
            failed.append(tag)
    if skipped:
        print(f"{len(skipped)} release(s) skipped: {' '.join(skipped)}")
    if failed:
        print(f"{len(failed)} release(s) failed: {' '.join(failed)}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
