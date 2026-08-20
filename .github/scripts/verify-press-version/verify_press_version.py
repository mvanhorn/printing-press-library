#!/usr/bin/env python3
"""Require newly published library CLIs to come from a current cli-printing-press."""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path, PurePosixPath


REPO_ROOT = Path(__file__).resolve().parents[3]
MIN_PRESS_VERSION = "4.10.0"
MIN_VERSION_PARTS = tuple(int(part) for part in MIN_PRESS_VERSION.split("."))


@dataclass(frozen=True)
class Problem:
    file: Path | None
    message: str


def annotation_escape(value: str) -> str:
    return value.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")


def emit_error(problem: Problem) -> None:
    message = annotation_escape(problem.message)
    if problem.file is None:
        print(f"::error::{message}")
        return
    print(f"::error file={rel(problem.file)}::{message}")


def rel(path: Path) -> str:
    return path.relative_to(REPO_ROOT).as_posix()


def run_git(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def library_cli_dir_for(path: str) -> Path | None:
    parts = PurePosixPath(path).parts
    if len(parts) < 3 or parts[0] != "library":
        return None
    return REPO_ROOT / parts[0] / parts[1] / parts[2]


def changed_cli_dirs(base_ref: str) -> list[Path]:
    result = run_git(["diff", "--name-only", "--diff-filter=d", f"{base_ref}...HEAD", "--", "library"])
    if result.returncode != 0:
        print(result.stderr, file=sys.stderr)
        raise SystemExit(result.returncode)

    dirs: set[Path] = set()
    for path in result.stdout.splitlines():
        cli_dir = library_cli_dir_for(path)
        if cli_dir is not None and cli_dir.is_dir():
            dirs.add(cli_dir)
    return sorted(dirs, key=rel)


def git_exists(base_ref: str, path: Path) -> bool:
    result = run_git(["cat-file", "-e", f"{base_ref}:{rel(path)}"])
    return result.returncode == 0


def is_new_cli(base_ref: str, cli_dir: Path) -> bool:
    return not git_exists(base_ref, cli_dir / ".printing-press.json")


def parse_semver(value: object) -> tuple[int, int, int] | None:
    if not isinstance(value, str):
        return None
    match = re.match(r"^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$", value.strip())
    if match is None:
        return None
    return tuple(int(part) for part in match.groups())


def upgrade_message(actual: object) -> str:
    if actual is None:
        version_clause = "printing_press_version is not set"
    elif not isinstance(actual, str) or not actual:
        version_clause = f"printing_press_version {actual!r} is not a valid version"
    elif parse_semver(actual) is None:
        version_clause = f"printing_press_version {actual!r} is not a valid version string"
    else:
        version_clause = (
            f"printing_press_version {actual!r} is below the required "
            f"cli-printing-press version v{MIN_PRESS_VERSION}"
        )
    return (
        f"{version_clause}. Upgrade the generator checkout and installed tooling, then "
        "re-run the print/publish step so this manifest records the new version. Required steps: "
        "git -C <cli-printing-press checkout> pull --ff-only; "
        "go install github.com/mvanhorn/cli-printing-press/v4/cmd/cli-printing-press@latest; "
        "cli-printing-press --version; "
        "install/update the latest cli-printing-press skills from mvanhorn/cli-printing-press; "
        "then re-run /cli-printing-press or /cli-printing-press-publish."
    )


def validate_cli_dir(cli_dir: Path) -> list[Problem]:
    manifest_path = cli_dir / ".printing-press.json"
    try:
        manifest = json.loads(manifest_path.read_text())
    except FileNotFoundError:
        return [Problem(manifest_path, "touched library CLI is missing .printing-press.json")]
    except json.JSONDecodeError as exc:
        return [Problem(manifest_path, f".printing-press.json is not valid JSON: {exc}")]

    if not isinstance(manifest, dict):
        return [Problem(manifest_path, ".printing-press.json must contain a JSON object")]

    raw_version = manifest.get("printing_press_version")
    parsed = parse_semver(raw_version)
    if parsed is None or parsed < MIN_VERSION_PARTS:
        return [Problem(manifest_path, upgrade_message(raw_version))]

    return []


def run(base_ref: str) -> int:
    problems: list[Problem] = []
    for cli_dir in changed_cli_dirs(base_ref):
        if not is_new_cli(base_ref, cli_dir):
            continue
        problems.extend(validate_cli_dir(cli_dir))

    if not problems:
        print(f"All newly added library CLIs declare cli-printing-press >= v{MIN_PRESS_VERSION}.")
        return 0

    for problem in problems:
        emit_error(problem)
    return 1


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-ref", required=True, help="Base git ref to compare against")
    args = parser.parse_args(argv)
    return run(args.base_ref)


if __name__ == "__main__":
    raise SystemExit(main())
