"""Focused checks for the independent Jalan source witness."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name("live-e2e.py")
spec = importlib.util.spec_from_file_location("jalan_live_e2e", SCRIPT)
live = importlib.util.module_from_spec(spec)
spec.loader.exec_module(live)
SEARCH_URL = "https://www.jalan.net/uw/uwp1400/uww1400.do?lrgCd=141600"


class WitnessTests(unittest.TestCase):
    def setUp(self):
        self.runner = live.Runner(SimpleNamespace(), "/unused")
        self.runner.current = {}
        self.item = {
            "id": "385995", "name_ja": "目標の宿",
            "evidence": [{"field": "name_ja", "text": "目標の宿"}],
            "price": {"amount": 50600, "basis": "whole_stay",
                      "evidence": [{"field": "price.amount", "text": "50,600円"}]},
        }

    def witness(self, markup):
        parser = live.SourceText()
        parser.feed(markup)
        self.runner.sources[SEARCH_URL] = {
            "text": live.compact_text(" ".join(parser.parts)),
            "cards": parser.cards,
            "charge_quotes": parser.charge_quotes,
        }
        return parser

    def test_search_price_must_be_on_matching_native_card(self):
        parser = self.witness('''
            <div class="p-yadoCassette" id="yadNo385995">目標の宿 40,000円</div>
            <div class="p-yadoCassette" id="yadNo371898">別の宿 50,600円</div>
            <section id="faq">目標の宿 50,600円</section>
        ''')
        self.assertIn("385995", parser.cards)
        with self.assertRaisesRegex(AssertionError, "base amount not present"):
            self.runner.evidence(self.item, SEARCH_URL, ("name_ja",))

    def test_search_price_on_matching_card_is_accepted(self):
        parser = self.witness('''
            <div class="p-yadoCassette" id="yadNo385995">目標の宿 50,600円</div>
            <div class="p-yadoCassette" id="yadNo371898">別の宿 80,000円</div>
            <section id="faq">目標の宿 1円</section>
        ''')
        witness = self.runner.evidence(self.item, SEARCH_URL, ("name_ja",))
        self.assertEqual(witness, parser.cards["385995"][0])
        self.assertIn("price.amount", self.runner.current["asserted_fields"])

    def test_sponsored_cards_and_faq_do_not_corroborate_missing_result(self):
        parser = self.witness('''
            <div class="p-yadoCassette p-yadoCassette--pr" id="yadNo385995">目標の宿 50,600円</div>
            <div class="p-yadoCassette" id="sa_yadNo385995">目標の宿 50,600円</div>
            <section id="faq">目標の宿 50,600円</section>
        ''')
        self.assertNotIn("385995", parser.cards)
        with self.assertRaisesRegex(AssertionError, "card.*absent"):
            self.runner.evidence(self.item, SEARCH_URL, ("name_ja",))

    def test_search_endpoint_requires_card_even_without_any_cards(self):
        self.witness('<section id="faq">目標の宿 50,600円</section>')
        with self.assertRaisesRegex(AssertionError, "card.*absent"):
            self.runner.evidence(self.item, SEARCH_URL, ("name_ja",))

    def test_witness_uses_cli_anonymous_request_headers(self):
        url = SEARCH_URL
        raw = '<div class="p-yadoCassette" id="yadNo385995">目標の宿 50,600円</div>'.encode("cp932")

        class FakeResponse:
            def __enter__(self):
                return self
            def __exit__(self, *_):
                return False
            def geturl(self):
                return url
            def read(self, limit):
                return raw[:limit]
            headers = SimpleNamespace(get_content_charset=lambda: "shift_jis")

        def serve(request, timeout):
            self.assertEqual(timeout, 20)
            self.assertEqual(request.get_header("User-agent"),
                             "jalan-pp-cli/1.0 (anonymous read-only accommodation research)")
            self.assertEqual(request.get_header("Accept"), "text/html")
            self.assertEqual(request.get_header("Accept-language"), "ja")
            return FakeResponse()

        with mock.patch.object(live.urllib.request, "urlopen", side_effect=serve):
            witness = self.runner.source(url)
        self.assertIn("385995", witness["cards"])
        self.assertEqual(self.runner.witness_requests, 1)
        self.assertEqual(len(self.runner.current["independent_sources"]), 1)

    def cached_snapshot(self, body, observed_at="2026-09-27T15:00:00Z"):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.runner.home = temporary.name
        digest = hashlib.sha256(SEARCH_URL.encode("utf-8")).hexdigest()
        path = Path(temporary.name) / "cache" / "observations" / f"v1-{digest}.json"
        path.parent.mkdir(parents=True)
        entry = {"version": 1, "url": SEARCH_URL, "observed_at": observed_at, "body": body}
        path.write_text(json.dumps(entry), encoding="utf-8")
        response = {"meta": {"cache_status": "live", "upstream_requests": 1,
                             "source_url": SEARCH_URL, "source_urls": [SEARCH_URL],
                             "observed_at": observed_at}}
        return path, entry, response

    def test_snapshot_witness_matches_cold_observation_and_remains_separate(self):
        body = '<div class="p-yadoCassette" id="yadNo385995">目標の宿 50,600円</div>'
        _, _, response = self.cached_snapshot(body)
        witness = self.runner.snapshot_evidence(self.item, response)
        self.assertIn("50,600円", witness)
        provenance = self.runner.current["source_snapshot_witness"]
        self.assertEqual(provenance["url"], SEARCH_URL)
        self.assertEqual(provenance["observed_at"], response["meta"]["observed_at"])
        self.assertEqual(provenance["sha256"], hashlib.sha256(body.encode("utf-8")).hexdigest())
        self.assertEqual(provenance["bytes"], len(body.encode("utf-8")))
        self.assertEqual(provenance["kind"], "fresh_cli_http_observation_independent_parser")
        self.assertEqual(self.runner.witness_requests, 0)
        self.assertEqual(self.runner.sources, {})
        self.assertNotIn("independent_sources", self.runner.current)

    def test_snapshot_rejects_mismatched_url_timestamp_and_stale_meta(self):
        body = '<div class="p-yadoCassette" id="yadNo385995">目標の宿 50,600円</div>'
        path, entry, response = self.cached_snapshot(body)
        cases = [
            ("wrong URL", {**entry, "url": SEARCH_URL + "&idx=30"}, response),
            ("wrong timestamp", {**entry, "observed_at": "2026-09-27T15:01:00Z"}, response),
            ("cache hit", entry, {"meta": {**response["meta"], "cache_status": "hit"}}),
            ("zero requests", entry, {"meta": {**response["meta"], "upstream_requests": 0}}),
        ]
        for label, cached, cold in cases:
            with self.subTest(label=label):
                path.write_text(json.dumps(cached), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    self.runner.snapshot_evidence(self.item, cold)

    def test_snapshot_still_requires_matching_card_price_and_bounded_body(self):
        body = ('''<div class="p-yadoCassette" id="yadNo385995">目標の宿 40,000円</div>
                  <div class="p-yadoCassette" id="yadNo371898">別の宿 50,600円</div>
                  <section id="faq">目標の宿 50,600円</section>''')
        path, _, response = self.cached_snapshot(body)
        with self.assertRaisesRegex(AssertionError, "base amount not present"):
            self.runner.snapshot_evidence(self.item, response)
        with path.open("wb") as file:
            file.truncate(16 * 1024 * 1024 + 1025)
        with self.assertRaisesRegex(AssertionError, "cache file exceeds"):
            self.runner.snapshot_evidence(self.item, response)

    def test_plan_charge_capture_remains_independent(self):
        parser = live.SourceText()
        parser.feed('<div class="p-planOverview__charge">50,600円</div>')
        self.assertEqual(parser.charge_quotes, ["50,600円"])


if __name__ == "__main__":
    unittest.main()
