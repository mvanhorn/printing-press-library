#!/usr/bin/env python3
"""Decide whether .go-version is behind the press release or the Go vuln DB.

govulncheck -format=json emits the vuln DB as concatenated JSON objects, not
only the findings for the scanned module. A stdlib entry can list a fix on
the 1.27 line in the same event list as the 1.26 fix. The target version is
the fix for the range that actually contains .go-version, not the highest
fixed version in the file.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import sys
from pathlib import Path


def _load_verify_go_floor():
    path = Path(__file__).with_name("verify-go-floor.py")
    spec = importlib.util.spec_from_file_location("verify_go_floor", path)
    if spec is None or spec.loader is None:
        raise ImportError("could not load verify-go-floor.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


verify_go_floor = _load_verify_go_floor()


STDLIB_PACKAGES = {"stdlib", "toolchain"}


def iter_messages(payload: str) -> list[dict]:
    decoder = json.JSONDecoder()
    index = 0
    messages: list[dict] = []
    length = len(payload)
    while index < length:
        while index < length and payload[index].isspace():
            index += 1
        if index >= length:
            break
        message, end = decoder.raw_decode(payload, index)
        if not isinstance(message, dict):
            raise ValueError("govulncheck JSON value was not an object")
        messages.append(message)
        index = end
    return messages


def _range_fix(floor: str, events: list[dict]) -> str | None:
    """Return the fix when floor sits in this event list.

    "" means the matching range has no fixed version. None means floor is
    outside the ranges encoded by the events.
    """
    introduced: str | None = None
    for event in events:
        if "introduced" in event:
            introduced = str(event["introduced"])
            continue
        if introduced is None:
            continue
        if "fixed" in event:
            fixed = verify_go_floor.normalize_go_version(str(event["fixed"]))
            if (
                verify_go_floor.compare_go_versions(floor, introduced) >= 0
                and verify_go_floor.compare_go_versions(floor, fixed) < 0
            ):
                return fixed
            introduced = None
            continue
        if "last_affected" in event:
            last = verify_go_floor.normalize_go_version(str(event["last_affected"]))
            if (
                verify_go_floor.compare_go_versions(floor, introduced) >= 0
                and verify_go_floor.compare_go_versions(floor, last) <= 0
            ):
                return ""
            introduced = None
    if introduced is not None and verify_go_floor.compare_go_versions(floor, introduced) >= 0:
        return ""
    return None


def stdlib_fixes(payload: str, floor: str) -> tuple[list[str], list[str]]:
    fixes: list[str] = []
    unfixed: list[str] = []
    for message in iter_messages(payload):
        osv = message.get("osv")
        if isinstance(osv, dict):
            osv_id = str(osv.get("id") or "")
            for affected in osv.get("affected") or []:
                if not isinstance(affected, dict):
                    continue
                package = affected.get("package") or {}
                name = package.get("name") if isinstance(package, dict) else None
                if name not in STDLIB_PACKAGES:
                    continue
                for item in affected.get("ranges") or []:
                    if not isinstance(item, dict) or item.get("type") not in (None, "SEMVER"):
                        continue
                    events = item.get("events") or []
                    if not isinstance(events, list):
                        continue
                    fix = _range_fix(floor, events)
                    if fix is None:
                        continue
                    if fix == "":
                        if osv_id:
                            unfixed.append(osv_id)
                    else:
                        fixes.append(fix)
        finding = message.get("finding")
        if not isinstance(finding, dict):
            continue
        trace = finding.get("trace") or []
        module = ""
        if trace and isinstance(trace[0], dict):
            module = str(trace[0].get("module") or "")
        if module not in STDLIB_PACKAGES:
            continue
        fixed = finding.get("fixed_version") or ""
        if fixed:
            normalized = verify_go_floor.normalize_go_version(str(fixed))
            if verify_go_floor.compare_go_versions(floor, normalized) < 0:
                fixes.append(normalized)
        else:
            osv_id = str(finding.get("osv") or "")
            if osv_id:
                unfixed.append(osv_id)
    return _unique(fixes), _unique(unfixed)


def _unique(values: list[str]) -> list[str]:
    seen: list[str] = []
    for value in values:
        if value not in seen:
            seen.append(value)
    return seen


def highest(versions: list[str]) -> str | None:
    best: str | None = None
    for version in versions:
        if best is None or verify_go_floor.compare_go_versions(version, best) > 0:
            best = version
    return best


def decide(floor: str, press: str | None, payload: str) -> dict:
    fixes, unfixed = stdlib_fixes(payload, floor)
    candidates = list(fixes)
    if press is not None and verify_go_floor.compare_go_versions(floor, press) < 0:
        candidates.append(press)
    target = highest(candidates)
    return {
        "floor": floor,
        "press_directive": press,
        "stdlib_fixes": fixes,
        "unfixed": unfixed,
        "target": target,
        "status": "behind" if target or unfixed else "current",
    }


def issue_body(decision: dict, press_tag: str, canary: str) -> str:
    fixes = decision["stdlib_fixes"] or ["none"]
    unfixed = decision["unfixed"] or ["none"]
    press = decision["press_directive"] or "unavailable"
    target = decision["target"]
    return "\n".join(
        [
            "The published library `.go-version` is `%s`." % decision["floor"],
            "",
            "- Latest cli-printing-press release `%s` `librarySafeGoDirective`: `%s`"
            % (press_tag, press),
            "- govulncheck (`%s`, module scan, live vuln DB) stdlib/toolchain fixes newer than the floor: %s"
            % (canary, ", ".join("`%s`" % item if item != "none" else item for item in fixes)),
            "- Unfixed stdlib findings: %s" % ", ".join(unfixed),
            "",
            "Bump `.go-version` to `%s` in this repo first. Set `MIN_PRESS_VERSION` to the first press release that prints that floor. Do not bulk-bump per-CLI `go` directives."
            % target,
            "",
            "Renovate automerges patch updates of `.go-version` when the GitHub App is installed. This issue is the backstop when the library is behind the press release or the vuln DB.",
            "",
        ]
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--floor-file", required=True)
    parser.add_argument("--press-file", required=True)
    parser.add_argument("--press-tag", required=True)
    parser.add_argument("--govuln-json", required=True)
    parser.add_argument("--canary", required=True)
    args = parser.parse_args(argv)

    try:
        floor = verify_go_floor.normalize_go_version(Path(args.floor_file).read_text().strip())
        verify_go_floor.version_parts(floor)
        press = verify_go_floor.press_directive(Path(args.press_file).read_text())
        if press is None:
            raise ValueError("press go_version.go did not contain librarySafeGoDirective")
        verify_go_floor.version_parts(press)
        payload = Path(args.govuln_json).read_text()
        if not payload.strip():
            raise ValueError("govulncheck JSON was empty")
        decision = decide(floor, press, payload)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(str(exc), file=sys.stderr)
        return 2

    decision["press_tag"] = args.press_tag
    decision["canary"] = args.canary
    decision["issue_body"] = issue_body(decision, args.press_tag, args.canary) if decision["target"] else ""
    json.dump(decision, sys.stdout)
    sys.stdout.write("\n")
    if decision["target"]:
        return 1
    if decision["unfixed"]:
        return 3
    return 0


if __name__ == "__main__":
    sys.exit(main())
