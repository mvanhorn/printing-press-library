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
        problems = verifier.validate_cli_dir(cli_dir, strict=True, changed_files=None)
        messages = [p.message for p in problems]

        self.assertTrue(any("AGENTS.md" in msg for msg in messages))
        self.assertTrue(any(".printing-press-patches.json" in msg for msg in messages))
        self.assertTrue(any("run_id" in msg for msg in messages))

    def test_valid_new_cli_and_pr_body_has_no_suggestions(self) -> None:
        self.write_valid_cli()
        self.git("add", ".")
        self.git("commit", "-m", "add example")

        touched, files_by_dir = verifier.changed_cli_dirs(self.base)
        new_dirs = [d for d in touched if verifier.is_new_cli(self.base, d)]
        body = "### Publication Path\nnew print\n\n### Novel Commands\n- search\n"
        problems = []
        for cli_dir in touched:
            problems.extend(
                verifier.validate_cli_dir(
                    cli_dir,
                    strict=cli_dir in new_dirs,
                    changed_files=files_by_dir.get(cli_dir, set()),
                )
            )
        suggestions = verifier.pr_body_suggestions(body, new_dirs)

        self.assertEqual([], problems)
        self.assertEqual([], suggestions)

    def test_missing_pr_body_sections_are_advisory_for_new_cli(self) -> None:
        self.write_valid_cli()
        self.git("add", ".")
        self.git("commit", "-m", "add example")

        touched, _ = verifier.changed_cli_dirs(self.base)
        new_dirs = [d for d in touched if verifier.is_new_cli(self.base, d)]
        suggestions = verifier.pr_body_suggestions("", new_dirs)

        self.assertEqual(1, len(suggestions))
        self.assertIn("### Novel Commands", suggestions[0])
        self.assertIn("### Publication Path", suggestions[0])
        self.assertIn("| `search` | Example search | Searches example data. |", suggestions[0])

    def test_new_cli_directory_with_pp_cli_suffix_fails(self) -> None:
        cli_dir = self.tmp / "library" / "cloud" / "example-pp-cli"
        manifest = {
            "schema_version": 1,
            "api_name": "example-pp-cli",
            "category": "cloud",
            "cli_name": "example-pp-cli",
            "printer": "tmchow",
            "printing_press_version": "4.0.1",
            "run_id": "20260509T010203Z-test",
            "novel_features": [{"name": "n", "command": "search", "description": "d"}],
        }
        files = {
            ".printing-press.json": json.dumps(manifest),
            ".printing-press-patches.json": json.dumps({"schema_version": 1, "applied_at": "2026-05-09", "patches": []}),
            "AGENTS.md": "# Agents\n",
            "README.md": "# Example\n",
            "SKILL.md": "---\nname: pp-example\n---\n",
            "go.mod": "module github.com/mvanhorn/printing-press-library/library/cloud/example-pp-cli\n",
            ".goreleaser.yaml": "version: 2\n",
            "LICENSE": "MIT\n",
            "NOTICE": "Example\n",
            "cmd/example-pp-cli/main.go": "package main\n",
            ".manuscripts/20260509T010203Z-test/research/research.json": "{}\n",
            ".manuscripts/20260509T010203Z-test/proofs/shipcheck.json": "{}\n",
        }
        for name, content in files.items():
            self.write(f"library/cloud/example-pp-cli/{name}", content)

        problems = verifier.validate_cli_dir(cli_dir, strict=True, changed_files=None)
        messages = [p.message for p in problems]

        self.assertTrue(any("-pp-cli/-pp-mcp binary suffix" in msg for msg in messages))

    def test_existing_cli_with_pp_cli_suffix_does_not_fail_when_non_strict(self) -> None:
        cli_dir = self.tmp / "library" / "cloud" / "legacy-pp-cli"
        manifest = {
            "schema_version": 1,
            "api_name": "legacy-pp-cli",
            "category": "cloud",
            "cli_name": "legacy-pp-cli",
        }
        self.write("library/cloud/legacy-pp-cli/.printing-press.json", json.dumps(manifest))
        self.write("library/cloud/legacy-pp-cli/cmd/legacy-pp-cli/main.go", "package main\n")

        problems = verifier.validate_cli_dir(cli_dir, strict=False, changed_files=set())
        messages = [p.message for p in problems]

        self.assertFalse(any("-pp-cli/-pp-mcp binary suffix" in msg for msg in messages))

    def test_readme_only_touch_skips_unrelated_patch_manifest(self) -> None:
        """A docs-only PR that touches README.md must not re-validate a
        pre-existing patches manifest whose state didn't change in this PR.
        """
        cli_dir = self.tmp / "library" / "cloud" / "legacy"
        # Seed pre-existing state on the base ref (main): a patches manifest
        # that references a source file missing a `// PATCH:` marker. This
        # mirrors the real digitalocean/fireflies inconsistency we're guarding
        # against.
        self.git("checkout", "-B", "scenario-main", self.base)
        patch_manifest = {
            "schema_version": 1,
            "applied_at": "2026-05-09",
            "patches": [
                {
                    "id": "old-customization",
                    "summary": "Historic tweak",
                    "reason": "Pre-existing state.",
                    "files": ["internal/cli/legacy.go"],
                }
            ],
        }
        self.write(
            "library/cloud/legacy/.printing-press.json",
            json.dumps({"schema_version": 1, "api_name": "legacy", "cli_name": "legacy-pp-cli"}),
        )
        self.write("library/cloud/legacy/.printing-press-patches.json", json.dumps(patch_manifest))
        self.write("library/cloud/legacy/internal/cli/legacy.go", "package cli\n")
        self.write("library/cloud/legacy/README.md", "# Legacy\n")
        self.git("add", ".")
        self.git("commit", "-m", "seed legacy CLI on main")
        new_base = self.git("rev-parse", "HEAD").stdout.strip()
        self.git("switch", "feature")
        self.git("merge", "--ff-only", "scenario-main")
        # Now bump only the README on the feature branch.
        self.write("library/cloud/legacy/README.md", "# Legacy v2\n")
        self.git("add", ".")
        self.git("commit", "-m", "docs: bump legacy README")

        touched, files_by_dir = verifier.changed_cli_dirs(new_base)
        self.assertEqual([cli_dir], touched)
        problems = verifier.validate_cli_dir(
            cli_dir,
            strict=False,
            changed_files=files_by_dir.get(cli_dir, set()),
        )

        self.assertFalse(
            any("PATCH marker" in p.message for p in problems),
            msg=f"docs-only touch should skip patch-manifest validation; got {[p.message for p in problems]}",
        )

    def test_touched_patches_file_triggers_validation(self) -> None:
        """If the patches manifest itself changes in the PR, validation runs
        even on an existing CLI (non-strict).
        """
        cli_dir = self.tmp / "library" / "cloud" / "legacy"
        self.git("checkout", "-B", "scenario-main", self.base)
        self.write(
            "library/cloud/legacy/.printing-press.json",
            json.dumps({"schema_version": 1, "api_name": "legacy", "cli_name": "legacy-pp-cli"}),
        )
        self.write("library/cloud/legacy/internal/cli/legacy.go", "package cli\n")
        self.git("add", ".")
        self.git("commit", "-m", "seed legacy CLI without patches")
        new_base = self.git("rev-parse", "HEAD").stdout.strip()
        self.git("switch", "feature")
        self.git("merge", "--ff-only", "scenario-main")

        patch_manifest = {
            "schema_version": 1,
            "applied_at": "2026-05-09",
            "patches": [
                {
                    "id": "added-now",
                    "summary": "New tweak",
                    "reason": "Reason.",
                    "files": ["internal/cli/legacy.go"],
                }
            ],
        }
        self.write("library/cloud/legacy/.printing-press-patches.json", json.dumps(patch_manifest))
        self.git("add", ".")
        self.git("commit", "-m", "add patches manifest")

        _, files_by_dir = verifier.changed_cli_dirs(new_base)
        problems = verifier.validate_cli_dir(
            cli_dir,
            strict=False,
            changed_files=files_by_dir.get(cli_dir, set()),
        )

        self.assertTrue(
            any("PATCH marker" in p.message for p in problems),
            msg=f"patches manifest changed → marker check should fire; got {[p.message for p in problems]}",
        )

    def test_marker_added_with_empty_patches_array_is_caught(self) -> None:
        """Regression for greptile-flagged scope-guard bug on #587: if a PR
        adds a // PATCH: marker to a source file but the CLI's
        .printing-press-patches.json has patches: [], the prior scope guard
        only added the manifest path to relevant_paths so the diff (which
        touched only the .go file) bypassed the check entirely. Validation
        must still fire and report markers-without-manifest.
        """
        cli_dir = self.tmp / "library" / "cloud" / "legacy"
        # Seed pre-existing state on main: empty-patches manifest +
        # marker-free source file.
        self.git("checkout", "-B", "scenario-main", self.base)
        self.write(
            "library/cloud/legacy/.printing-press.json",
            json.dumps({"schema_version": 1, "api_name": "legacy", "cli_name": "legacy-pp-cli"}),
        )
        self.write(
            "library/cloud/legacy/.printing-press-patches.json",
            json.dumps({"schema_version": 1, "applied_at": "2026-05-09", "patches": []}),
        )
        self.write("library/cloud/legacy/internal/cli/legacy.go", "package cli\n")
        self.git("add", ".")
        self.git("commit", "-m", "seed legacy CLI with empty patches[]")
        new_base = self.git("rev-parse", "HEAD").stdout.strip()
        self.git("switch", "feature")
        self.git("merge", "--ff-only", "scenario-main")

        # PR adds a PATCH marker to the Go file without touching the manifest.
        self.write(
            "library/cloud/legacy/internal/cli/legacy.go",
            "// PATCH: tweak request envelope\npackage cli\n",
        )
        self.git("add", ".")
        self.git("commit", "-m", "add patch marker but forget manifest entry")

        _, files_by_dir = verifier.changed_cli_dirs(new_base)
        problems = verifier.validate_cli_dir(
            cli_dir,
            strict=False,
            changed_files=files_by_dir.get(cli_dir, set()),
        )
        self.assertTrue(
            any("PATCH markers but patches[] is empty" in p.message for p in problems),
            msg=f"empty patches[] + new marker should fire; got {[p.message for p in problems]}",
        )

    def test_patch_entry_with_only_non_go_files_skips_marker_check(self) -> None:
        """JSON/YAML-only customizations (e.g. spec.json redaction, schema
        bump in .printing-press.json) record the patch in the manifest but
        can't carry an inline // PATCH: comment. The manifest entry IS the
        marker."""
        cli_dir = self.tmp / "library" / "cloud" / "legacy"
        patch_manifest = {
            "schema_version": 1,
            "applied_at": "2026-05-09",
            "patches": [
                {
                    "id": "spec-redaction",
                    "summary": "Redacted example tokens in spec.",
                    "reason": "Push-protection.",
                    "files": ["spec.json"],
                }
            ],
        }
        self.write(
            "library/cloud/legacy/.printing-press.json",
            json.dumps({"schema_version": 1, "api_name": "legacy", "cli_name": "legacy-pp-cli"}),
        )
        self.write("library/cloud/legacy/.printing-press-patches.json", json.dumps(patch_manifest))
        self.write("library/cloud/legacy/spec.json", '{"openapi": "3.0.0"}\n')

        problems = verifier.validate_patch_manifest(cli_dir, changed_files=None)
        self.assertFalse(
            any("PATCH marker" in p.message for p in problems),
            msg=f"JSON-only patch should skip marker check; got {[p.message for p in problems]}",
        )


class HasPatchMarkerTest(unittest.TestCase):
    """Unit tests for ``has_patch_marker`` to ensure only the documented
    ``// PATCH:`` / ``// PATCH(...)`` comment convention is detected, not bare
    HTTP method literals or other coincidental occurrences of the word.
    """

    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="has-patch-marker-"))
        self.addCleanup(lambda: shutil.rmtree(self.tmp))

    def _write(self, name: str, body: str) -> Path:
        path = self.tmp / name
        path.write_text(body)
        return path

    def test_detects_real_patch_marker(self) -> None:
        path = self._write(
            "real.go",
            "// PATCH: align response envelope with upstream\nfunc foo() {}\n",
        )
        self.assertTrue(verifier.has_patch_marker(path))

    def test_detects_patch_marker_with_upstream_ref(self) -> None:
        path = self._write(
            "real.go",
            "    // PATCH(upstream cli-printing-press#842): auto-fill AccountSid\n",
        )
        self.assertTrue(verifier.has_patch_marker(path))

    def test_ignores_http_method_string_literal(self) -> None:
        path = self._write(
            "client.go",
            'return c.do("PATCH", path, nil, body, nil)\n',
        )
        self.assertFalse(verifier.has_patch_marker(path))

    def test_ignores_makeAPIHandler_PATCH(self) -> None:
        path = self._write(
            "tools.go",
            'makeAPIHandler("PATCH", "/conversations/{id}", bindings, positional)\n',
        )
        self.assertFalse(verifier.has_patch_marker(path))

    def test_ignores_switch_case_PATCH(self) -> None:
        path = self._write(
            "tools.go",
            'switch method {\ncase "POST", "PUT", "PATCH":\n    return body\n}\n',
        )
        self.assertFalse(verifier.has_patch_marker(path))

    def test_ignores_annotation_map_value(self) -> None:
        path = self._write(
            "cmd.go",
            'Annotations: map[string]string{"pp:method": "PATCH", "pp:path": "/x"},\n',
        )
        self.assertFalse(verifier.has_patch_marker(path))

    def test_ignores_word_PATCH_in_string_or_comment(self) -> None:
        path = self._write(
            "doc.go",
            "// This handler issues HTTP PATCH requests against the upstream API.\n",
        )
        # Plain prose comment without the colon/paren marker shape is not a
        # customization marker.
        self.assertFalse(verifier.has_patch_marker(path))

    def test_detects_marker_with_inline_patch_id(self) -> None:
        # The `// PATCH <id>:` form is in use across pre-existing CLIs
        # (e.g. openrouter, fireflies) and mirrors the `id` field shape from
        # .printing-press-patches.json. Treat it as a documented third form.
        path = self._write(
            "marked.go",
            "// PATCH mcp-http-transport: added --http :addr flag\n",
        )
        self.assertTrue(verifier.has_patch_marker(path))

    def test_detects_marker_with_inline_patch_id_and_parens(self) -> None:
        path = self._write(
            "marked.go",
            "// PATCH transcendence-commands(upstream cli-printing-press#825): hand-built\n",
        )
        self.assertTrue(verifier.has_patch_marker(path))


if __name__ == "__main__":
    unittest.main()
