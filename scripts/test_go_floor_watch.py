#!/usr/bin/env python3
"""Tests for scripts/go_floor_watch.py."""

from __future__ import annotations

import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

import go_floor_watch


def govuln(*objects: dict) -> str:
    return "\n".join(json.dumps(obj) for obj in objects)


X509 = {
    "osv": {
        "id": "GO-2026-6809",
        "affected": [
            {
                "package": {"name": "stdlib", "ecosystem": "Go"},
                "ranges": [
                    {
                        "type": "SEMVER",
                        "events": [
                            {"introduced": "0"},
                            {"fixed": "1.26.10"},
                            {"introduced": "1.27.0-0"},
                            {"fixed": "1.27.3"},
                        ],
                    }
                ],
            }
        ],
    }
}

NET = {
    "osv": {
        "id": "GO-2099-0001",
        "affected": [
            {
                "package": {"name": "golang.org/x/net", "ecosystem": "Go"},
                "ranges": [
                    {
                        "type": "SEMVER",
                        "events": [
                            {"introduced": "0"},
                            {"fixed": "0.99.0"},
                        ],
                    }
                ],
            }
        ],
    }
}


class GoFloorWatchTest(unittest.TestCase):
    def test_current_floor_is_not_behind_a_higher_minor_fix(self) -> None:
        decision = go_floor_watch.decide("1.26.10", "1.26.10", govuln(X509, NET))
        self.assertEqual(decision["status"], "current")
        self.assertIsNone(decision["target"])
        self.assertEqual(decision["stdlib_fixes"], [])

    def test_older_floor_uses_the_matching_line_fix(self) -> None:
        decision = go_floor_watch.decide("1.26.9", "1.26.10", govuln(X509, NET))
        self.assertEqual(decision["target"], "1.26.10")
        self.assertEqual(decision["stdlib_fixes"], ["1.26.10"])

    def test_press_ahead_of_a_clean_toolchain_sets_the_target(self) -> None:
        decision = go_floor_watch.decide("1.26.10", "1.26.11", govuln(X509))
        self.assertEqual(decision["target"], "1.26.11")

    def test_vuln_fix_above_press_wins(self) -> None:
        newer = {
            "finding": {
                "osv": "GO-2026-9999",
                "fixed_version": "v1.26.12",
                "trace": [{"module": "stdlib", "version": "v1.26.9"}],
            }
        }
        decision = go_floor_watch.decide("1.26.9", "1.26.10", govuln(X509, newer))
        self.assertEqual(decision["target"], "1.26.12")

    def test_unfixed_stdlib_range_is_reported_without_a_target(self) -> None:
        payload = govuln(
            {
                "osv": {
                    "id": "GO-2099-1111",
                    "affected": [
                        {
                            "package": {"name": "stdlib"},
                            "ranges": [
                                {"type": "SEMVER", "events": [{"introduced": "1.26.0"}]}
                            ],
                        }
                    ],
                }
            }
        )
        decision = go_floor_watch.decide("1.26.10", "1.26.10", payload)
        self.assertIsNone(decision["target"])
        self.assertEqual(decision["unfixed"], ["GO-2099-1111"])
        self.assertEqual(decision["status"], "behind")

    def test_main_exit_codes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            floor = root / ".go-version"
            press = root / "go_version.go"
            vuln = root / "govuln.json"
            floor.write_text("1.26.9\n")
            press.write_text('const librarySafeGoDirective = "1.26.10"\n')
            vuln.write_text(govuln(X509))
            args = [
                "--floor-file",
                str(floor),
                "--press-file",
                str(press),
                "--press-tag",
                "v4.33.9",
                "--govuln-json",
                str(vuln),
                "--canary",
                "library/health/grants/cmd/grants-pp-cli",
            ]
            with redirect_stdout(io.StringIO()):
                behind = go_floor_watch.main(args)
            self.assertEqual(behind, 1)
            floor.write_text("1.26.10\n")
            with redirect_stdout(io.StringIO()):
                current = go_floor_watch.main(args)
            self.assertEqual(current, 0)


if __name__ == "__main__":
    unittest.main()
