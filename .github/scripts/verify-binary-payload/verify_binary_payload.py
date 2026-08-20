#!/usr/bin/env python3
"""Block committed MCPB/native binary payloads under library/.

Published CLI and MCPB release assets must be built from source in GitHub
Actions. This guard catches PRs that add or modify prebuilt payloads in the
catalog tree before those files can be signed or released.
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]

BINARY_SUFFIXES = {
    ".mcpb",
    ".dylib",
    ".so",
    ".dll",
    ".exe",
}

MAGIC_PREFIXES = {
    b"\x7fELF": "ELF executable/shared object",
    b"MZ": "Windows PE executable",
    b"PK\x03\x04": "ZIP/MCPB archive",
    b"\xca\xfe\xba\xbe": "Mach-O universal binary / Java class file",
    b"\xfe\xed\xfa\xce": "Mach-O executable",
    b"\xfe\xed\xfa\xcf": "Mach-O executable",
    b"\xce\xfa\xed\xfe": "Mach-O executable",
    b"\xcf\xfa\xed\xfe": "Mach-O executable",
}


def suffix_kind(path: Path) -> str | None:
    name = path.name.lower()
    for suffix in BINARY_SUFFIXES:
        if name.endswith(suffix) or (suffix == ".so" and ".so." in name):
            return f"{suffix} artifact"
    return None


def run(args: list[str]) -> str:
    completed = subprocess.run(
        args,
        cwd=REPO_ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    )
    return completed.stdout


def changed_library_paths(base_ref: str) -> list[Path]:
    diff = run(
        [
            "git",
            "diff",
            "--name-only",
            "--diff-filter=AMRC",
            f"{base_ref}...HEAD",
            "--",
            "library/",
        ]
    )
    return [REPO_ROOT / line for line in diff.splitlines() if line]


def binary_kind(path: Path) -> str | None:
    if kind := suffix_kind(path):
        return kind
    with path.open("rb") as fh:
        prefix = fh.read(4)
    for magic, label in MAGIC_PREFIXES.items():
        if prefix.startswith(magic):
            return label
    return None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--base-ref",
        default="origin/main",
        help="Base git ref to compare against; defaults to origin/main.",
    )
    args = parser.parse_args()

    try:
        paths = changed_library_paths(args.base_ref)
    except subprocess.CalledProcessError as exc:
        output = (exc.stderr or exc.output or "").strip()
        detail = f": {output}" if output else ""
        print(f"::error::Could not list changed library files (git exited {exc.returncode}){detail}")
        return 1

    problems: list[str] = []
    for path in paths:
        if not path.is_file():
            continue
        rel = path.relative_to(REPO_ROOT)
        try:
            kind = binary_kind(path)
        except OSError as exc:
            problems.append(
                f"::error file={rel}::Could not read changed library file to check for "
                f"binary content: {exc}"
            )
            continue
        if kind:
            problems.append(
                f"::error file={rel}::Do not commit {kind} payloads under library/. "
                "MCPB and native binaries must be built from source in GitHub Actions before signing."
            )

    if problems:
        for problem in problems:
            print(problem)
        return 1

    print("No newly added or modified MCPB/native binary payloads under library/.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
