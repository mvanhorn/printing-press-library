#!/usr/bin/env python3
"""Opt-in, finite TableCheck planning E2E checks. No bookings or holds are made."""
import argparse
import datetime as dt
from decimal import Decimal
import importlib.util
import json
from pathlib import Path
import re
import sys
import tempfile
from urllib.parse import parse_qs, urlparse

spec = importlib.util.spec_from_file_location("planning_live_common", Path(__file__).with_name("planning-live-common.py"))
common = importlib.util.module_from_spec(spec)
spec.loader.exec_module(common)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-live", action="store_true", help="Explicitly authorize the fixed anonymous HTTP reads")
    parser.add_argument("--binary", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--date", help="Tokyo YYYY-MM-DD; default is Tokyo today plus two days")
    args = parser.parse_args()
    if not args.run_live:
        print(json.dumps({"passed": False, "errors": ["--run-live is required; no network requests made"]}))
        return 2
    cases, errors = [], []
    budget = common.Budget(40)
    runner = None
    current = {}
    started = dt.datetime.now(dt.timezone.utc).isoformat()
    try:
        day, tomorrow = common.date_range(args.date)
        runner = common.Runner(args.binary, args.output_dir, budget)
        cache_parent = runner.output / "cache"
        cache_parent.mkdir(exist_ok=True)
        cache = Path(tempfile.mkdtemp(prefix="live-", dir=cache_parent))

        def run_case(name, callback):
            case = common.Assertions(name)
            try:
                callback(case)
            except Exception as error:
                case.errors.append("%s: %s" % (type(error).__name__, error))
            cases.append(case.result())
            if runner.halted:
                raise RuntimeError("stopped suite after unknown request count; reserved ceiling retained")

        def cuisines(case):
            data, record = runner.cli("cuisines", ["cuisines", "list", "--query", "sushi", "--limit", "5"], cache, 2)
            case.records.append(record)
            items = (data or {}).get("items", [])
            case.check("stable_sushi_key", any(item.get("key") == "sushi" for item in items), items)
            case.check("bounded_rows", len(items) <= 5, len(items), "<=5")
            case.check("Japanese_label", any(item.get("key") == "sushi" and item.get("name_ja") for item in items))
        run_case("cuisine_lookup", cuisines)

        search_args = ["venues", "search", "--lat", "35.681236", "--lon", "139.767125", "--radius", "3000",
                       "--cuisine", "sushi", "--budget-max", "20000", "--date", day, "--time", "18:00", "--party", "2", "--limit", "2"]

        def search(case):
            data, record = runner.cli("search-first", search_args, cache, 2)
            case.records.append(record)
            current["search"] = data or {}
            items = current["search"].get("items", [])
            query = current["search"].get("query", {})
            case.check("nonempty_bounded_discovery", 0 < len(items) <= 2, len(items), "1..2")
            case.check("explicit_geography", query.get("latitude") == 35.681236 and query.get("longitude") == 139.767125 and query.get("radius_m") == 3000, query)
            case.check("exact_preferences", query.get("date") == day and query.get("time") == "18:00" and query.get("party") == 2 and query.get("cuisine") == "sushi", query)
            case.check("venue_average_budget_label", query.get("budget_basis") == "venue_dinner_average", query.get("budget_basis"))
            case.check("stable_IDs_and_cuisine", all(item.get("id") and item.get("slug") and "sushi" in (item.get("cuisines") or []) for item in items))
            budgets = [item.get("dinner_average_budget") for item in items]
            case.check("decimal_budget_text", all(value is None or isinstance(value, str) and re.fullmatch(r"\d+(\.\d+)?", value) for value in budgets), budgets)
            known_budgets = [value for value in budgets if isinstance(value, str)]
            case.check("source_budgets_meet_ceiling", all(Decimal(value) <= Decimal("20000") for value in known_budgets), known_budgets, "<=20000 venue dinner average")
            distances = [item.get("distance_m") for item in items]
            case.check("source_distances_inside_explicit_radius", all(isinstance(value, (int, float)) and 0 <= value <= 3000 for value in distances), distances, "0..3000 metres")
            dates = []
            for item in items:
                for stamp in item.get("discovery_availability") or []:
                    if not isinstance(stamp, str):
                        raise ValueError("unexpected discovery availability schema")
                    parsed = dt.datetime.fromisoformat(stamp.replace("Z", "+00:00"))
                    if parsed.tzinfo is None:
                        raise ValueError("discovery timestamp lacks UTC offset")
                    dates.append(parsed.astimezone(common.TOKYO).date().isoformat())
            case.check("returned_discovery_dates_match_preference", bool(dates) and all(value == day for value in dates), sorted(set(dates)), day)
            case.check("discovery_scope", all(item.get("availability_scope") == "discovery_summary" for item in items))
        run_case("geographic_cuisine_budget_date_party_search", search)

        def cursor(case):
            first = current.get("search", {})
            token = first.get("pagination", {}).get("next_cursor")
            case.check("source_cursor_available", isinstance(token, str) and bool(token), token)
            if not token:
                return
            data, record = runner.cli("search-next", search_args + ["--cursor", token], cache, 2)
            case.records.append(record)
            first_ids = {item.get("id") for item in first.get("items", [])}
            next_items = (data or {}).get("items", [])
            next_ids = {item.get("id") for item in next_items}
            case.check("distinct_next_page_IDs", bool(next_ids) and not first_ids.intersection(next_ids), sorted(next_ids))
            case.check("bounded_next_page", len(next_items) <= 2, len(next_items))
            distances = [item.get("distance_m") for item in first.get("items", []) + next_items]
            known = [value for value in distances if isinstance(value, (int, float))]
            case.check("source_distance_order", known == sorted(known), known)
        run_case("opaque_cursor_page", cursor)

        def venue(case):
            data, record = runner.cli("venue", ["venues", "get", common.VENUE], cache, 2)
            case.records.append(record)
            value = (data or {}).get("venue", {})
            current["venue"] = value
            case.check("known_stable_identity", value.get("id") == common.VENUE_ID and value.get("slug") == common.VENUE, {"id": value.get("id"), "slug": value.get("slug")})
            normalized = re.sub(r"[^A-Z0-9]", "", str(value.get("name") or "").upper())
            case.check("public_venue_name", "SUSHITOKYO81" in normalized, value.get("name"))
            case.check("venue_timezone_currency", value.get("time_zone") == "Asia/Tokyo" and value.get("currency") == "JPY", {"time_zone": value.get("time_zone"), "currency": value.get("currency")})
            latitude, longitude = value.get("latitude"), value.get("longitude")
            case.check("venue_geocode_not_caller_location", isinstance(latitude, (int, float)) and isinstance(longitude, (int, float)) and 35 <= latitude <= 36 and 139 <= longitude <= 140, [latitude, longitude])
            case.check("canonical_public_venue_URL", value.get("venue_url") == "https://www.tablecheck.com/en/" + common.VENUE, value.get("venue_url"))
        run_case("venue_identity_and_geocode", venue)

        def courses(case):
            data, record = runner.cli("courses-list", ["courses", "list", common.VENUE, "--from", day, "--to", tomorrow, "--limit", "10"], cache, 4)
            case.records.append(record)
            items = (data or {}).get("items", [])
            case.check("nonempty_bounded_courses", 0 < len(items) <= 10, len(items))
            case.check("stable_IDs_and_decimal_strings", all(item.get("id") and (item.get("price") is None or isinstance(item.get("price"), str)) for item in items))
            chosen = next((item for item in items if item.get("id") == common.KNOWN_COURSE_ID), items[0] if items else None)
            current["course_id"] = chosen.get("id") if chosen else None
            current["course_summary"] = chosen
            current["historical_course_present"] = any(item.get("id") == common.KNOWN_COURSE_ID for item in items)
            case.check("course_eligibility_scope", all(item.get("availability_scope") == "course_listing_eligibility" for item in items))
        run_case("course_summaries", courses)

        def course_detail(case):
            course_id = current.get("course_id")
            if not course_id:
                raise ValueError("no source-listed course available for detail comparison")
            data, record = runner.cli("course-detail", ["courses", "get", common.VENUE, course_id, "--from", day, "--to", tomorrow], cache, 4)
            case.records.append(record)
            detail = (data or {}).get("course", {})
            start = dt.datetime.combine(dt.date.fromisoformat(day), dt.time(), common.TOKYO)
            end = dt.datetime.combine(dt.date.fromisoformat(tomorrow) + dt.timedelta(days=1), dt.time(), common.TOKYO)
            body = {"shop_id": common.VENUE_ID, "locale": "en", "date_min": start.astimezone(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
                    "date_max": end.astimezone(dt.timezone.utc).isoformat().replace("+00:00", "Z"), "has_price": True}
            source, direct_record = runner.direct("source-menus", "/v2/hub/menu_items", body)
            case.records.append(direct_record)
            item = next((item for item in (source or {}).get("menu_items", []) if item.get("id") == course_id), None)
            case.check("current_source_course_found", item is not None, course_id)
            if not item:
                return
            keys = ("id", "price", "prev_price", "price_mode", "tax_type", "service_fee_type", "fine_print_translations",
                    "days", "meals", "valid_date_ranges", "valid_time_ranges", "min_time_cutoff_at", "qty_remaining",
                    "cancel_fee_rules", "questions", "pax_prices", "payment_type", "service_category_ids")
            for key in keys:
                if key != "min_time_cutoff_at":
                    case.check("source_" + key, detail.get(key) == item.get(key), detail.get(key), item.get(key))
            cached_item, cache_proof = common.cached_menu_item(cache, body, course_id)
            cutoff = "min_time_cutoff_at"
            case.check("cached_source_min_time_cutoff_at_exact", detail.get(cutoff) == cached_item.get(cutoff),
                       detail.get(cutoff), cached_item.get(cutoff))
            cutoff_evidence = common.cutoff_comparison(detail.get(cutoff), item.get(cutoff))
            case.check("independent_min_time_cutoff_at_within_one_second", cutoff_evidence["within_one_second"],
                       cutoff_evidence, "offset-aware timestamps within <=1 second")
            cutoff_evidence.update(cache_proof)
            cutoff_evidence["cached_source_exact"] = cached_item.get(cutoff)
            cutoff_evidence["preservation_rule"] = "CLI value must equal its own raw cached response exactly; no CLI normalization"
            cutoff_evidence["source_observation_note"] = "Independent responses can recompute cutoff fractional seconds; all other conditions are compared exactly."
            common.save_json(runner.output / "course-cutoff-preservation-proof.json", cutoff_evidence)
            case.check("no_inferred_group_order_price_basis", detail.get("price_basis") == item.get("price_basis"), detail.get("price_basis"))
            case.check("no_fabricated_total", detail.get("computed_total") is None, detail.get("computed_total"))
            english = next((row.get("translation") for row in item.get("fine_print_translations", []) if row.get("locale") == "en"), None)
            case.check("selected_fine_print_preserved", detail.get("fine_print") == english, detail.get("fine_print"), english)
            current["historical_reference"] = {"course_id": common.KNOWN_COURSE_ID, "present_in_current_catalog": current.get("historical_course_present"),
                                               "historical_snapshot_price": "19800.0", "immutable_price_assumed": False,
                                               "compared_current_course_id": course_id, "compared_current_price": item.get("price")}
            common.save_json(runner.output / "source-menu-projection.json", {"request": body, "menu_item": {key: item.get(key) for key in keys}})
        run_case("course_price_and_conditions_against_current_source", course_detail)

        def calendar(case, party):
            data, record = runner.cli("calendar-party%d" % party, ["availability", "check", common.VENUE, "--date", day,
                                     "--time", "18:00", "--party", str(party), "--include-unavailable", "--limit", "50"], cache, 4)
            case.records.append(record)
            rows = (data or {}).get("checks", [])
            case.check("one_requested_date_row", len(rows) == 1, len(rows))
            row = rows[0] if rows else {}
            case.check("date_party_venue_scope", row.get("date") == day and row.get("party") == party and row.get("venue_id") == common.VENUE_ID and row.get("scope") == "venue", {key: row.get(key) for key in ("date", "party", "venue_id", "scope")})
            start = dt.datetime.combine(dt.date.fromisoformat(day), dt.time(18), common.TOKYO)
            body = {"shop_id": common.VENUE_ID, "locale": "en", "start_at": start.astimezone(dt.timezone.utc).isoformat().replace("+00:00", "Z"), "num_people": party}
            source, direct_record = runner.direct("source-calendar-party%d" % party, "/v2/hub/availability_calendar_v2", body)
            case.records.append(direct_record)
            if source is None:
                return
            source_points = common.source_slots(source, day)
            points = {point["starts_at"]: point for point in source_points}
            compared = []
            for slot in row.get("slots", []):
                stamp = dt.datetime.fromisoformat(slot["starts_at"].replace("Z", "+00:00")).astimezone(dt.timezone.utc)
                converted = stamp.astimezone(common.TOKYO)
                case.check("slot_local_date_" + slot.get("local_time", "unknown"), converted.date().isoformat() == day and slot.get("local_time") == converted.strftime("%H:%M"), slot)
                if stamp in points:
                    point = points[stamp]
                    case.check("independent_boolean_" + slot.get("local_time", "unknown"), slot.get("is_available") is point["is_available"], slot.get("is_available"), point["is_available"])
                    compared.append(slot.get("local_time"))
            case.check("at_least_one_observed_boolean_compared", bool(compared), compared)
            case.check("Tokyo_time_zone", row.get("time_zone") == "Asia/Tokyo", row.get("time_zone"))
            requested = next((point["is_available"] for point in source_points if point["local_time"] == "18:00"), None)
            closed = day in source.get("availability_calendar", {}).get("closed_dates", [])
            expected_status = "closed" if closed else "available" if requested is True else "unavailable" if requested is False else "unknown"
            case.check("exact_requested_time_status", row.get("status") == expected_status, row.get("status"), expected_status)
            case.check("false_is_not_invented_sold_out_or_waitlist", row.get("status") not in ("sold_out", "waitlist"), row.get("status"))
            common.save_json(runner.output / ("source-calendar-party%d-projection.json" % party), {"request": body, "calendar": common.calendar_projection(source, [day, tomorrow])})
            current["party%d_source" % party] = source
            current["party%d" % party] = {"source_available_count": sum(point["is_available"] for point in source_points),
                                            "compared_local_times": compared, "requested_time_available": requested,
                                            "cli_status": row.get("status")}
        run_case("party2_calendar_independent_comparison", lambda case: calendar(case, 2))

        def default_calendar_window(case):
            # This no-time read must share the existing party2/18:00 cache key.
            # The preceding independent HTTP response is reused, without a new source call.
            data, record = runner.cli("calendar-default-dinner", ["availability", "check", common.VENUE,
                                     "--date", day, "--party", "2", "--include-unavailable", "--limit", "50"], cache, 1)
            case.records.append(record)
            rows = (data or {}).get("checks", [])
            case.check("one_default_date_row", len(rows) == 1, len(rows))
            row = rows[0] if rows else {}
            case.check("omitted_preference_remains_null", row.get("requested_time") is None and row.get("requested_time_available") is None,
                       {key: row.get(key) for key in ("requested_time", "requested_time_available")})
            case.check("default_dinner_anchor", row.get("query_anchor_time") == "18:00", row.get("query_anchor_time"), "18:00")
            case.check("default_reuses_existing_18_calendar", record["requests"] == 0, record["requests"], 0)
            source = current.get("party2_source")
            if source is None:
                raise ValueError("preceding independent party2 source response unavailable")
            source_points = common.source_slots(source, day)
            points = {point["starts_at"]: point for point in source_points}
            matched_times = []
            source_calendar = source.get("availability_calendar", {})
            for stamp in source_calendar.get("data", {}).get(day, {}):
                local = dt.datetime.fromisoformat(stamp.replace("Z", "+00:00")).astimezone(common.TOKYO)
                if local.date().isoformat() == day:
                    matched_times.append(local.strftime("%H:%M"))
            matched_times.sort()
            coverage = row.get("coverage") or {}
            case.check("window_not_full_day", coverage.get("time_scope") == "source_time_window" and coverage.get("full_day") is False, coverage)
            expected_from = matched_times[0] if matched_times else None
            expected_to = matched_times[-1] if matched_times else None
            case.check("bounds_are_actual_source_times", coverage.get("returned_time_from") == expected_from and coverage.get("returned_time_to") == expected_to,
                       {key: coverage.get(key) for key in ("returned_time_from", "returned_time_to")}, [expected_from, expected_to])
            compared = []
            for slot in row.get("slots", []):
                stamp = dt.datetime.fromisoformat(slot["starts_at"].replace("Z", "+00:00")).astimezone(dt.timezone.utc)
                local = stamp.astimezone(common.TOKYO)
                case.check("default_slot_local_date_" + slot.get("local_time", "unknown"), local.date().isoformat() == day and slot.get("local_time") == local.strftime("%H:%M"))
                if stamp in points:
                    case.check("default_independent_boolean_" + slot.get("local_time", "unknown"), slot.get("is_available") is points[stamp]["is_available"],
                               slot.get("is_available"), points[stamp]["is_available"])
                    compared.append(slot.get("local_time"))
            case.check("default_observed_boolean_compared", bool(compared), compared)
            closed = day in source_calendar.get("closed_dates", [])
            any_available = any(point["is_available"] for point in source_points)
            all_known_false = bool(source_points) and len(source_points) == len(matched_times) and not any_available
            expected_status = "closed" if closed else "available" if any_available else "unavailable" if all_known_false else "unknown"
            case.check("default_any_slot_status", row.get("status") == expected_status, row.get("status"), expected_status)
            common.save_json(runner.output / "default-dinner-window-proof.json", {
                "independent_response_reused": "source-calendar-party2-projection.json",
                "query_anchor_time": row.get("query_anchor_time"), "requested_time": row.get("requested_time"),
                "coverage": coverage, "compared_local_times": compared, "status": row.get("status"),
                "source_available_local_times": [point["local_time"] for point in source_points if point["is_available"]]})
        run_case("default_dinner_time_window_without_exact_preference", default_calendar_window)
        run_case("party20_calendar_independent_comparison", lambda case: calendar(case, 20))

        def booking(case):
            data, record = runner.cli("booking-url", ["booking-url", common.VENUE, "--date", day, "--time", "18:00", "--party", "2"], cache, 2)
            case.records.append(record)
            value = data or {}
            parsed = urlparse(value.get("booking_url") or "")
            mode = current.get("venue", {}).get("booking_page_mode")
            expected = "/en/" + common.VENUE + "/reserve/landing" if mode == "v2" else "/en/shops/" + common.VENUE + "/reserve" if mode == "v1" else None
            case.check("canonical_official_handoff", parsed.scheme == "https" and parsed.netloc == "www.tablecheck.com" and parsed.path == expected, parsed.geturl())
            case.check("exact_current_portal_query", parse_qs(parsed.query) == {"start_date": [day], "start_time": ["18:00"], "num_people": ["2"]}, parse_qs(parsed.query))
            case.check("no_booking_or_availability_guarantee", value.get("reservation_created") is False and value.get("availability_checked") is False)
        run_case("canonical_booking_handoff", booking)

        def partial_scan(case):
            invalid = "printing-press-e2e-no-such-venue-20260927"
            data, record = runner.cli("partial-scan", ["availability", "scan", common.VENUE, invalid, "--from", day, "--to", tomorrow,
                                     "--time", "18:00", "--party", "2", "--limit", "5"], cache, 6, expected_exit="nonzero")
            case.records.append(record)
            value = data or {}
            rows = value.get("checks", [])
            keys = {(row.get("slug"), row.get("date")) for row in rows}
            expected = {(slug, date) for slug in (common.VENUE, invalid) for date in (day, tomorrow)}
            case.check("all_four_requested_rows_preserved", len(rows) == 4 and keys == expected, sorted(keys))
            case.check("bad_venue_is_failed_not_empty_inventory", all(row.get("status") == "failed" and row.get("error") for row in rows if row.get("slug") == invalid))
            case.check("good_venue_rows_retained", all(row.get("status") != "failed" for row in rows if row.get("slug") == common.VENUE))
            case.check("failure_list_visible", any(failure.get("slug") == invalid for failure in value.get("fetch_failures", [])), value.get("fetch_failures"))
        run_case("bounded_two_venue_two_day_partial_scan", partial_scan)
    except Exception as error:
        errors.append("%s: %s" % (type(error).__name__, error))
    report = {"harness": "tablecheck-planning-live", "started_at": started, "finished_at": dt.datetime.now(dt.timezone.utc).isoformat(),
              "passed": not errors and bool(cases) and all(case["passed"] for case in cases), "cases": cases, "errors": errors,
              "request_budget": budget.report(), "invocations": runner.records if runner else [],
              "historical_reference": current.get("historical_reference"),
              "party_comparison": {key: current.get(key) for key in ("party2", "party20")},
              "limits": {"cli_wall_seconds": 35, "independent_http_wall_seconds": 10, "payload_bytes": common.MAX_PAYLOAD,
                         "direct_retries": 0, "cli_retries_max": 1, "bookings_created": 0}}
    if runner:
        common.save_json(runner.output / "live-report.json", report)
    print(json.dumps(report, ensure_ascii=False))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
