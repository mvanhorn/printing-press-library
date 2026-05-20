#!/usr/bin/env python3
"""Guard per-CLI attribution against accidental ownership flips.

This check is intentionally PR-scoped. It compares the pull request head to
the base branch and enforces three rules:

1. Every touched CLI manifest in the PR head must include non-empty printer fields.
2. Existing CLIs may not change attribution fields in the same PR that changes
   the CLI surface. Attribution corrections are allowed, but they must be
   reviewable as attribution-only changes.
3. Newly-added CLIs must carry explicit printer attribution in
   .printing-press.json.

The check does not try to rediscover the original author from git history on
every PR. The manifests and curated sweep map are the source of truth; this
script only prevents a normal CLI improvement PR from rewriting them.
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[3]
ATTRIBUTION_FIELDS = {"printer", "printer_name"}
PLACEHOLDER_PRINTERS = {"", "USER", "user"}
SWEEP_CANONICAL_PATH = "tools/sweep-canonical/main.go"


def git(*args: str, allow_error: bool = False) -> str:
    proc = subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0 and not allow_error:
        raise RuntimeError(
            f"git {' '.join(args)} failed with exit {proc.returncode}: {proc.stderr.strip()}"
        )
    return proc.stdout


def path_exists(ref: str, path: str) -> bool:
    return (
        subprocess.run(
            ["git", "cat-file", "-e", f"{ref}:{path}"],
            cwd=REPO_ROOT,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        ).returncode
        == 0
    )


def read_file(ref: str, path: str) -> str | None:
    if not path_exists(ref, path):
        return None
    return git("show", f"{ref}:{path}")


def read_json(ref: str, path: str) -> dict[str, Any] | None:
    text = read_file(ref, path)
    if text is None:
        return None
    try:
        data = json.loads(text)
    except json.JSONDecodeError as exc:
        print(f"::error file={path}::{path} is not valid JSON at {ref}: {exc}")
        return None
    if not isinstance(data, dict):
        print(f"::error file={path}::{path} is not a JSON object at {ref}")
        return None
    return data


def changed_files(base: str, head: str) -> list[str]:
    out = git("diff", "--name-only", f"{base}...{head}")
    return [line for line in out.splitlines() if line]


def cli_root_for_path(path: str) -> tuple[str, str] | None:
    parts = path.split("/")
    if len(parts) < 4 or parts[0] != "library":
        return None
    root = "/".join(parts[:3])
    rest = "/".join(parts[3:])
    return root, rest


def cli_skill_api_for_path(path: str) -> str | None:
    match = re.fullmatch(r"cli-skills/pp-([^/]+)/SKILL\.md", path)
    return match.group(1) if match else None


def api_name_for_root(ref: str, root: str) -> str:
    manifest = read_json(ref, f"{root}/.printing-press.json") or {}
    return str(manifest.get("api_name") or Path(root).name)


def unquote_scalar(raw: str) -> str:
    raw = raw.strip()
    if len(raw) >= 2 and raw[0] == raw[-1] == '"':
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return raw[1:-1]
    if len(raw) >= 2 and raw[0] == raw[-1] == "'":
        return raw[1:-1]
    return raw


FRONTMATTER_RE = re.compile(r"\A---\n(?P<body>.*?)(?:\n---\n|\n---\Z)", re.DOTALL)
AUTHOR_LINE_RE = re.compile(r"(?m)^author:\s*(?P<value>.+?)\s*$")


def skill_author(text: str | None) -> str | None:
    if text is None:
        return None
    fm = FRONTMATTER_RE.match(text)
    if not fm:
        return None
    author = AUTHOR_LINE_RE.search(fm.group("body"))
    if not author:
        return None
    return unquote_scalar(author.group("value"))


def normalize_skill_author(text: str | None) -> str | None:
    if text is None:
        return None
    fm = FRONTMATTER_RE.match(text)
    if not fm:
        return text
    body = AUTHOR_LINE_RE.sub("author: <ATTRIBUTION>", fm.group("body"))
    return text[: fm.start("body")] + body + text[fm.end("body") :]


def manifest_without_attribution(manifest: dict[str, Any] | None) -> dict[str, Any] | None:
    if manifest is None:
        return None
    return {k: v for k, v in manifest.items() if k not in ATTRIBUTION_FIELDS}


MAP_BLOCK_RE = re.compile(
    r"var\s+cliAuthorByAPIName\s*=\s*map\[string\]string\s*\{(?P<body>.*?)\n\}",
    re.DOTALL,
)
MAP_ENTRY_RE = re.compile(r'^\s*"(?P<api>[^"]+)"\s*:\s*"(?P<author>(?:\\.|[^"])*)"', re.MULTILINE)


def parse_cli_author_map(ref: str) -> dict[str, str]:
    text = read_file(ref, SWEEP_CANONICAL_PATH)
    if text is None:
        return {}
    block = MAP_BLOCK_RE.search(text)
    if not block:
        return {}
    entries: dict[str, str] = {}
    for match in MAP_ENTRY_RE.finditer(block.group("body")):
        try:
            author = json.loads(f'"{match.group("author")}"')
        except json.JSONDecodeError:
            author = match.group("author")
        entries[match.group("api")] = author
    return entries


@dataclass
class CLIChange:
    api_name: str
    root: str
    existing: bool
    changed_files: list[str] = field(default_factory=list)
    attribution_changes: list[str] = field(default_factory=list)
    surface_changes: list[str] = field(default_factory=list)


def add_attribution_change(changes: dict[str, CLIChange], api: str, detail: str) -> None:
    change = changes.get(api)
    if change is None:
        change = CLIChange(api_name=api, root=f"library/*/{api}", existing=True)
        changes[api] = change
    change.attribution_changes.append(detail)


def analyze_library_file(
    base: str,
    head: str,
    change: CLIChange,
    path: str,
    rest: str,
) -> None:
    if rest == ".printing-press.json":
        before = read_json(base, path)
        after = read_json(head, path)
        if before is None or after is None:
            change.surface_changes.append(path)
            return
        for field_name in sorted(ATTRIBUTION_FIELDS):
            if before.get(field_name) != after.get(field_name):
                change.attribution_changes.append(f"{path} {field_name}")
        if manifest_without_attribution(before) != manifest_without_attribution(after):
            change.surface_changes.append(path)
        return

    if rest == "SKILL.md":
        before = read_file(base, path)
        after = read_file(head, path)
        if skill_author(before) != skill_author(after):
            change.attribution_changes.append(f"{path} author")
        if normalize_skill_author(before) != normalize_skill_author(after):
            change.surface_changes.append(path)
        return

    change.surface_changes.append(path)


def validate_new_cli(head: str, change: CLIChange) -> list[str]:
    manifest_path = f"{change.root}/.printing-press.json"
    manifest = read_json(head, manifest_path)
    if manifest is None:
        return [f"::error file={manifest_path}::new CLI {change.root} must include .printing-press.json"]

    problems: list[str] = []
    printer = str(manifest.get("printer") or "")
    printer_name = str(manifest.get("printer_name") or "")
    api_name = str(manifest.get("api_name") or change.api_name)

    if printer in PLACEHOLDER_PRINTERS:
        problems.append(
            f"::error file={manifest_path}::new CLI {api_name} must set .printing-press.json printer to a real GitHub handle"
        )
    if printer_name in PLACEHOLDER_PRINTERS:
        problems.append(
            f"::error file={manifest_path}::new CLI {api_name} must set .printing-press.json printer_name to the printer display name"
        )
    if printer and printer == api_name:
        problems.append(
            f"::error file={manifest_path}::new CLI {api_name} has printer equal to api_name; set the human GitHub handle instead"
        )

    return problems


def validate_manifest_attribution_presence(head: str, roots: list[str]) -> list[str]:
    problems: list[str] = []
    for root in roots:
        manifest_path = f"{root}/.printing-press.json"
        if not path_exists(head, manifest_path):
            continue
        manifest = read_json(head, manifest_path)
        if manifest is None:
            problems.append(
                f"::error file={manifest_path}::unable to verify printer attribution because manifest JSON could not be read"
            )
            continue
        api_name = str(manifest.get("api_name") or Path(manifest_path).parent.name)
        for field_name in ("printer", "printer_name"):
            value = str(manifest.get(field_name) or "")
            if value in PLACEHOLDER_PRINTERS:
                problems.append(
                    f"::error file={manifest_path}::{api_name} is missing .printing-press.json {field_name}; every published CLI must preserve original printer attribution"
                )
    return problems


def run(base: str, head: str) -> int:
    files = changed_files(base, head)
    changes_by_api: dict[str, CLIChange] = {}
    failures: list[str] = []

    for path in files:
        root_info = cli_root_for_path(path)
        if root_info is None:
            continue
        root, rest = root_info
        base_manifest_path = f"{root}/.printing-press.json"
        head_manifest_path = f"{root}/.printing-press.json"
        existing = path_exists(base, base_manifest_path)
        api_name = api_name_for_root(base if existing else head, root)
        change = changes_by_api.setdefault(
            api_name,
            CLIChange(api_name=api_name, root=root, existing=existing),
        )
        change.changed_files.append(path)
        if existing:
            analyze_library_file(base, head, change, path, rest)
        elif rest != ".printing-press.json" or path_exists(head, head_manifest_path):
            change.surface_changes.append(path)

    changed_existing_roots = sorted(
        {change.root for change in changes_by_api.values() if change.existing}
    )
    failures.extend(validate_manifest_attribution_presence(head, changed_existing_roots))

    # cli-skills mirrors are generated, but an author flip there is still an
    # attribution-sensitive change. Pair it with library surface changes below.
    for path in files:
        api_name = cli_skill_api_for_path(path)
        if not api_name:
            continue
        before = read_file(base, path)
        after = read_file(head, path)
        if skill_author(before) != skill_author(after):
            add_attribution_change(changes_by_api, api_name, f"{path} author")

    if SWEEP_CANONICAL_PATH in files:
        before_map = parse_cli_author_map(base)
        after_map = parse_cli_author_map(head)
        for api_name in sorted(set(before_map) | set(after_map)):
            if before_map.get(api_name) != after_map.get(api_name):
                add_attribution_change(
                    changes_by_api,
                    api_name,
                    f"{SWEEP_CANONICAL_PATH} cliAuthorByAPIName[{api_name!r}]",
                )

    for change in sorted(changes_by_api.values(), key=lambda item: item.api_name):
        if not change.existing:
            failures.extend(validate_new_cli(head, change))
            continue

        if change.attribution_changes and change.surface_changes:
            detail = "; ".join(change.attribution_changes)
            surface = ", ".join(change.surface_changes[:5])
            if len(change.surface_changes) > 5:
                surface += ", ..."
            failures.append(
                f"::error file={change.root}/.printing-press.json::existing CLI {change.api_name} changes attribution ({detail}) while also changing CLI surface ({surface}). Split attribution corrections into a dedicated PR with primary evidence."
            )

    if failures:
        print("Attribution guard failed:")
        for failure in failures:
            print(failure)
        return 1

    checked = len(changes_by_api)
    print(f"Attribution guard passed for {checked} affected CLI(s).")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True, help="base ref to compare against")
    parser.add_argument("--head", default="HEAD", help="head ref to compare (default: HEAD)")
    args = parser.parse_args()

    try:
        return run(args.base, args.head)
    except RuntimeError as exc:
        print(f"::error::{exc}")
        return 2


if __name__ == "__main__":
    sys.exit(main())
