#!/usr/bin/env python3
from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path

import verify_press_version as verifier


class PressVersionVerifierTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="verify-press-version-"))
        self.addCleanup(lambda: shutil.rmtree(self.tmp))
        self.old_root = verifier.REPO_ROOT
        verifier.REPO_ROOT = self.tmp
        self.git("init", "-q")
        self.git("config", "user.email", "test@example.com")
        self.git("config", "user.name", "Test User")

    def tearDown(self) -> None:
        verifier.REPO_ROOT = self.old_root

    def git(self, *args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["git", *args],
            cwd=self.tmp,
            check=True,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    def write(self, rel: str, content: str) -> None:
        path = self.tmp / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def write_manifest(self, root: str, version: object) -> None:
        self.write(
            f"{root}/.printing-press.json",
            json.dumps(
                {
                    "api_name": Path(root).name,
                    "cli_name": f"{Path(root).name}-pp-cli",
                    "printing_press_version": version,
                }
            ),
        )

    def run_quiet(self, base: str) -> int:
        with redirect_stdout(StringIO()):
            return verifier.run(base)

    def commit_base(self) -> str:
        self.write_manifest("library/cloud/old", "4.0.0")
        self.write("library/cloud/old/README.md", "# Old\n")
        self.write("README.md", "# Repo\n")
        self.git("add", ".")
        self.git("commit", "-m", "base")
        return self.git("rev-parse", "HEAD").stdout.strip()

    def test_non_library_pr_ignores_old_baseline_manifests(self) -> None:
        base = self.commit_base()
        self.git("switch", "-c", "feature")
        self.write(".github/workflows/verify-library-conventions.yml", "name: Verify\n")
        self.git("add", ".")
        self.git("commit", "-m", "update workflow")

        self.assertEqual(0, self.run_quiet(base))

    def test_touched_existing_cli_with_old_press_version_passes(self) -> None:
        base = self.commit_base()
        self.git("switch", "-c", "feature")
        self.write("library/cloud/old/README.md", "# Updated\n")
        self.git("add", ".")
        self.git("commit", "-m", "update old cli")

        self.assertEqual(0, self.run_quiet(base))

    def test_new_cli_with_old_press_version_fails(self) -> None:
        base = self.commit_base()
        self.git("switch", "-c", "feature")
        self.write_manifest("library/cloud/new", "4.9.9")
        self.write("library/cloud/new/README.md", "# New\n")
        self.git("add", ".")
        self.git("commit", "-m", "add stale cli")

        self.assertEqual(1, self.run_quiet(base))

    def test_new_cli_with_current_press_version_passes(self) -> None:
        base = self.commit_base()
        self.git("switch", "-c", "feature")
        self.write_manifest("library/cloud/new", "4.10.0")
        self.write("library/cloud/new/README.md", "# New\n")
        self.git("add", ".")
        self.git("commit", "-m", "add current cli")

        self.assertEqual(0, self.run_quiet(base))

    def test_v_prefixed_newer_version_passes(self) -> None:
        base = self.commit_base()
        self.git("switch", "-c", "feature")
        self.write_manifest("library/cloud/newer", "v4.11.0")
        self.write("library/cloud/newer/README.md", "# Newer\n")
        self.git("add", ".")
        self.git("commit", "-m", "add newer cli")

        self.assertEqual(0, self.run_quiet(base))

    def test_missing_version_fails_with_upgrade_guidance(self) -> None:
        cli_dir = self.tmp / "library" / "cloud" / "missing"
        self.write_manifest("library/cloud/missing", "")

        problems = verifier.validate_cli_dir(cli_dir)

        self.assertEqual(1, len(problems))
        self.assertIn("go install github.com/mvanhorn/cli-printing-press/v4/cmd/cli-printing-press@latest", problems[0].message)
        self.assertIn("/cli-printing-press-publish", problems[0].message)

    def test_absent_version_key_reports_not_set(self) -> None:
        cli_dir = self.tmp / "library" / "cloud" / "missing-key"
        self.write("library/cloud/missing-key/.printing-press.json", json.dumps({"api_name": "missing-key"}))

        problems = verifier.validate_cli_dir(cli_dir)

        self.assertEqual(1, len(problems))
        self.assertIn("printing_press_version is not set", problems[0].message)

    def test_unparseable_version_reports_invalid_string(self) -> None:
        cli_dir = self.tmp / "library" / "cloud" / "bad-version"
        self.write_manifest("library/cloud/bad-version", "4.10")

        problems = verifier.validate_cli_dir(cli_dir)

        self.assertEqual(1, len(problems))
        self.assertIn("printing_press_version '4.10' is not a valid version string", problems[0].message)


if __name__ == "__main__":
    unittest.main()
