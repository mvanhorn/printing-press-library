#!/usr/bin/env python3
"""Offline regression for exact cutoff preservation and independent subsecond drift."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("planning_live_common", Path(__file__).with_name("planning-live-common.py"))
common = importlib.util.module_from_spec(spec)
spec.loader.exec_module(common)


class CutoffEvidenceTests(unittest.TestCase):
    def test_observed_subsecond_drift_retains_both_exact_strings(self):
        cli = "2026-09-29T12:30:00.380+09:00"
        independent = "2026-09-29T12:30:00.039+09:00"
        proof = common.cutoff_comparison(cli, independent)
        self.assertTrue(proof["within_one_second"])
        self.assertTrue(proof["dynamic_subsecond_difference_observed"])
        self.assertAlmostEqual(proof["delta_seconds"], .341)
        self.assertEqual(proof["cli_exact"], cli)
        self.assertEqual(proof["independent_exact"], independent)

    def test_tolerance_rejects_material_change_and_naive_time(self):
        self.assertFalse(common.cutoff_comparison("2026-09-29T12:30:00.380+09:00", "2026-09-29T12:30:01.381+09:00")["within_one_second"])
        self.assertTrue(common.cutoff_comparison("2026-09-29T12:30:00+09:00", "2026-09-29T03:30:01Z")["within_one_second"])
        self.assertFalse(common.cutoff_comparison(None, "2026-09-29T12:30:00+09:00")["within_one_second"])
        self.assertTrue(common.cutoff_comparison(None, None)["within_one_second"])
        with self.assertRaises(ValueError):
            common.cutoff_comparison("2026-09-29T12:30:00", "2026-09-29T12:30:00+09:00")

    def test_exact_request_fingerprint_reads_raw_field_without_normalizing(self):
        body = {"shop_id": common.VENUE_ID, "locale": "en", "date_min": "2026-09-28T15:00:00Z",
                "date_max": "2026-09-30T15:00:00Z", "has_price": True}
        # The producer hashes method, exact URL and sorted compact JSON body.
        wire = (b'POST\nhttps://production.tablecheck.com/v2/hub/menu_items\n'
                b'{"date_max":"2026-09-30T15:00:00Z","date_min":"2026-09-28T15:00:00Z","has_price":true,"locale":"en","shop_id":"67e657634474874e35785280"}')
        fingerprint = hashlib.sha256(wire).hexdigest()
        cutoff = "2026-09-29T12:30:00.380+09:00"
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / (fingerprint + ".json")
            path.write_text(json.dumps({"version": 1, "fetched_at": "2026-09-27T15:00:00Z", "body": {
                "menu_items": [{"id": common.KNOWN_COURSE_ID, "min_time_cutoff_at": cutoff}]}}), encoding="utf-8")
            item, proof = common.cached_menu_item(directory, body, common.KNOWN_COURSE_ID)
            self.assertEqual(item["min_time_cutoff_at"], cutoff)
            self.assertEqual(proof["request_fingerprint"], fingerprint)
            self.assertEqual(proof["cache_file"], str(path))
            with self.assertRaises(ValueError):
                common.cached_menu_item(directory, body, "not-the-source-course")


if __name__ == "__main__":
    unittest.main()
