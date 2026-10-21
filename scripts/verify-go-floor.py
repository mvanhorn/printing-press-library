#!/usr/bin/env python3
"""Check published-library Go floor surfaces against .go-version.

.go-version is the CI toolchain. setup-go reads it, then the workflow sets
GOTOOLCHAIN=local so the go command cannot download a different toolchain.
This script fails when a tracked surface disagrees with that file. The
comparison with cli-printing-press librarySafeGoDirective warns, and does
not fail, when the raw file cannot be fetched.
"""

from __future__ import annotations

import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

import yaml


REPO_ROOT = Path(__file__).resolve().parents[1]
PRESS_GO_VERSION_URL = (
    "https://raw.githubusercontent.com/mvanhorn/cli-printing-press/main/"
    "internal/generator/go_version.go"
)

GO_OR_NEWER = re.compile(r"\bGo (?P<version>\d+\.\d+(?:\.\d+)?) or newer\b")
TOOLCHAIN = re.compile(r"(?m)^[ \t]*toolchain\s+go(?P<version>\S+)\s*$")
DEFAULT_FLOOR = re.compile(r'(?m)^DEFAULT_GO_FLOOR\s*=\s*"(?P<version>[^"]+)"')
PRESS_DIRECTIVE = re.compile(r'librarySafeGoDirective\s*=\s*"(?P<version>[^"]+)"')

# This job sparse-checkouts one CLI, so the repo .go-version file is absent.
GO_VERSION_FILE_EXCEPTIONS = {
    ".github/workflows/verify-ordertogo-windows.yml": "library/food-and-dining/ordertogo/go.mod",
}


def normalize_go_version(value: str) -> str:
    text = value.strip()
    if text.startswith("go") and len(text) > 2 and text[2].isdigit():
        text = text[2:]
    if text.startswith("v") and len(text) > 1 and text[1].isdigit():
        text = text[1:]
    return text


def version_parts(value: str) -> tuple[int, ...]:
    text = normalize_go_version(value)
    if not text:
        raise ValueError("empty Go version")
    parts: list[int] = []
    for component in text.split("."):
        digits = ""
        for char in component:
            if char.isdigit():
                digits += char
            else:
                break
        if not digits:
            raise ValueError("unparseable Go version %r" % value)
        parts.append(int(digits))
    return tuple(parts)


def compare_go_versions(left: str, right: str) -> int:
    left_parts = version_parts(left)
    right_parts = version_parts(right)
    width = max(len(left_parts), len(right_parts))
    for idx in range(width):
        lval = left_parts[idx] if idx < len(left_parts) else 0
        rval = right_parts[idx] if idx < len(right_parts) else 0
        if lval != rval:
            return -1 if lval < rval else 1
    return 0


def read_floor(root: Path) -> str:
    path = root / ".go-version"
    if not path.is_file():
        raise ValueError(".go-version is missing")
    floor = path.read_text().strip()
    version_parts(floor)
    return floor


def fetch_press_source(url: str = PRESS_GO_VERSION_URL, timeout: float = 20) -> str:
    request = urllib.request.Request(url, headers={"User-Agent": "printing-press-library-go-floor"})
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return response.read().decode("utf-8")


def press_directive(source: str) -> str | None:
    match = PRESS_DIRECTIVE.search(source)
    if not match:
        return None
    return normalize_go_version(match.group("version"))


def _check_default_floor(root: Path, floor: str, failures: list[str]) -> None:
    path = root / ".github/scripts/verify-supply-chain/signals.py"
    if not path.is_file():
        failures.append("signals.py is missing; DEFAULT_GO_FLOOR cannot be checked")
        return
    versions = [match.group("version") for match in DEFAULT_FLOOR.finditer(path.read_text())]
    if versions != [floor]:
        failures.append(
            "DEFAULT_GO_FLOOR is %s; .go-version is %s" % (versions or "missing", floor)
        )


def _check_sweep(root: Path, floor: str, failures: list[str]) -> None:
    path = root / "tools/sweep-canonical/main.go"
    if not path.is_file():
        failures.append("tools/sweep-canonical/main.go is missing")
        return
    text = path.read_text()
    literals = [match.group("version") for match in GO_OR_NEWER.finditer(text)]
    derived = "readRepoGoVersion" in text and ".go-version" in text
    stale = [version for version in literals if version != floor]
    if derived:
        if stale:
            failures.append(
                "sweep-canonical reads .go-version but also hardcodes Go %s or newer"
                % ", ".join(stale)
            )
        return
    if not literals or stale:
        failures.append(
            "sweep-canonical minimum is not derived from .go-version and does not match %s" % floor
        )


def _go_or_newer_versions(text: str) -> list[str]:
    return [match.group("version") for match in GO_OR_NEWER.finditer(text)]


def _check_readme(root: Path, floor: str, failures: list[str]) -> None:
    _check_root_readme(root, floor, failures)
    _check_npm_readme(root, floor, failures)


def _check_root_readme(root: Path, floor: str, failures: list[str]) -> None:
    rel = "README.md"
    path = root / rel
    if not path.is_file():
        failures.append("%s is missing" % rel)
        return
    versions = _go_or_newer_versions(path.read_text())
    if not versions:
        failures.append("%s has no 'Go X or newer' floor" % rel)
        return
    stale = [version for version in versions if version != floor]
    if stale:
        failures.append("%s says Go %s or newer; .go-version is %s" % (rel, ", ".join(stale), floor))


def _check_npm_readme(root: Path, floor: str, failures: list[str]) -> None:
    """npm/README.md names .go-version so a patch bump does not publish npm.

    A hardcoded ``Go X or newer`` line is still checked. A stale literal fails
    even when the file also names .go-version.
    """
    rel = "npm/README.md"
    path = root / rel
    if not path.is_file():
        failures.append("%s is missing" % rel)
        return
    text = path.read_text()
    versions = _go_or_newer_versions(text)
    stale = [version for version in versions if version != floor]
    if stale:
        failures.append("%s says Go %s or newer; .go-version is %s" % (rel, ", ".join(stale), floor))
        return
    if versions or ".go-version" in text:
        return
    failures.append("%s must mention .go-version or say 'Go %s or newer'" % (rel, floor))


def _check_toolchains(root: Path, floor: str, failures: list[str]) -> None:
    library = root / "library"
    if not library.is_dir():
        return
    for path in sorted(library.glob("*/*/go.mod")):
        for match in TOOLCHAIN.finditer(path.read_text()):
            declared = match.group("version")
            try:
                older = compare_go_versions(declared, floor) < 0
            except ValueError:
                failures.append("%s has an unparseable toolchain go%s" % (path.relative_to(root), declared))
                continue
            if older:
                rel = path.relative_to(root)
                failures.append("%s toolchain go%s is below .go-version %s" % (rel, declared, floor))


def _step_uses_setup_go(step: object) -> bool:
    if not isinstance(step, dict):
        return False
    uses = step.get("uses")
    return isinstance(uses, str) and uses.startswith("actions/setup-go@")


def _step_pins_local_toolchain(step: object) -> bool:
    if not isinstance(step, dict):
        return False
    run = step.get("run")
    if isinstance(run, str):
        return "GOTOOLCHAIN=local" in run
    if isinstance(run, list):
        return any(isinstance(item, str) and "GOTOOLCHAIN=local" in item for item in run)
    return False


def _step_go_version_file(step: object) -> str | None:
    if not isinstance(step, dict):
        return None
    with_block = step.get("with")
    if not isinstance(with_block, dict):
        return None
    value = with_block.get("go-version-file")
    if not isinstance(value, str):
        return None
    return value.strip()


def _check_job_setup_go(rel: str, job_name: str, steps: list[object], failures: list[str]) -> None:
    for index, step in enumerate(steps):
        if not _step_uses_setup_go(step):
            continue
        later = steps[index + 1 :]
        if not any(_step_pins_local_toolchain(item) for item in later):
            failures.append(
                "%s job %s uses setup-go without a later GOTOOLCHAIN=local step" % (rel, job_name)
            )
        expected = GO_VERSION_FILE_EXCEPTIONS.get(rel)
        declared = _step_go_version_file(step)
        if expected is not None:
            if declared != expected:
                failures.append("%s job %s must keep go-version-file: %s" % (rel, job_name, expected))
            continue
        if declared != ".go-version":
            failures.append(
                "%s job %s setup-go must use go-version-file: .go-version" % (rel, job_name)
            )


def _check_workflows(root: Path, failures: list[str]) -> None:
    workflow_dir = root / ".github" / "workflows"
    if not workflow_dir.is_dir():
        return
    paths = sorted(workflow_dir.glob("*.yml")) + sorted(workflow_dir.glob("*.yaml"))
    for path in paths:
        text = path.read_text()
        if "actions/setup-go" not in text:
            continue
        rel = path.relative_to(root).as_posix()
        try:
            document = yaml.safe_load(text)
        except yaml.YAMLError as exc:
            failures.append("%s is not valid workflow YAML (%s)" % (rel, exc))
            continue
        if not isinstance(document, dict):
            failures.append("%s workflow YAML did not parse to a mapping" % rel)
            continue
        jobs = document.get("jobs")
        if not isinstance(jobs, dict):
            failures.append("%s uses setup-go but has no jobs mapping" % rel)
            continue
        saw_setup_go = False
        for job_name, job in jobs.items():
            if not isinstance(job, dict):
                continue
            steps = job.get("steps")
            if not isinstance(steps, list):
                continue
            if any(_step_uses_setup_go(step) for step in steps):
                saw_setup_go = True
            _check_job_setup_go(rel, str(job_name), steps, failures)
        if not saw_setup_go:
            failures.append("%s mentions setup-go but no job has an actions/setup-go step" % rel)


def _check_press(floor: str, press_source: str | None, press_fetch_error: str | None) -> tuple[list[str], list[str]]:
    failures: list[str] = []
    warnings: list[str] = []
    if press_fetch_error is not None:
        warnings.append(
            "could not fetch press librarySafeGoDirective (%s); skipping the upstream comparison"
            % press_fetch_error
        )
        return failures, warnings
    if press_source is None:
        return failures, warnings
    directive = press_directive(press_source)
    if directive is None:
        failures.append("press go_version.go did not contain librarySafeGoDirective")
        return failures, warnings
    try:
        behind = compare_go_versions(floor, directive) < 0
    except ValueError:
        failures.append("press librarySafeGoDirective %s is unparseable" % directive)
        return failures, warnings
    if behind:
        failures.append(
            ".go-version %s is behind press librarySafeGoDirective %s" % (floor, directive)
        )
    return failures, warnings


def evaluate(
    root: Path,
    press_source: str | None = None,
    press_fetch_error: str | None = None,
) -> tuple[list[str], list[str]]:
    failures: list[str] = []
    try:
        floor = read_floor(root)
    except ValueError as exc:
        return [str(exc)], []
    _check_default_floor(root, floor, failures)
    _check_sweep(root, floor, failures)
    _check_readme(root, floor, failures)
    _check_toolchains(root, floor, failures)
    _check_workflows(root, failures)
    press_failures, warnings = _check_press(floor, press_source, press_fetch_error)
    failures.extend(press_failures)
    return failures, warnings


def main() -> int:
    try:
        press_source = fetch_press_source()
        press_error = None
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        press_source = None
        press_error = str(exc)
    failures, warnings = evaluate(REPO_ROOT, press_source, press_error)
    for warning in warnings:
        print("warning: %s" % warning, file=sys.stderr)
    if failures:
        print("Go floor check failed:")
        for failure in failures:
            print("  %s" % failure)
        return 1
    floor = read_floor(REPO_ROOT)
    print("Go floor check passed: tracked surfaces match .go-version %s." % floor)
    return 0


if __name__ == "__main__":
    sys.exit(main())
