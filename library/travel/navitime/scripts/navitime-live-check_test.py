"""Consequential harness checks: failure accounting, immutable reuse, retry cap."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("navitime_live_check", Path(__file__).with_name("navitime-live-check.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class RecoveryEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="navitime-evidence-test-")
        self.addCleanup(self.temp.cleanup)
        self.out = Path(self.temp.name)
        self.binary = (self.out / "binary-fixture").resolve()
        self.binary.write_bytes(b"test seam; never executed")
        self.date = "2026-09-29"
        output = self.out / "cached.json"
        output.write_text('{"routes":[{"id":"original-snapshot"}],"meta":{"cache_hit":true}}\n')
        failure = self.out / "refresh.stderr"
        failure.write_text("Error: NAVITIME returned HTTP 202; public data unavailable\n")
        self.good = {"case":"cached", "exit_code":0, "request_count":0, "stdout_file":str(output), "assertions":[{"assertion":"actual cached output", "passed":True}]}
        self.bad = {"case":"refresh", "exit_code":5, "request_count":1, "stderr_file":str(failure), "assertions":[{"assertion":"exit 0", "passed":False}]}
        # The original counter bug underreported the failing request as zero.
        (self.out / "report.json").write_text(json.dumps({"binary":str(self.binary), "japan_date":self.date, "requests":0, "cases":[self.good,self.bad]}))

    def acceptance(self):
        return module.Acceptance(self.binary,self.out,self.date,26,resume=True,recovery_attempts=2,cooldown=0,recovery_delay=0,min_interval=0)

    def test_resume_counts_failure_and_keeps_immutable_cached_evidence(self):
        acceptance = self.acceptance()
        self.assertEqual(acceptance.total_requests,1)
        original = Path(self.good["stdout_file"]).read_bytes()
        with patch.object(acceptance,"_run_once",side_effect=AssertionError("must not execute cached evidence")):
            value,row = acceptance.run("cached",["routes","search"])
        self.assertEqual(value["routes"][0]["id"],"original-snapshot")
        self.assertTrue(row["evidence_reused"])
        self.assertEqual(acceptance.total_requests,1)
        self.assertEqual(Path(self.good["stdout_file"]).read_bytes(),original)
        self.assertTrue(Path(acceptance.previous_report).exists())
        self.assertEqual(acceptance.failed_attempts[0]["request_count"],1)

    def test_challenge_has_at_most_two_recovery_attempts_and_never_becomes_empty_success(self):
        acceptance = self.acceptance()
        calls = []
        def challenged(*args):
            calls.append(args)
            row = dict(self.bad)
            row["assertions"] = [{"assertion":"exit 0","passed":False}]
            acceptance.cases.append(row)
            acceptance.total_requests += 1
            raise AssertionError("HTTP202 is unavailable")
        with patch.object(acceptance,"_run_once",side_effect=challenged), patch.object(module.time,"sleep"):
            with self.assertRaisesRegex(AssertionError,"unavailable"):
                acceptance.run("refresh",["passes","list","--refresh"])
        self.assertEqual(len(calls),2)
        self.assertEqual(acceptance.total_requests,3)
        self.assertEqual(len(acceptance.failed_attempts),2)
        self.assertEqual(acceptance.cases[-1]["exit_code"],5)
        self.assertFalse(acceptance.cases[-1]["assertions"][-1]["passed"])


    def test_recovery_budget_survives_resuming_an_exhausted_case(self):
        report = json.loads((self.out / "report.json").read_text())
        report["failed_attempts"] = [dict(self.bad),dict(self.bad)]
        (self.out / "report.json").write_text(json.dumps(report))
        acceptance = self.acceptance()
        self.assertEqual(acceptance.total_requests,3)
        self.assertEqual(len(acceptance.previous_state["failed_attempts"]),2)
        with patch.object(acceptance,"_run_once",side_effect=AssertionError("must not make another request")) as invoke:
            with self.assertRaisesRegex(AssertionError,"budget exhausted"):
                acceptance.run("refresh",["passes","list","--refresh"])
        invoke.assert_not_called()
        self.assertEqual(acceptance.total_requests,3)


if __name__ == "__main__":
    unittest.main()
