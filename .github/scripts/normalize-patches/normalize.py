#!/usr/bin/env python3
"""Normalize legacy single-array .printing-press-patches.json into a per-patch directory.

Why this exists
---------------
`.printing-press-patches.json` is a single JSON array (`patches: [...]`). Every
PR that customizes the *same* published CLI appends one object to that one
array, so any two in-flight PRs on the same CLI conflict on this file at merge
time — a structural conflict unrelated to the actual code change
(mvanhorn/cli-printing-press#2496).

The fix is to store one patch per file under a directory:

    library/<cat>/<slug>/.printing-press-patches/
      <id>.json     one self-contained patch per file
      _meta.json    only when the CLI carries global (non-per-patch) lists
      .gitkeep      present even at zero patches (git won't track an empty dir)

Two PRs adding different patches now write different files, which git never
conflicts on.

Because the Printing Press is distributed and versioned, old binaries/skills
keep emitting the single-array file indefinitely. So this is not a one-shot
migration: it runs post-merge on `main` (.github/workflows/normalize-patches.yml)
to convert whatever old-format files land, and the same script is used for the
one-time targeted sweep of existing high-churn CLIs.

Usage
-----
    normalize.py [--root DIR] [--check] [CLI_DIR ...]

  * With no CLI_DIR args, scans --root (default: library) for every
    .printing-press-patches.json and normalizes each.
  * With CLI_DIR args, normalizes exactly those directories (the targeted-sweep
    path); each must be a library/<cat>/<slug> directory.
  * --check makes no writes and exits 1 if any directory would change. Used by
    the idempotency test and as a dry run.

Idempotency is a hard requirement: running twice yields zero diff. The
normalize_test.py suite enforces it.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

LEGACY_FILENAME = ".printing-press-patches.json"
PATCHES_DIRNAME = ".printing-press-patches"
META_FILENAME = "_meta.json"
GITKEEP_FILENAME = ".gitkeep"

SCHEMA_VERSION = 2

# Top-level keys that are provenance the per-patch files inherit, not CLI-global
# data. Everything else at the top level (deferred_to_upstream, upstream_tracking,
# machine_followups, …) is CLI-global and is preserved in _meta.json.
PROVENANCE_KEYS = ("applied_at", "base_run_id", "base_printing_press_version")
STD_TOP_KEYS = {"schema_version", "patches", *PROVENANCE_KEYS}

# Order the well-known fields first in each emitted patch file for readability;
# any other original keys follow in their original order.
PATCH_KEY_ORDER = ("schema_version", "id", *PROVENANCE_KEYS)


def slugify(value: str) -> str:
    """Filesystem-safe kebab slug. Used for patch filenames only; the original
    `id` value is preserved verbatim inside the file."""
    value = value.strip().lower()
    value = re.sub(r"[^a-z0-9]+", "-", value)
    value = re.sub(r"-{2,}", "-", value).strip("-")
    return value[:60].strip("-")


def patch_filename(patch: dict, index: int, taken: set[str]) -> str:
    """Deterministic filename for a patch. Prefers a slug of its `id`; falls back
    to a slug of `summary`, then `patch-NN`. Dedups against names already taken."""
    raw_id = patch.get("id")
    base = ""
    if isinstance(raw_id, str):
        base = slugify(raw_id)
    if not base and isinstance(patch.get("summary"), str):
        base = slugify(patch["summary"])
    if not base:
        base = f"patch-{index + 1:02d}"

    name = base
    suffix = 2
    while f"{name}.json" in taken:
        name = f"{base}-{suffix}"
        suffix += 1
    taken.add(f"{name}.json")
    return f"{name}.json"


def build_patch_object(patch: dict, top: dict) -> dict:
    """Return the self-contained per-patch object: schema_version + inherited
    provenance + every original key, with well-known keys ordered first."""
    out: dict = {"schema_version": SCHEMA_VERSION}

    # Ensure an `id` is present (synthesized callers pass it through the original
    # dict, but a patch may legitimately lack one — keep whatever is there).
    if patch.get("id") is not None:
        out["id"] = patch["id"]

    # Inherit provenance from the top level when the patch doesn't carry its own.
    for key in PROVENANCE_KEYS:
        if patch.get(key) is not None:
            out[key] = patch[key]
        elif top.get(key) is not None:
            out[key] = top[key]

    # Append every remaining original key in its original order.
    for key, val in patch.items():
        if key not in out:
            out[key] = val

    # Reorder so PATCH_KEY_ORDER comes first, then the rest as inserted.
    ordered = {k: out[k] for k in PATCH_KEY_ORDER if k in out}
    for k, v in out.items():
        if k not in ordered:
            ordered[k] = v
    return ordered


def desired_files(legacy_data: dict, existing_dir: dict[str, dict] | None) -> dict[str, str]:
    """Compute the full desired content of the patches dir as {filename: text}.

    existing_dir maps already-present <name>.json -> parsed object (the inflow /
    merge case where a dir already exists alongside an incoming legacy file). On
    a plain migration it is None/empty.
    """
    existing_dir = existing_dir or {}
    files: dict[str, str] = {}

    # Preserve any files already in the dir verbatim (dir is source of truth on
    # merge); their names are reserved so incoming patches can't collide.
    taken = set(existing_dir.keys())
    existing_ids = {
        obj.get("id")
        for obj in existing_dir.values()
        if isinstance(obj, dict) and obj.get("id") is not None
    }
    for name, obj in existing_dir.items():
        if name in (META_FILENAME, GITKEEP_FILENAME):
            continue
        files[name] = dumps(obj)

    patches = legacy_data.get("patches") or []
    if isinstance(patches, list):
        for index, patch in enumerate(patches):
            if not isinstance(patch, dict):
                continue
            # On merge, an incoming patch whose id already lives in the dir is a
            # duplicate (old-Press branch re-emitting a converted patch) — skip.
            if patch.get("id") is not None and patch["id"] in existing_ids:
                continue
            name = patch_filename(patch, index, taken)
            files[name] = dumps(build_patch_object(patch, legacy_data))

    # CLI-global lists (everything at the top level that isn't provenance or the
    # patches array) live in _meta.json. This is the one shared file that
    # remains, and it changes rarely.
    global_keys = {k: v for k, v in legacy_data.items() if k not in STD_TOP_KEYS}
    existing_meta = existing_dir.get(META_FILENAME) or {}
    if global_keys or existing_meta:
        meta: dict = {"schema_version": SCHEMA_VERSION}
        # Existing meta wins for scalar collisions; lists are unioned.
        for key, val in {**global_keys, **{k: v for k, v in existing_meta.items() if k != "schema_version"}}.items():
            if isinstance(global_keys.get(key), list) and isinstance(existing_meta.get(key), list):
                merged = list(existing_meta[key])
                for item in global_keys[key]:
                    if item not in merged:
                        merged.append(item)
                meta[key] = merged
            else:
                meta[key] = val
        files[META_FILENAME] = dumps(meta)

    return files


def dumps(obj: dict) -> str:
    return json.dumps(obj, indent=2, ensure_ascii=False) + "\n"


def read_json(path: Path) -> dict:
    return json.loads(path.read_text())


def normalize_dir(cli_dir: Path, check: bool) -> tuple[bool, list[str]]:
    """Normalize one CLI directory. Returns (changed, errors)."""
    errors: list[str] = []
    legacy = cli_dir / LEGACY_FILENAME
    patches_dir = cli_dir / PATCHES_DIRNAME

    has_legacy = legacy.is_file()
    has_dir = patches_dir.is_dir()
    if not has_legacy and not has_dir:
        return (False, errors)

    legacy_data: dict = {}
    if has_legacy:
        try:
            legacy_data = read_json(legacy)
        except (OSError, json.JSONDecodeError) as exc:
            errors.append(f"{legacy}: unparseable, skipped ({exc})")
            return (False, errors)
        if not isinstance(legacy_data, dict):
            errors.append(f"{legacy}: top level is not an object, skipped")
            return (False, errors)

    existing: dict[str, dict] = {}
    if has_dir:
        for f in sorted(patches_dir.glob("*.json")):
            try:
                existing[f.name] = read_json(f)
            except (OSError, json.JSONDecodeError) as exc:
                errors.append(f"{f}: unparseable, skipped ({exc})")
                return (False, errors)

    desired = desired_files(legacy_data, existing if has_dir else None)

    # Compute the target on-disk state of the dir: desired *.json + .gitkeep,
    # nothing else. .gitkeep guarantees the dir survives at zero patches.
    target: dict[str, str | None] = dict(desired)
    target[GITKEEP_FILENAME] = ""

    # Determine current on-disk state of the dir.
    current: dict[str, str] = {}
    if has_dir:
        for f in sorted(patches_dir.iterdir()):
            if f.is_file():
                current[f.name] = f.read_text()

    changed = False

    # Files to write or rewrite.
    writes = {n: t for n, t in target.items() if current.get(n) != t}
    # Stray files in the dir not in target (besides ones we manage) get removed
    # only if they are .json we'd be replacing the legacy source of — to stay
    # conservative we remove only *.json no longer desired plus an obsolete
    # legacy file. We never touch unknown non-json files.
    deletes = [n for n in current if n.endswith(".json") and n not in target]

    if writes or deletes or has_legacy:
        changed = True

    if check or not changed:
        return (changed, errors)

    patches_dir.mkdir(exist_ok=True)
    for name, text in writes.items():
        (patches_dir / name).write_text(text)
    for name in deletes:
        (patches_dir / name).unlink()
    if has_legacy:
        legacy.unlink()

    return (changed, errors)


def discover(root: Path) -> list[Path]:
    dirs: set[Path] = set()
    for legacy in root.rglob(LEGACY_FILENAME):
        dirs.add(legacy.parent)
    for d in root.rglob(PATCHES_DIRNAME):
        if d.is_dir():
            dirs.add(d.parent)
    return sorted(dirs)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--root", default="library", help="Library root to scan when no CLI_DIR is given (default: library)")
    parser.add_argument("--check", action="store_true", help="Report changes without writing; exit 1 if any dir would change")
    parser.add_argument("dirs", nargs="*", help="Specific library/<cat>/<slug> directories to normalize")
    args = parser.parse_args(argv)

    if args.dirs:
        targets = [Path(d) for d in args.dirs]
    else:
        targets = discover(Path(args.root))

    any_changed = False
    all_errors: list[str] = []
    for cli_dir in targets:
        changed, errors = normalize_dir(cli_dir, args.check)
        all_errors.extend(errors)
        if changed:
            any_changed = True
            print(f"{'would change' if args.check else 'normalized'}: {cli_dir}")

    for err in all_errors:
        print(f"ERROR: {err}", file=sys.stderr)

    if all_errors:
        return 2
    if args.check and any_changed:
        return 1
    if not any_changed:
        print("No patches files needed normalization.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
