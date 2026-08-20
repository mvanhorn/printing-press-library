#!/usr/bin/env python3
from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

import verify_attribution as verifier


class AttributionVerifierTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="verify-attribution-"))
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

    def write_manifest(self, root: str, **overrides: object) -> None:
        data = {
            "api_name": Path(root).name,
            "cli_name": f"{Path(root).name}-pp-cli",
            "printer": "tmchow",
            "printer_name": "Trevin Chow",
        }
        data.update(overrides)
        self.write(f"{root}/.printing-press.json", json.dumps(data))

    def commit_base_with_stale_manifest(self) -> str:
        self.write_manifest("library/search/google-search-console", printer_name="")
        self.write("library/search/google-search-console/README.md", "# Google Search Console\n")
        self.git("add", ".")
        self.git("commit", "-m", "base")
        return self.git("rev-parse", "HEAD").stdout.strip()

    def test_non_library_pr_ignores_unrelated_baseline_attribution_gaps(self) -> None:
        base = self.commit_base_with_stale_manifest()
        self.git("switch", "-c", "feature")
        self.write(".github/workflows/verify-library-conventions.yml", "name: Verify\n")
        self.git("add", ".")
        self.git("commit", "-m", "update workflow")

        self.assertEqual(0, verifier.run(base, "HEAD"))

    def test_touched_existing_cli_still_requires_attribution(self) -> None:
        base = self.commit_base_with_stale_manifest()
        self.git("switch", "-c", "feature")
        self.write("library/search/google-search-console/README.md", "# Updated\n")
        self.git("add", ".")
        self.git("commit", "-m", "update cli docs")

        self.assertEqual(1, verifier.run(base, "HEAD"))

    def test_new_cli_allows_printer_equal_to_owner(self) -> None:
        self.write("README.md", "# repo\n")
        self.git("add", ".")
        self.git("commit", "-m", "base")
        base = self.git("rev-parse", "HEAD").stdout.strip()

        self.git("switch", "-c", "feature")
        self.write_manifest(
            "library/other/openalex",
            owner="hiten-shah",
            printer="hiten-shah",
            printer_name="Hiten Shah",
        )
        self.write("library/other/openalex/README.md", "# OpenAlex\n")
        self.git("add", ".")
        self.git("commit", "-m", "add openalex")

        self.assertEqual(0, verifier.run(base, "HEAD"))


class SkillAuthorParsingTest(unittest.TestCase):
    PLAIN = '---\nname: pp-foo\nauthor: "Ada Lovelace"\n---\n\n# Foo\n'

    def test_plain_frontmatter_author(self) -> None:
        self.assertEqual("Ada Lovelace", verifier.skill_author(self.PLAIN))

    def test_leading_bom_does_not_hide_author(self) -> None:
        # A SKILL.md with a leading UTF-8 BOM must resolve to the same author as
        # the BOM-free file, so stripping the BOM is not read as an attribution flip.
        bom = "\ufeff" + self.PLAIN
        self.assertEqual("Ada Lovelace", verifier.skill_author(bom))
        self.assertEqual(verifier.skill_author(bom), verifier.skill_author(self.PLAIN))

    def test_leading_comment_does_not_hide_author(self) -> None:
        commented = "<!-- // PATCH: hand-edited headline -->\n" + self.PLAIN
        self.assertEqual("Ada Lovelace", verifier.skill_author(commented))
        self.assertEqual(verifier.skill_author(commented), verifier.skill_author(self.PLAIN))

    def test_bom_then_comment_does_not_hide_author(self) -> None:
        noisy = "\ufeff<!-- // PATCH: note -->\n" + self.PLAIN
        self.assertEqual("Ada Lovelace", verifier.skill_author(noisy))
        self.assertEqual(verifier.skill_author(noisy), verifier.skill_author(self.PLAIN))

    def test_multiple_leading_comments_do_not_hide_author(self) -> None:
        noisy = "<!-- generated header -->\n<!-- // PATCH: note -->\n" + self.PLAIN
        self.assertEqual("Ada Lovelace", verifier.skill_author(noisy))
        self.assertEqual(verifier.skill_author(noisy), verifier.skill_author(self.PLAIN))

    def test_no_frontmatter_still_returns_none(self) -> None:
        self.assertIsNone(verifier.skill_author("# Foo\n\nNo frontmatter here.\n"))

    def test_attribution_only_correction_on_bom_file_is_not_a_surface_change(self) -> None:
        # Same BOM on both sides, only the author value changes: normalize must
        # report no surface change so an attribution-only fix isn't gated.
        before = "\ufeff" + self.PLAIN
        after = "\ufeff" + self.PLAIN.replace("Ada Lovelace", "Grace Hopper")
        self.assertEqual(
            verifier.normalize_skill_author(before),
            verifier.normalize_skill_author(after),
        )

    def test_bom_removal_alone_is_not_a_surface_change(self) -> None:
        self.assertEqual(
            verifier.normalize_skill_author("\ufeff" + self.PLAIN),
            verifier.normalize_skill_author(self.PLAIN),
        )


if __name__ == "__main__":
    unittest.main()
