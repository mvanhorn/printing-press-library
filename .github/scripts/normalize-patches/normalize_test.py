#!/usr/bin/env python3
"""Tests for normalize.py. Run from this directory: python3 -m unittest normalize_test"""
from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import normalize


def write(path: Path, obj) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(obj, indent=2) + "\n")


def read(path: Path) -> dict:
    return json.loads(path.read_text())


class NormalizeTest(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)
        self.cli = self.root / "library" / "cat" / "slug"
        self.cli.mkdir(parents=True)
        self.legacy = self.cli / normalize.LEGACY_FILENAME
        self.pdir = self.cli / normalize.PATCHES_DIRNAME

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def run_norm(self, check: bool = False):
        return normalize.normalize_dir(self.cli, check=check)

    # --- core migration ---------------------------------------------------

    def test_explodes_multi_patch_array(self) -> None:
        write(self.legacy, {
            "schema_version": 1,
            "applied_at": "2026-05-21",
            "base_run_id": "RUN-1",
            "base_printing_press_version": "4.9.0",
            "patches": [
                {"id": "alpha", "summary": "A", "reason": "ra"},
                {"id": "beta", "summary": "B", "reason": "rb"},
            ],
        })
        changed, errors = self.run_norm()
        self.assertTrue(changed)
        self.assertEqual(errors, [])
        self.assertFalse(self.legacy.exists(), "legacy file removed")
        self.assertTrue((self.pdir / "alpha.json").exists())
        self.assertTrue((self.pdir / "beta.json").exists())
        self.assertTrue((self.pdir / normalize.GITKEEP_FILENAME).exists())

        alpha = read(self.pdir / "alpha.json")
        self.assertEqual(alpha["schema_version"], 2)
        self.assertEqual(alpha["id"], "alpha")
        # inherited provenance
        self.assertEqual(alpha["applied_at"], "2026-05-21")
        self.assertEqual(alpha["base_run_id"], "RUN-1")
        self.assertEqual(alpha["base_printing_press_version"], "4.9.0")
        self.assertEqual(alpha["reason"], "ra")
        # well-known keys ordered first
        self.assertEqual(
            list(alpha.keys())[:5],
            ["schema_version", "id", "applied_at", "base_run_id", "base_printing_press_version"],
        )

    def test_idempotent(self) -> None:
        write(self.legacy, {
            "schema_version": 1,
            "applied_at": "2026-05-21",
            "base_run_id": "RUN-1",
            "base_printing_press_version": "4.9.0",
            "patches": [{"id": "alpha", "summary": "A"}],
        })
        self.run_norm()
        before = {p.name: p.read_text() for p in self.pdir.iterdir()}
        # Second pass must be a no-op.
        changed, errors = self.run_norm()
        self.assertFalse(changed)
        self.assertEqual(errors, [])
        after = {p.name: p.read_text() for p in self.pdir.iterdir()}
        self.assertEqual(before, after)
        # --check agrees there is nothing to do.
        changed_check, _ = self.run_norm(check=True)
        self.assertFalse(changed_check)

    def test_empty_array_yields_gitkeep_only(self) -> None:
        write(self.legacy, {
            "schema_version": 1,
            "applied_at": "2026-05-21",
            "base_run_id": "RUN-1",
            "base_printing_press_version": "4.9.0",
            "patches": [],
        })
        changed, errors = self.run_norm()
        self.assertTrue(changed)
        self.assertEqual(errors, [])
        self.assertFalse(self.legacy.exists())
        entries = sorted(p.name for p in self.pdir.iterdir())
        self.assertEqual(entries, [normalize.GITKEEP_FILENAME])

    def test_patch_inherits_only_missing_provenance(self) -> None:
        write(self.legacy, {
            "schema_version": 1,
            "applied_at": "2026-05-21",
            "base_run_id": "RUN-1",
            "base_printing_press_version": "4.9.0",
            "patches": [{"id": "alpha", "applied_at": "2026-06-01"}],
        })
        self.run_norm()
        alpha = read(self.pdir / "alpha.json")
        # patch's own applied_at wins over the top-level one
        self.assertEqual(alpha["applied_at"], "2026-06-01")

    # --- edge cases the real data showed ---------------------------------

    def test_id_less_patch_gets_synthesized_filename(self) -> None:
        write(self.legacy, {
            "schema_version": 1,
            "patches": [
                {"summary": "Fix the products parser for wall connectors"},
                {"summary": "Fix the products parser for wall connectors"},  # collision
            ],
        })
        changed, errors = self.run_norm()
        self.assertTrue(changed)
        self.assertEqual(errors, [])
        names = sorted(p.name for p in self.pdir.glob("*.json"))
        self.assertEqual(
            names,
            ["fix-the-products-parser-for-wall-connectors-2.json",
             "fix-the-products-parser-for-wall-connectors.json"],
        )

    def test_global_lists_go_to_meta(self) -> None:
        write(self.legacy, {
            "schema_version": 1,
            "applied_at": "2026-05-21",
            "base_run_id": "RUN-1",
            "base_printing_press_version": "4.9.0",
            "patches": [{"id": "alpha"}],
            "deferred_to_upstream": [{"item": "x"}],
            "upstream_tracking": ["https://example/issue/1"],
        })
        changed, errors = self.run_norm()
        self.assertTrue(changed)
        self.assertEqual(errors, [])
        meta = read(self.pdir / normalize.META_FILENAME)
        self.assertEqual(meta["schema_version"], 2)
        self.assertEqual(meta["deferred_to_upstream"], [{"item": "x"}])
        self.assertEqual(meta["upstream_tracking"], ["https://example/issue/1"])
        # provenance does NOT leak into meta (it is per-patch)
        self.assertNotIn("base_run_id", meta)

    # --- inflow / merge case ---------------------------------------------

    def test_merges_incoming_legacy_into_existing_dir(self) -> None:
        # Dir already has alpha (converted earlier); an old-Press branch lands a
        # legacy file re-adding alpha plus a new gamma.
        self.pdir.mkdir()
        write(self.pdir / "alpha.json", {"schema_version": 2, "id": "alpha", "summary": "A"})
        (self.pdir / normalize.GITKEEP_FILENAME).write_text("")
        write(self.legacy, {
            "schema_version": 1,
            "patches": [
                {"id": "alpha", "summary": "A-dup"},  # dup id -> skipped
                {"id": "gamma", "summary": "G"},       # new -> added
            ],
        })
        changed, errors = self.run_norm()
        self.assertTrue(changed)
        self.assertEqual(errors, [])
        self.assertFalse(self.legacy.exists())
        # alpha preserved from the dir (not overwritten by the dup)
        self.assertEqual(read(self.pdir / "alpha.json")["summary"], "A")
        self.assertEqual(read(self.pdir / "gamma.json")["summary"], "G")

    # --- robustness -------------------------------------------------------

    def test_unparseable_legacy_is_reported_not_destroyed(self) -> None:
        self.legacy.write_text("{ not json")
        changed, errors = self.run_norm()
        self.assertFalse(changed)
        self.assertEqual(len(errors), 1)
        self.assertTrue(self.legacy.exists(), "bad file left intact for a human")
        self.assertFalse(self.pdir.exists())

    def test_no_patches_artifacts_is_noop(self) -> None:
        changed, errors = self.run_norm()
        self.assertFalse(changed)
        self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
