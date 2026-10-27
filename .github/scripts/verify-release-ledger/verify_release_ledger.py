#!/usr/bin/env python3
"""Verify PRs leave per-CLI release ledgers to post-merge automation."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path


RELEASE_MANIFEST = ".printing-press-release.json"
CHANGELOG = "CHANGELOG.md"
VERSION_VAR_RE = re.compile(r"\bvar\s+version\s*=")
STAMP_LITERAL_RE = re.compile(r'\bvar\s+version\s*=\s*"([^"]*)"')
CHANGELOG_RELEASE_RE = re.compile(r"^##\s+\d{4}\.\d{1,2}\.\d+\b", re.MULTILINE)


def git(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        check=check,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def changed_files(base_ref: str) -> list[str]:
    result = git("diff", "--name-only", f"{base_ref}...HEAD")
    return [line for line in result.stdout.splitlines() if line]


def base_has(base_ref: str, path: str) -> bool:
    return git("cat-file", "-e", f"{base_ref}:{path}", check=False).returncode == 0


def cli_parts(path: str) -> tuple[str, str] | None:
    parts = Path(path).parts
    if len(parts) < 4 or parts[0] != "library":
        return None
    return ("/".join(parts[:3]), "/".join(parts[3:]))


def is_release_file(rel: str) -> bool:
    return rel in {RELEASE_MANIFEST, CHANGELOG}


_module_rel_cache: dict[str, str] = {}


def is_direct_runtime_version(rel: str) -> bool:
    parts = Path(rel).parts
    if parts == ("internal", "cli", "root.go"):
        return True
    if parts == ("internal", "cli", "version.go"):
        return True
    return len(parts) == 3 and parts[0] == "cmd" and parts[2] == "main.go" and parts[1].endswith("-pp-mcp")


def module_rel(root: str) -> str:
    """Relative module dir under a catalog root, or "" when the module is the root.

    Same rule as resolveModuleDir in tools/release-ledger/main.go. The logic
    lives in this verifier, which CI overlays from the base branch. A PR cannot
    replace it with a helper script, and a branch that never had
    resolve-module-dir.sh still runs.
    """
    key = str(Path(root).resolve())
    if key in _module_rel_cache:
        return _module_rel_cache[key]
    rel = _module_rel_uncached(Path(root))
    _module_rel_cache[key] = rel
    return rel


def _module_rel_uncached(catalog: Path) -> str:
    if (catalog / "go.mod").is_file():
        return ""
    rel = _goreleaser_module_rel(catalog)
    if rel and (catalog / rel).is_dir():
        return rel
    return _sole_nested_module_rel(catalog)


def _goreleaser_module_rel(catalog: Path) -> str:
    path = catalog / ".goreleaser.yaml"
    if not path.is_file():
        return ""
    dirs: list[str] = []
    in_builds = False
    for line in path.read_text(encoding="utf-8").splitlines():
        trimmed = line.strip()
        if not trimmed or trimmed.startswith("#"):
            continue
        # Leading spaces only, matching indentOf in the Go stamper.
        if len(line) - len(line.lstrip(" ")) == 0:
            in_builds = trimmed == "builds:"
            continue
        if not in_builds:
            continue
        item = trimmed[2:].strip() if trimmed.startswith("- ") else trimmed
        key, sep, val = item.partition(":")
        if not sep or key.strip() != "dir":
            continue
        normalized = _normalize_build_dir(catalog, val)
        if normalized:
            dirs.append(normalized)
    unique = list(dict.fromkeys(dirs))
    if len(unique) > 1:
        raise RuntimeError(
            f"{catalog}: .goreleaser.yaml builds disagree on dir: {', '.join(unique)}"
        )
    return unique[0] if unique else ""


def _normalize_build_dir(catalog: Path, raw: str) -> str:
    val = raw.strip()
    comment = val.find(" #")
    if comment >= 0:
        val = val[:comment].strip()
    val = val.strip("'\"")
    val = val.strip()
    if val in ("", ".", "./"):
        return ""
    if val.startswith("./"):
        val = val[2:]
    val = val.rstrip("/")
    if val in ("", "."):
        return ""
    if ".." in val or val.startswith("/") or any(ch in val for ch in " \t\\"):
        raise RuntimeError(f"{catalog}: refusing goreleaser build dir {raw.strip()!r}")
    return val


def _sole_nested_module_rel(catalog: Path) -> str:
    if not catalog.is_dir():
        return ""
    found: list[str] = []
    for entry in catalog.iterdir():
        name = entry.name
        if not name or name.startswith(".") or not entry.is_dir():
            continue
        if (entry / "go.mod").is_file():
            found.append(name)
    if len(found) == 1:
        return found[0]
    return ""


def is_runtime_version_candidate(root: str, rel: str) -> bool:
    if is_direct_runtime_version(rel):
        return True
    nested = module_rel(root)
    if not nested:
        return False
    prefix = nested + "/"
    if not rel.startswith(prefix):
        return False
    return is_direct_runtime_version(rel[len(prefix):])


def diff_line_kinds(base_ref: str, path: str) -> tuple[bool, bool]:
    """Return (touches_version_var, touches_other_content)."""
    result = git("diff", "--unified=0", f"{base_ref}...HEAD", "--", path)
    version = False
    other = False
    for line in result.stdout.splitlines():
        if not line or line[0] not in "+-":
            continue
        if line.startswith(("+++", "---")):
            continue
        if VERSION_VAR_RE.search(line[1:]):
            version = True
        else:
            other = True
    return version, other


@dataclass
class CLIChange:
    root: str
    existed_on_base: bool
    release_files: set[str] = field(default_factory=set)
    runtime_version_files: set[str] = field(default_factory=set)
    non_release_files: set[str] = field(default_factory=set)


def collect_changes(base_ref: str) -> dict[str, CLIChange]:
    changes: dict[str, CLIChange] = {}
    for path in changed_files(base_ref):
        parsed = cli_parts(path)
        if not parsed:
            continue
        root, rel = parsed
        change = changes.setdefault(
            root,
            CLIChange(
                root=root,
                existed_on_base=base_has(base_ref, f"{root}/.printing-press.json"),
            ),
        )
        if is_release_file(rel):
            change.release_files.add(rel)
            continue
        if is_runtime_version_candidate(root, rel):
            version_change, other_change = diff_line_kinds(base_ref, path)
            if version_change:
                change.runtime_version_files.add(rel)
            if other_change:
                change.non_release_files.add(rel)
            continue
        change.non_release_files.add(rel)
    return changes


def validate_new_cli_skeleton(root: str, rel: str) -> list[str]:
    path = Path(root) / rel
    errors: list[str] = []
    if rel == RELEASE_MANIFEST:
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except Exception as exc:  # noqa: BLE001 - this is a verifier diagnostic.
            return [f"{path}: could not parse release manifest JSON: {exc}"]
        for key in ("version", "released_at", "source_commit"):
            if data.get(key):
                errors.append(f"{path}: new CLI release skeleton must leave {key!r} blank")
        if data.get("changes") not in (None, []):
            errors.append(f"{path}: new CLI release skeleton must omit changes or set it to []")
    elif rel == CHANGELOG:
        text = path.read_text(encoding="utf-8")
        if CHANGELOG_RELEASE_RE.search(text):
            errors.append(f"{path}: new CLI changelog skeleton must not contain a CalVer release section")
    return errors


def added_stamp_literals(base_ref: str, path: str) -> list[str]:
    result = git("diff", "--unified=0", f"{base_ref}...HEAD", "--", path)
    literals: list[str] = []
    for line in result.stdout.splitlines():
        if not line.startswith("+") or line.startswith("+++"):
            continue
        match = STAMP_LITERAL_RE.search(line[1:])
        if match:
            literals.append(match.group(1))
    return literals


def aligns_with_current_ledger(change: CLIChange, base_ref: str) -> bool:
    """True when new stamp literals equal the ledger version this PR did not bump.

    Introducing `var version = "<ledger>"` so the release job can see it is not
    a hand-bump. A literal that disagrees with .printing-press-release.json, or
    any edit to the ledger files themselves, still fails.
    """
    if change.release_files or not change.runtime_version_files:
        return False
    ledger_path = Path(change.root) / RELEASE_MANIFEST
    try:
        version = json.loads(ledger_path.read_text(encoding="utf-8")).get("version") or ""
    except (OSError, json.JSONDecodeError):
        return False
    if not version:
        return False
    saw_literal = False
    for rel in sorted(change.runtime_version_files):
        for literal in added_stamp_literals(base_ref, f"{change.root}/{rel}"):
            saw_literal = True
            if literal != version:
                return False
    return saw_literal


def verify(base_ref: str) -> list[str]:
    errors: list[str] = []
    try:
        changes = collect_changes(base_ref)
    except RuntimeError as exc:
        return [str(exc)]
    for change in changes.values():
        if not change.release_files and not change.runtime_version_files:
            continue

        if not change.existed_on_base:
            for rel in sorted(change.release_files):
                errors.extend(validate_new_cli_skeleton(change.root, rel))
            continue

        if change.non_release_files and not aligns_with_current_ledger(change, base_ref):
            files = ", ".join(sorted(change.release_files | change.runtime_version_files))
            normal = ", ".join(sorted(change.non_release_files)[:5])
            errors.append(
                f"{change.root}: release-ledger files/runtime version changed with normal CLI files. "
                f"Remove {files} from this PR; post-merge automation updates them. "
                f"Normal changed files include: {normal}"
            )
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-ref", required=True)
    args = parser.parse_args()

    errors = verify(args.base_ref)
    if errors:
        print("Release ledger changes must be left to post-merge automation.", file=sys.stderr)
        for error in errors:
            print(f"::error::{error}", file=sys.stderr)
        return 1
    print("Release ledger PR guard passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
