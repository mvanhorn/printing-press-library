#!/usr/bin/env python3
from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

import verify_publish_package as verifier


class PublishPackageVerifierTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="verify-publish-package-"))
        self.addCleanup(lambda: shutil.rmtree(self.tmp))
        self.old_root = verifier.REPO_ROOT
        verifier.REPO_ROOT = self.tmp
        self.git("init", "-q")
        self.git("config", "user.email", "test@example.com")
        self.git("config", "user.name", "Test User")
        self.git("commit", "--allow-empty", "-m", "base")
        self.base = self.git("rev-parse", "HEAD").stdout.strip()
        self.git("switch", "-c", "feature")

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

    def write(self, rel: str, content: str = "") -> None:
        path = self.tmp / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def write_valid_cli(self) -> Path:
        cli_dir = self.tmp / "library" / "cloud" / "example"
        manifest = {
            "schema_version": 1,
            "api_name": "example",
            "category": "cloud",
            "cli_name": "example-pp-cli",
            "printer": "tmchow",
            "printing_press_version": "4.0.1",
            "run_id": "20260509T010203Z-test",
            "mcp_binary": "example-pp-mcp",
            "mcp_tool_count": 1,
            "novel_features": [
                {
                    "name": "Example search",
                    "command": "search",
                    "description": "Searches example data.",
                }
            ],
        }
        patch_manifest = {"schema_version": 1, "applied_at": "2026-05-09", "patches": []}
        files = {
            ".printing-press.json": json.dumps(manifest),
            ".printing-press-patches.json": json.dumps(patch_manifest),
            "AGENTS.md": "# Agents\n",
            "README.md": "# Example\n",
            "SKILL.md": "---\nname: pp-example\n---\n",
            "go.mod": "module github.com/mvanhorn/printing-press-library/library/cloud/example\n",
            ".goreleaser.yaml": "version: 2\n",
            "LICENSE": "MIT\n",
            "NOTICE": "Example\n",
            "manifest.json": "{}\n",
            "tools-manifest.json": "{}\n",
            "cmd/example-pp-cli/main.go": "package main\n",
            "cmd/example-pp-mcp/main.go": "package main\n",
            ".manuscripts/20260509T010203Z-test/research/research.json": "{}\n",
            ".manuscripts/20260509T010203Z-test/proofs/shipcheck.json": "{}\n",
        }
        for name, content in files.items():
            self.write(f"library/cloud/example/{name}", content)
        return cli_dir

    def test_new_cli_missing_publish_artifacts_fails(self) -> None:
        self.write("library/cloud/bad/.printing-press.json", '{"api_name": "bad", "cli_name": "bad-pp-cli"}')
        self.write("library/cloud/bad/go.mod", "module github.com/mvanhorn/printing-press-library/library/cloud/bad\n")

        cli_dir = self.tmp / "library" / "cloud" / "bad"
        problems = verifier.validate_cli_dir(cli_dir, strict=True)
        messages = [p.message for p in problems]

        self.assertTrue(any("AGENTS.md" in msg for msg in messages))
        self.assertTrue(any(".printing-press-patches.json" in msg for msg in messages))
        self.assertTrue(any("run_id" in msg for msg in messages))

    def test_valid_new_cli_and_pr_body_has_no_suggestions(self) -> None:
        self.write_valid_cli()
        self.git("add", ".")
        self.git("commit", "-m", "add example")

        touched = verifier.changed_cli_dirs(self.base)
        new_dirs = [d for d in touched if verifier.is_new_cli(self.base, d)]
        body = "### Publication Path\nnew print\n\n### Novel Commands\n- search\n"
        problems = []
        for cli_dir in touched:
            problems.extend(verifier.validate_cli_dir(cli_dir, strict=cli_dir in new_dirs))
        suggestions = verifier.pr_body_suggestions(body, new_dirs)

        self.assertEqual([], problems)
        self.assertEqual([], suggestions)

    def test_missing_pr_body_sections_are_advisory_for_new_cli(self) -> None:
        self.write_valid_cli()
        self.git("add", ".")
        self.git("commit", "-m", "add example")

        touched = verifier.changed_cli_dirs(self.base)
        new_dirs = [d for d in touched if verifier.is_new_cli(self.base, d)]
        suggestions = verifier.pr_body_suggestions("", new_dirs)

        self.assertEqual(1, len(suggestions))
        self.assertIn("### Novel Commands", suggestions[0])
        self.assertIn("### Publication Path", suggestions[0])
        self.assertIn("| `search` | Example search | Searches example data. |", suggestions[0])


if __name__ == "__main__":
    unittest.main()
