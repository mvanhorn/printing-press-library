#!/usr/bin/env python3
"""Tests for scripts/verify_go_floor.py."""

from __future__ import annotations

import importlib.util
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


def _load_verify_go_floor():
    path = Path(__file__).with_name("verify-go-floor.py")
    spec = importlib.util.spec_from_file_location("verify_go_floor", path)
    if spec is None or spec.loader is None:
        raise ImportError("could not load verify-go-floor.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules["verify_go_floor"] = module
    spec.loader.exec_module(module)
    return module


verify_go_floor = _load_verify_go_floor()


PRESS_SOURCE = 'package generator\n\nconst librarySafeGoDirective = "%s"\n'


def write_repo(root: Path, floor: str = "1.26.10", toolchain: str | None = None) -> None:
    (root / ".go-version").write_text(floor + "\n")
    signals = root / ".github/scripts/verify-supply-chain"
    signals.mkdir(parents=True, exist_ok=True)
    (signals / "signals.py").write_text('DEFAULT_GO_FLOOR = "%s"\n' % floor)
    sweep = root / "tools/sweep-canonical"
    sweep.mkdir(parents=True, exist_ok=True)
    (sweep / "main.go").write_text(
        "func readRepoGoVersion() (string, error) {\n"
        '\tcandidate := filepath.Join(dir, ".go-version")\n'
        "\treturn version, nil\n"
        "}\n"
    )
    (root / "README.md").write_text("Install with Go %s or newer.\n" % floor)
    npm = root / "npm"
    npm.mkdir(exist_ok=True)
    (npm / "README.md").write_text(
        "- The Go release recorded in `.go-version`, or newer\n"
    )
    workflows = root / ".github/workflows"
    workflows.mkdir(parents=True, exist_ok=True)
    (workflows / "ci.yml").write_text(
        "jobs:\n  test:\n    steps:\n"
        "      - uses: actions/setup-go@v6\n"
        "        with:\n"
        "          go-version-file: .go-version\n"
        "      - name: Pin CI Go to .go-version\n"
        '        run: echo "GOTOOLCHAIN=local" >> "$GITHUB_ENV"\n'
    )
    (workflows / "verify-ordertogo-windows.yml").write_text(
        "jobs:\n  test:\n    steps:\n"
        "      - uses: actions/setup-go@v6\n"
        "        with:\n"
        "          go-version-file: library/food-and-dining/ordertogo/go.mod\n"
        "      - name: Pin CI Go to .go-version\n"
        "        shell: bash\n"
        '        run: echo "GOTOOLCHAIN=local" >> "$GITHUB_ENV"\n'
    )
    if toolchain is not None:
        module = root / "library/travel/example"
        module.mkdir(parents=True, exist_ok=True)
        (module / "go.mod").write_text("module example\n\ngo 1.26.6\n\ntoolchain go%s\n" % toolchain)


class VerifyGoFloorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def test_matching_surfaces_pass(self) -> None:
        write_repo(self.root)
        failures, warnings = verify_go_floor.evaluate(
            self.root, PRESS_SOURCE % "1.26.10", None
        )
        self.assertEqual(failures, [])
        self.assertEqual(warnings, [])

    def test_default_floor_mismatch_fails(self) -> None:
        write_repo(self.root)
        path = self.root / ".github/scripts/verify-supply-chain/signals.py"
        path.write_text('DEFAULT_GO_FLOOR = "1.26.9"\n')
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any("DEFAULT_GO_FLOOR" in failure for failure in failures))

    def test_readme_mismatch_fails(self) -> None:
        write_repo(self.root)
        (self.root / "README.md").write_text("Install with Go 1.26.5 or newer.\n")
        (self.root / "npm/README.md").write_text("- Go 1.26.6 or newer\n")
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any(failure.startswith("README.md") for failure in failures))
        self.assertTrue(any(failure.startswith("npm/README.md") for failure in failures))

    def test_toolchain_below_floor_fails_and_equal_or_newer_passes(self) -> None:
        write_repo(self.root, toolchain="1.26.9")
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any("toolchain go1.26.9" in failure for failure in failures))

        write_repo(self.root, toolchain="1.26.10")
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertEqual(failures, [])

        write_repo(self.root, toolchain="1.27.3")
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertEqual(failures, [])

    def test_missing_toolchain_line_passes(self) -> None:
        write_repo(self.root)
        module = self.root / "library/health/grants"
        module.mkdir(parents=True, exist_ok=True)
        (module / "go.mod").write_text("module example\n\ngo 1.26.6\n")
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertEqual(failures, [])

    def test_press_ahead_of_floor_fails(self) -> None:
        write_repo(self.root)
        failures, warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.11", None)
        self.assertEqual(warnings, [])
        self.assertTrue(any("behind press" in failure for failure in failures))

    def test_press_fetch_failure_warns(self) -> None:
        write_repo(self.root)
        failures, warnings = verify_go_floor.evaluate(self.root, None, "timed out")
        self.assertEqual(failures, [])
        self.assertEqual(len(warnings), 1)
        self.assertIn("timed out", warnings[0])

    def test_hardcoded_sweep_floor_fails_when_not_derived(self) -> None:
        write_repo(self.root)
        (self.root / "tools/sweep-canonical/main.go").write_text(
            'const minimumGoVersion = "Go 1.26.6 or newer"\n'
        )
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any("sweep-canonical" in failure for failure in failures))

    def test_npm_readme_without_floor_reference_fails(self) -> None:
        write_repo(self.root)
        (self.root / "npm/README.md").write_text("- Node.js 20+\n")
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any(failure.startswith("npm/README.md") for failure in failures))

    def test_stale_npm_literal_fails_when_go_version_is_also_named(self) -> None:
        write_repo(self.root)
        (self.root / "npm/README.md").write_text(
            "- Go 1.26.6 or newer (see `.go-version`)\n"
        )
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any("npm/README.md says Go 1.26.6" in failure for failure in failures))

    def test_setup_go_pin_is_per_job(self) -> None:
        write_repo(self.root)
        (self.root / ".github/workflows/ci.yml").write_text(
            "jobs:\n"
            "  pinned:\n"
            "    steps:\n"
            "      - uses: actions/setup-go@v6\n"
            "        with:\n"
            "          go-version-file: .go-version\n"
            "      - name: Pin CI Go to .go-version\n"
            '        run: echo "GOTOOLCHAIN=local" >> "$GITHUB_ENV"\n'
            "  unpinned:\n"
            "    steps:\n"
            "      - uses: actions/setup-go@v6\n"
            "        with:\n"
            "          go-version-file: .go-version\n"
        )
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any("job unpinned" in failure for failure in failures))
        self.assertFalse(any("job pinned" in failure for failure in failures))

    def test_pin_before_setup_go_does_not_count(self) -> None:
        write_repo(self.root)
        (self.root / ".github/workflows/ci.yml").write_text(
            "jobs:\n"
            "  test:\n"
            "    steps:\n"
            "      - name: Pin CI Go to .go-version\n"
            '        run: echo "GOTOOLCHAIN=local" >> "$GITHUB_ENV"\n'
            "      - uses: actions/setup-go@v6\n"
            "        with:\n"
            "          go-version-file: .go-version\n"
        )
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any("job test" in failure and "GOTOOLCHAIN=local" in failure for failure in failures))

    def test_both_setup_go_jobs_pass_when_each_is_pinned(self) -> None:
        write_repo(self.root)
        (self.root / ".github/workflows/ci.yml").write_text(
            "jobs:\n"
            "  bundle:\n"
            "    steps:\n"
            "      - uses: actions/setup-go@v6\n"
            "        with:\n"
            "          go-version-file: .go-version\n"
            "      - name: Pin CI Go to .go-version\n"
            '        run: echo "GOTOOLCHAIN=local" >> "$GITHUB_ENV"\n'
            "  cli:\n"
            "    steps:\n"
            "      - uses: actions/setup-go@v6\n"
            "        with:\n"
            "          go-version-file: .go-version\n"
            "      - name: Pin CI Go to .go-version\n"
            '        run: echo "GOTOOLCHAIN=local" >> "$GITHUB_ENV"\n'
        )
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertEqual(failures, [])

    def test_renovate_groups_the_floor_surfaces(self) -> None:
        config_path = Path(__file__).resolve().parents[1] / "renovate.json"
        config = json.loads(config_path.read_text())
        patterns = [
            pattern
            for manager in config["customManagers"]
            for pattern in manager["managerFilePatterns"]
        ]
        joined = "\n".join(patterns)
        self.assertIn(r"\.go-version", joined)
        self.assertIn(r"signals\.py", joined)
        self.assertIn(r"README\.md", joined)
        self.assertNotIn("npm", joined)
        self.assertEqual(config["enabledManagers"], ["custom.regex"])
        self.assertIn("go-floor", [rule.get("groupName") for rule in config["packageRules"]])
        patch_rules = [
            rule for rule in config["packageRules"] if rule.get("matchUpdateTypes") == ["patch"]
        ]
        self.assertEqual(len(patch_rules), 1)
        self.assertTrue(patch_rules[0].get("automerge"))
        disabled = [rule for rule in config["packageRules"] if rule.get("enabled") is False]
        self.assertTrue(any(rule.get("matchUpdateTypes") == ["minor", "major"] for rule in disabled))

    def test_setup_go_without_local_toolchain_fails(self) -> None:
        write_repo(self.root)
        (self.root / ".github/workflows/ci.yml").write_text(
            "jobs:\n  test:\n    steps:\n"
            "      - uses: actions/setup-go@v6\n"
            "        with:\n"
            "          go-version: '1.26.10'\n"
        )
        failures, _warnings = verify_go_floor.evaluate(self.root, PRESS_SOURCE % "1.26.10", None)
        self.assertTrue(any("GOTOOLCHAIN=local" in failure for failure in failures))
        self.assertTrue(any("go-version-file: .go-version" in failure for failure in failures))

    def test_main_warns_and_passes_when_fetch_fails(self) -> None:
        write_repo(self.root)
        with mock.patch.object(verify_go_floor, "REPO_ROOT", self.root):
            with mock.patch.object(
                verify_go_floor, "fetch_press_source", side_effect=TimeoutError("timed out")
            ):
                stderr = io.StringIO()
                with mock.patch.object(verify_go_floor.sys, "stderr", stderr):
                    rc = verify_go_floor.main()
        self.assertEqual(rc, 0)
        self.assertIn("warning:", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
