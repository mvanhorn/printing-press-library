#!/usr/bin/env python3
"""Run bounded, read-only live semantic checks against the built Travel CLI."""
import argparse
import datetime as dt
import json
from pathlib import Path
import subprocess
import tempfile
import time
from urllib.parse import urlparse
from zoneinfo import ZoneInfo


def default_checkin():
    day = dt.datetime.now(ZoneInfo("Asia/Tokyo")).date() + dt.timedelta(days=42)
    return day + dt.timedelta(days=(6 - day.weekday()) % 7)


class Verification:
    def __init__(self, binary, evidence, home):
        self.binary, self.evidence, self.home = binary, evidence, home
        self.runs, self.assertions = [], []
        self.last_start = 0.0

    def run(self, name, args):
        # Separate invocations also respect the public request floor.
        remaining = 1.0 - (time.monotonic() - self.last_start)
        if remaining > 0:
            time.sleep(remaining)
        self.last_start = time.monotonic()
        command = [str(self.binary), *args, "--home", self.home, "--timeout", "60s", "--json"]
        started = time.monotonic()
        try:
            process = subprocess.run(command, capture_output=True, timeout=85, check=False)
            code, stdout, stderr = process.returncode, process.stdout, process.stderr
        except subprocess.TimeoutExpired as exc:
            code, stdout, stderr = 124, exc.stdout or b"", (exc.stderr or b"") + b"\nharness timeout"
        elapsed = time.monotonic() - started
        stem = f"{len(self.runs) + 1:02d}_{name}"
        (self.evidence / (stem + ".stdout.json")).write_bytes(stdout)
        (self.evidence / (stem + ".stderr.txt")).write_bytes(stderr)
        try:
            payload = json.loads(stdout)
        except (ValueError, UnicodeDecodeError):
            payload = None
        record = {"name": name, "arguments": args, "exit_code": code,
                  "stdout_bytes": len(stdout), "wall_seconds": round(elapsed, 6),
                  "stdout_artifact": stem + ".stdout.json", "stderr_artifact": stem + ".stderr.txt"}
        self.runs.append(record)
        return {"record": record, "payload": payload, "stderr": stderr.decode("utf-8", "replace")}

    def check(self, name, ok, detail=""):
        self.assertions.append({"name": name, "passed": bool(ok), "detail": detail})

    def envelope(self, name, run, expect_list):
        payload = run["payload"]
        valid = (run["record"]["exit_code"] == 0 and isinstance(payload, dict)
                 and isinstance(payload.get("meta"), dict)
                 and isinstance(payload.get("results"), list if expect_list else dict))
        self.check(name + "_json_envelope", valid, f"exit={run['record']['exit_code']}")
        return payload if valid else {"meta": {}, "results": [] if expect_list else {}}

    def source(self, name, meta):
        source = meta.get("source_info", {})
        self.check(name + "_source_observation", isinstance(source, dict)
                   and source.get("name") == "Rakuten Travel"
                   and bool(source.get("fetched_at")) and bool(source.get("observed_at"))
                   and source.get("cache_state") in ("disabled", "miss", "refresh", "hit", "invocation_snapshot")
                   and urlparse(source.get("url", "")).scheme == "https",
                   str(source))

    def offer(self, name, row, query):
        if not isinstance(row, dict):
            self.check(name + "_actual_offer", False, "No actual room offer was available")
            return
        echoed = row.get("query", {})
        expected = {"hotel_id": query["hotel"], "checkin": query["checkin"],
                    "checkout": query["checkout"], "rooms": query["rooms"],
                    "adults_per_room": 2}
        self.check(name + "_identity_and_query", all(isinstance(row.get(key), str) and row[key]
                   for key in ("hotel_id", "plan_id", "room_id"))
                   and all(echoed.get(key) == value for key, value in expected.items())
                   and echoed.get("children_per_room", {}).get("infant_none") == query.get("infant_none", 0),
                   str(echoed))
        price = row.get("price", {})
        self.check(name + "_whole_stay_per_room_price", price.get("currency") == "JPY"
                   and isinstance(price.get("per_room_whole_stay_jpy"), int)
                   and price["per_room_whole_stay_jpy"] > 0
                   and price.get("consumption_tax") in ("included", "unknown")
                   and price.get("accommodation_tax") in ("unknown", "excluded", "included")
                   and bool(price.get("source_label"))
                   and "per_night_jpy" not in price and "all_rooms_total_jpy" not in price,
                   str(price))
        handoff = urlparse(row.get("booking_url", ""))
        self.check(name + "_dated_room_handoff", handoff.scheme == "https"
                   and handoff.hostname == "hotel.travel.rakuten.co.jp"
                   and handoff.fragment == row.get("room_anchor") and bool(handoff.fragment)
                   and "f_nen1=" in handoff.query and "f_heya_su=" in handoff.query,
                   row.get("booking_url", ""))


def offer_args(query, leaf="search", limit=5):
    args = ["offers", leaf, "--hotel", query["hotel"], "--checkin", query["checkin"],
            "--checkout", query["checkout"], "--rooms", str(query["rooms"]), "--adults-per-room", "2"]
    if query.get("infant_none"):
        args += ["--infant-none", str(query["infant_none"])]
    if leaf == "search":
        args += ["--limit", str(limit)]
    return args


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("evidence_dir", type=Path)
    parser.add_argument("--checkin", type=dt.date.fromisoformat, default=None,
                        help="Reproducible override; default next Sunday at least 42 days ahead in JST")
    args = parser.parse_args()
    binary = args.binary.expanduser().resolve(strict=True)
    evidence = args.evidence_dir.expanduser().resolve()
    evidence.mkdir(parents=True, exist_ok=True)
    checkin = args.checkin or default_checkin()
    environment = {"mode": "live_public_network", "started_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
                   "binary": str(binary), "checkin": checkin.isoformat(), "nights": 2}
    with tempfile.TemporaryDirectory(prefix="rakuten-travel-live-") as home:
        verify = Verification(binary, evidence, home)
        areas = verify.envelope("areas", verify.run("areas", ["areas", "list", "--parent", "tokyo"]), True)
        verify.source("areas", areas["meta"])
        verify.check("area_path_ids", bool(areas["results"])
                     and all(isinstance(row.get("id"), str) and row["id"].startswith("tokyo/")
                             for row in areas["results"]))

        for name, keyword in (("keyword_ja", "品川"), ("keyword_en", "Shinagawa")):
            payload = verify.envelope(name, verify.run(name, ["hotels", "search", "--query", keyword]), True)
            verify.source(name, payload["meta"])
            verify.check(name + "_literal_query", payload["meta"].get("query", {}).get("query") == keyword,
                         "Japanese and English subsets need not contain identical hotels")
            verify.check(name + "_actual_candidates", bool(payload["results"]) and all(
                isinstance(row.get("hotel_id"), str) and row["hotel_id"].isdigit()
                for row in payload["results"]))

        details = verify.envelope("hotel_details", verify.run("hotel_details", ["hotels", "show", "--hotel", "51870"]), False)
        verify.source("hotel_details", details["meta"])
        hotel = details["results"]
        verify.check("hotel_details_content", hotel.get("hotel_id") == "51870"
                     and all(isinstance(hotel.get(key), list) for key in
                             ("access", "parking", "hotel_amenities", "room_amenities", "notes", "property_cancellation_policy"))
                     and bool(hotel.get("access")) and bool(hotel.get("hotel_amenities"))
                     and hotel.get("coordinates") is None and bool(hotel.get("coordinates_reason")))

        query = {"hotel": "51870", "checkin": checkin.isoformat(),
                 "checkout": (checkin + dt.timedelta(days=2)).isoformat(), "rooms": 1}
        base_run = verify.run("offers_base", offer_args(query))
        base = verify.envelope("offers_base", base_run, True)
        verify.source("offers_base", base["meta"])
        if not base["results"]:
            # One bounded alternative date; no hidden property/date expansion.
            alternate = checkin + dt.timedelta(days=7)
            query = dict(query, checkin=alternate.isoformat(), checkout=(alternate + dt.timedelta(days=2)).isoformat())
            base_run = verify.run("offers_bounded_date_fallback", offer_args(query))
            base = verify.envelope("offers_fallback", base_run, True)
            verify.source("offers_fallback", base["meta"])
        verify.check("actual_dated_inventory", bool(base["results"]), "Empty inventory cannot satisfy the actual-offer assertion")
        selected = base["results"][0] if base["results"] else None
        verify.offer("base", selected, query)
        if selected:
            show_args = offer_args(query, "show") + ["--plan", selected["plan_id"], "--room", selected["room_id"]]
            shown = verify.envelope("offers_show", verify.run("offers_show", show_args), False)
            verify.offer("show", shown["results"].get("offer"), query)
            inspected = shown["results"].get("offer", {})
            verify.check("show_exact_tuple", all(inspected.get(key) == selected.get(key)
                         for key in ("hotel_id", "plan_id", "room_id")))
            verify.check("show_property_policy_scope", isinstance(shown["results"].get("property_fee_notes"), list)
                         and shown["results"].get("property", {}).get("hotel_id") == query["hotel"]
                         and shown["results"].get("property_cancellation_policy", {}).get("level") == "property"
                         and (inspected.get("plan_cancellation_policy") is None
                              or inspected["plan_cancellation_policy"].get("level") == "plan"))
        else:
            verify.check("show_exact_tuple", False, "No real source tuple available for inspection")

        for name, modified in (("offers_child", dict(query, infant_none=1)),
                               ("offers_two_rooms", dict(query, rooms=2))):
            payload = verify.envelope(name, verify.run(name, offer_args(modified)), True)
            verify.source(name, payload["meta"])
            verify.offer(name, payload["results"][0] if payload["results"] else None, modified)

        first = verify.envelope("paging_first", verify.run("paging_first", offer_args(query, limit=1)), True)
        page = first["meta"].get("page", {})
        rows_seen = page.get("rows_seen", 0)
        source_page = page.get("source_page", 1)
        within = page.get("has_more") and page.get("next_page") == source_page and isinstance(page.get("next_offset"), int)
        verify.check("within_page_continuation_available", within, str(page))
        if within:
            continuation = offer_args(query, limit=1) + ["--page", str(source_page), "--offset", str(page["next_offset"])]
            second = verify.envelope("paging_offset", verify.run("paging_offset", continuation), True)
            verify.check("within_page_followed", bool(second["results"])
                         and second["meta"].get("page", {}).get("offset") == page["next_offset"]
                         and second["results"][0].get("query", {}).get("checkin") == query["checkin"]
                         and (second["results"][0].get("plan_id"), second["results"][0].get("room_id"))
                         != (first["results"][0].get("plan_id"), first["results"][0].get("room_id")))
        if isinstance(rows_seen, int) and rows_seen > 0:
            tail = verify.envelope("paging_source_tail", verify.run(
                "paging_source_tail", offer_args(query, limit=1) + ["--offset", str(rows_seen - 1)]), True)
            tail_page = tail["meta"].get("page", {})
        else:
            tail_page = {}
        next_source = tail_page.get("has_more") and isinstance(tail_page.get("next_page"), int) and tail_page["next_page"] > source_page
        verify.check("source_page_continuation_available", next_source, str(tail_page))
        if next_source:
            following = verify.envelope("paging_source_next", verify.run(
                "paging_source_next", offer_args(query, limit=1) + ["--page", str(tail_page["next_page"]), "--offset", "0"]), True)
            verify.check("source_page_followed", bool(following["results"])
                         and following["meta"].get("page", {}).get("source_page") == tail_page["next_page"]
                         and following["meta"].get("page", {}).get("source_unit") == "plans")

        in_date = dt.date.fromisoformat(query["checkin"])
        compare_dates = [in_date.isoformat(), (in_date + dt.timedelta(days=1)).isoformat()]
        comparison = verify.envelope("compare", verify.run("compare", ["compare", "--hotels", "51870,72056",
            "--checkins", ",".join(compare_dates), "--nights", "2", "--rooms", "1", "--adults-per-room", "2"]), False)
        cells = comparison["results"].get("cells", [])
        failures = comparison["results"].get("fetch_failures", [])
        expected_cells = {(hotel_id, day) for hotel_id in ("51870", "72056") for day in compare_dates}
        actual_cells = {(cell.get("query", {}).get("hotel_id"), cell.get("query", {}).get("checkin")) for cell in cells}
        verify.check("compare_complete_bounded_matrix", actual_cells == expected_cells and len(cells) == 4
                     and comparison["meta"].get("source") == "computed"
                     and comparison["meta"].get("price_comparison_scope") == "bounded_source_page_only")
        verify.check("compare_failures_preserved", isinstance(failures, list)
                     and len(failures) == sum(cell.get("status") == "error" for cell in cells))
        verify.check("compare_actual_available_cell", any(cell.get("status") == "ok" for cell in cells))
        for index, cell in enumerate(cells):
            cell_query = cell.get("query", {})
            valid = cell.get("status") in ("ok", "no_availability", "error") and cell_query.get("rooms") == 1 and cell_query.get("adults_per_room") == 2
            if cell.get("status") == "ok":
                minimum = cell.get("bounded_source_page_minimum") or {}
                valid = valid and minimum.get("price", {}).get("per_room_whole_stay_jpy", 0) > 0 and minimum.get("price", {}).get("currency") == "JPY"
            else:
                valid = valid and cell.get("bounded_source_page_minimum") is None
            verify.check(f"compare_cell_{index}_status_price", valid, str(cell.get("status")))

        no_match = verify.envelope("zero_keyword", verify.run("zero_keyword", ["hotels", "search",
            "--query", "CodexNoSuchRakutenProperty" + dt.datetime.now().strftime("%Y%m%d%H%M")]), True)
        verify.check("zero_results_are_successful_empty_array", no_match["results"] == [] and no_match["meta"].get("status") == "no_matches")
        invalid = verify.run("invalid_date_control", offer_args(dict(query, checkout=query["checkin"])))
        verify.check("invalid_date_usage_error", invalid["record"]["exit_code"] == 2
                     and invalid["payload"] is None and bool(invalid["stderr"]))
        budget = verify.run("request_budget_control", ["hotels", "show", "--hotel", "51870", "--max-requests", "1", "--no-cache"])
        verify.check("request_failure_is_not_empty_inventory", budget["record"]["exit_code"] == 5
                     and "request_budget" in budget["stderr"])

        report = {"environment": environment, "runs": verify.runs, "assertions": verify.assertions,
                  "passed": all(check["passed"] for check in verify.assertions)}
        (evidence / "live-verification.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        failed = [check for check in verify.assertions if not check["passed"]]
        lines = ["# Live public-source verification", "", f"Check-in: {query['checkin']}; two nights; anonymous read-only backend.",
                 f"Result: {'PASS' if report['passed'] else 'UNSATISFIED'}; {len(verify.assertions) - len(failed)}/{len(verify.assertions)} assertions passed.", "",
                 "Each command's stdout and stderr are stored beside the machine report. Empty availability is recorded explicitly and cannot satisfy actual-offer checks.", ""]
        lines += [f"- {check['name']}: {check['detail'] or 'assertion was not satisfied'}" for check in failed]
        (evidence / "live-verification.md").write_text("\n".join(lines) + "\n")
        print(json.dumps({"passed": report["passed"], "assertions": len(verify.assertions), "unsatisfied": [check["name"] for check in failed],
                          "report": str(evidence / "live-verification.json")}, ensure_ascii=False))
        return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
