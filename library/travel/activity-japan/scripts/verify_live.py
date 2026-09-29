#!/usr/bin/env python3
"""Live read-only acceptance against Activity Japan plan JSON and sitemaps."""
import argparse
import datetime as dt
import json
import subprocess
import urllib.parse
import urllib.request

BASE = "https://gd.activityjapan.com"
LIMIT = 2 << 20


def source(path, **query):
    url = BASE + path + "?" + urllib.parse.urlencode(query)
    request = urllib.request.Request(url, headers={"User-Agent": "activity-japan-pp-cli-live-proof/1.0"})
    with urllib.request.urlopen(request, timeout=10) as response:
        body = response.read(LIMIT + 1)
        assert response.status == 200 and len(body) <= LIMIT, (url, response.status, len(body))
        assert "json" in response.headers.get("Content-Type", "").lower(), url
    return json.loads(body)


def cli(binary, *args, ok=True):
    result = subprocess.run([binary, *args], capture_output=True, text=True, timeout=27)
    if ok:
        assert result.returncode == 0, (args, result.stderr[:250])
        document = json.loads(result.stdout)
        assert document["meta"]["provider"] == "activity-japan", args
        assert document["meta"]["reservation_confirmed"] is False, args
        return document["results"], document["meta"]
    assert result.returncode != 0 and not result.stdout.strip(), (args, result.returncode, result.stdout[:100])
    return result.stderr[:250], None


def check(name, condition, observed):
    assert condition, (name, observed)
    return {"check": name, "pass": True, "observed": observed}


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--binary", required=True)
    p.add_argument("--date", default="2026-10-08")
    p.add_argument("--request-date", default="2026-10-01")
    args = p.parse_args()
    checks = []
    kyoto, meta = cli(args.binary, "experience", "detail", "62375", "--lang", "en", "--agent")
    en = source("/get_plan_price_info", plan_id="62375", lang_flag="en", url="https://en.activityjapan.com/publish/plan/62375")
    ja = source("/get_plan_price_info", plan_id="62375", lang_flag="ja", url="https://activityjapan.com/publish/plan/62375")
    checks.append(check("Kyoto search-to-detail identity and original name", kyoto["plan_id"] == str(en["plan_data"]["plan_id"]) == str(ja["plan_data"]["plan_id"]) and kyoto["name_original_ja"] == ja["plan_data"]["plan_name"] and kyoto["operator_id"] == str(en["plan_data"]["partner_id"]), {"plan_id": kyoto["plan_id"], "operator_id": kyoto["operator_id"], "observed_at": meta["observed_at"]}))
    checks.append(check("Kyoto headline versus options", kyoto["derived_headline_from_jpy"] == en["plan_data"]["base_price"] - en["plan_data"]["discount_price"] == 1800 and any(o["option_id"] == "176511" for o in kyoto["options"]), {"headline_jpy": kyoto["derived_headline_from_jpy"], "option_ids": [o["option_id"] for o in kyoto["options"]]}))

    sushi, _ = cli(args.binary, "experience", "price", "62061", "--date", args.date, "--adults", "2", "--agent")
    sushi_source = source("/get_plan_price_info", plan_id="62061", lang_flag="en", url="https://en.activityjapan.com/publish/plan/62061")
    dated = source("/plan/get_plan_price", plan_id="62061", date=args.date)
    raw_quotes = {str(v["price_id"]): v["price"] for group in dated["planPriceList"].values() for v in group}
    quotes = {q["option_id"]: q for q in sushi["selected_date_quotes"]}
    checks.append(check("Osaka sushi source price IDs and adult age band", sushi["derived_headline_from_jpy"] == sushi_source["plan_data"]["base_price"] == 9000 and all(quotes[id]["selected_date_unit_price_jpy"] == price for id, price in raw_quotes.items()) and quotes["175524"]["derived_subtotal_jpy"] == 18000 and quotes["175525"]["derived_subtotal_jpy"] is None and quotes["175526"]["derived_subtotal_jpy"] is None, {"headline_jpy": sushi["derived_headline_from_jpy"], "adult_subtotal_jpy": quotes["175524"]["derived_subtotal_jpy"], "child_subtotal": quotes["175525"]["derived_subtotal_jpy"], "infant_subtotal": quotes["175526"]["derived_subtotal_jpy"]}))

    okinawa, _ = cli(args.binary, "experience", "detail", "2044", "--agent")
    okinawa_source = source("/get_plan_price_info", plan_id="2044", lang_flag="en", url="https://en.activityjapan.com/publish/plan/2044")
    checks.append(check("Okinawa outdoor plan identity and mixed duration prose", okinawa["plan_id"] == str(okinawa_source["plan_data"]["plan_id"]) and okinawa["age_min_years"] == okinawa_source["plan_data"]["age_start"] and okinawa["derived_total_minutes"] is None, {"plan_id": okinawa["plan_id"], "age_min_years": okinawa["age_min_years"], "derived_total_minutes": None}))

    sessions, _ = cli(args.binary, "experience", "sessions", "62375", "--date", args.date, "--limit", "50", "--agent")
    source_sessions = source("/select_plan_course", plan_id="62375", selected_date=args.date)
    raw_status = {str(k): str(v) for k, v in source_sessions["course_status"].items()}
    observed_sessions = sessions["sessions"]
    checks.append(check("Dated session IDs, statuses and pagination", all(raw_status[s["session_id"]] == s["source_status"] for s in observed_sessions) and sessions["pagination"]["total"] == len(source_sessions["course_name"]) and all(s["start_local"].endswith("+09:00") for s in observed_sessions), {"total": sessions["pagination"]["total"], "states": sorted({s["availability"] for s in observed_sessions})}))
    instant = next(s for s in observed_sessions if s["source_status"] == "1")
    stock, _ = cli(args.binary, "experience", "check", "62375", "--date", args.date, "--session", instant["session_id"], "--adults", "2", "--agent")
    checks.append(check("Dated party stock remains unconfirmed", stock["plan_id"] == "62375" and stock["session_id"] == instant["session_id"] and stock["participants"] == 2 and stock["reservation_confirmed"] is False and stock["source_result"] in ("1", "2", "3", "4", "5"), {"session_id": stock["session_id"], "result": stock["source_result"], "availability": stock["availability"]}))

    request_sessions, _ = cli(args.binary, "experience", "sessions", "64974", "--date", args.request_date, "--agent")
    request_raw = source("/select_plan_course", plan_id="64974", selected_date=args.request_date)
    checks.append(check("Fukuoka reservation-request state", any(s["availability"] == "reservation_request" and str(request_raw["course_status"][s["session_id"]]) == "3" for s in request_sessions["sessions"]), {"date": args.request_date, "states": sorted({s["availability"] for s in request_sessions["sessions"]})}))
    pair, _ = cli(args.binary, "experience", "price", "64974", "--date", "2026-10-03", "--adults", "2", "--agent")
    checks.append(check("Pair unit prices one group, not two people", pair["selected_date_quotes"][0]["basis"] == "per_group" and pair["selected_date_quotes"][0]["group_size"] == 2 and pair["selected_date_quotes"][0]["derived_subtotal_jpy"] == pair["selected_date_quotes"][0]["selected_date_unit_price_jpy"], {"option_id": pair["selected_date_quotes"][0]["option_id"], "subtotal_jpy": pair["selected_date_quotes"][0]["derived_subtotal_jpy"]}))
    err, _ = cli(args.binary, "experience", "check", "64974", "--date", "2026-10-03", "--session", "0", "--adults", "2", "--agent", ok=False)
    checks.append(check("Group stock count refuses unverified basis", "unverified" in err, {"error": err.strip()}))

    dates, _ = cli(args.binary, "experience", "dates", "62375", "--dates", "2026-10-07,2026-10-08", "--agent")
    rows = dates["dates"]
    checks.append(check("Distinct selected-date prices", rows[0]["option_prices"][0]["unit_price_jpy"] == 2000 and rows[1]["option_prices"][0]["unit_price_jpy"] == 1800 and dates["partial"] is False, {"dates": [r["date"] for r in rows], "option_176511_jpy": [r["option_prices"][0]["unit_price_jpy"] for r in rows]}))
    comparison, _ = cli(args.binary, "experience", "compare", "62375", "2044", "--max-minutes", "90", "--agent")
    verdicts = {p["plan_id"]: p["verdicts"]["total_duration"] for p in comparison["plans"]}
    checks.append(check("Duration comparison preserves unknown", verdicts == {"62375": "match", "2044": "unknown"}, verdicts))
    coverage, _ = cli(args.binary, "inventory", "languages", "62375", "--agent")
    checks.append(check("Separate English and Japanese sitemap presence", coverage["english_indexed"] and coverage["japanese_indexed"] and coverage["english_plan_url_count"] > 10000 and coverage["japanese_plan_url_count"] > coverage["english_plan_url_count"], {"en_count": coverage["english_plan_url_count"], "ja_count": coverage["japanese_plan_url_count"]}))
    handoff, _ = cli(args.binary, "experience", "handoff", "62375", "--lang", "en", "--agent")
    checks.append(check("Canonical English handoff", handoff["canonical_url"] == coverage["english_url"] and handoff["indexed_in_sitemap"], {"url": handoff["canonical_url"]}))
    bad, _ = cli(args.binary, "experience", "detail", "99999999", "--agent", ok=False)
    checks.append(check("Invalid plan HTML is an error", "HTML" in bad or "invalid JSON" in bad, {"error": bad.strip()}))
    print(json.dumps({"observed_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(), "date": args.date, "checks": checks, "passed": len(checks)}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
