"""Bounded read-only acceptance against the real public Japan Guide source."""
import json
import pathlib
import re
import subprocess
import time

ROOT = pathlib.Path(__file__).resolve().parent.parent
EVIDENCE = ROOT / "evidence"
BINARY = ROOT / "japan-guide-pp-cli"
REPORT = []


def run(name, args, check, expected_exit=0):
    started = time.monotonic()
    command = [str(BINARY), *args, "--agent", "--no-learn"]
    process = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, timeout=40)
    (EVIDENCE / f"live-{name}.json").write_text(process.stdout)
    if process.stderr:
        (EVIDENCE / f"live-{name}.stderr").write_text(process.stderr)
    assert process.returncode == expected_exit, (name, process.returncode, process.stderr[-600:])
    result = json.loads(process.stdout) if process.stdout else None
    value = result.get("results", result) if isinstance(result, dict) else result
    check(value, result)
    row = {
        "case": name,
        "status": "PASS",
        "command": command[1:],
        "json_bytes": len(process.stdout.encode()),
        "wall_ms": round((time.monotonic() - started) * 1000),
        "metrics": value.get("metrics") if isinstance(value, dict) else None,
        "evidence": f"evidence/live-{name}.json",
    }
    REPORT.append(row)
    print(json.dumps(row), flush=True)
    return value


def require(condition, message):
    assert condition, message


destinations = run("destinations", ["guide", "destinations", "--region", "kanto", "--limit", "5"],
    lambda v, _: require(v["items"][0]["id"] == "e2164" and all(x["region"] == "Kanto" and x["kind"] == "destination" for x in v["items"]) and v["metrics"]["upstream_requests"] == 1, "destination region/identity"))
require(destinations["next_offset"] == 5, "destination pagination continuation")

temples = run("temples", ["guide", "attractions", "e2164", "--interest", "temples", "--limit", "5"],
    lambda v, _: require(any(x["id"] == "e3001" for x in v["items"]) and all("Temples" in x["interests"] for x in v["items"]) and v["metrics"]["upstream_requests"] == 1, "source interest relevance"))
require(all(x["editorial_recommendation"]["scale"] == 3 and "editorial" in x["editorial_recommendation"]["kind"] for x in temples["items"] if x.get("editorial_recommendation")), "editorial/visitor distinction")

run("no-match", ["guide", "attractions", "e2164", "--query", "no-such-place-xyz"],
    lambda v, _: require(v["items"] == [] and v["total_matches"] == 0 and v["scanned_records"] > 0 and v["note"], "honest no-match coverage"))

run("category-identity", ["guide", "attractions", "e2164", "--offset", "50", "--limit", "50"],
    lambda v, _: require(any(x["id"] == "e3063" and x["kind"] == "event" for x in v["items"]) and any("Side Trips" in x["category"] and x["kind"] == "destination" for x in v["items"]), "event versus side-trip identity"))

run("selected-temples", ["guide", "attractions", "e2164", "--interest", "temples", "--limit", "3", "--select", "items"],
    lambda v, e: require(isinstance(v, list) and len(v) > 0 and len(v) <= 3 and e["meta"]["source"] == "live", "selected array/agent provenance"))

run("interests", ["guide", "interests", "--query", "temples", "--limit", "3"],
    lambda v, _: require(len(v["items"]) == 1 and v["items"][0]["id"] == "e2058" and v["items"][0]["kind"] == "interest", "source interest directory"))

sensoji = run("sensoji", ["guide", "inspect", "e3001", "--cache", "--cache-dir", str(EVIDENCE / "facts-cache")],
    lambda v, _: require(v["item"]["japanese_name"] == "浅草寺" and v["item"]["open_now"] is None and v["item"]["visit_information"] and v["item"]["source_updated"], "Japanese source facts/opening uncertainty"))

run("offline", ["guide", "inspect", "e3001", "--offline", "--cache-dir", str(EVIDENCE / "facts-cache")],
    lambda v, e: require(v["item"]["retrieved_at"] == sensoji["item"]["retrieved_at"] and v["retrieved_at"] == sensoji["item"]["retrieved_at"] and v["item"]["source_updated"] == sensoji["item"]["source_updated"] and v["item"]["freshness"] == "offline_snapshot" and e["meta"]["source"] == "local" and v["metrics"]["upstream_requests"] == 0, "original offline timestamps/provenance"))

run("meiji", ["guide", "inspect", "e3002"],
    lambda v, _: require([x["facility"] for x in v["item"]["visit_information"]] == ["Meiji Shrine", "Meiji Jingu Museum", "Inner Garden"] and v["item"]["visit_information"][1]["admission"] == "1000 yen" and v["item"]["visit_information"][2]["admission"] == "500 yen" and any("November" in x for x in v["item"]["seasonal_notes"]), "scoped facilities/JPY/seasonal qualifiers"))

run("construction", ["guide", "inspect", "e3911"],
    lambda v, _: require(any("2026" in x and "2027" in x and "closed" in x for x in v["item"]["planning_notices"]), "dated construction closure notice"))

run("event", ["guide", "inspect", "e3063"],
    lambda v, _: require(v["item"]["kind"] == "event" and v["item"]["japanese_name"] == "三社祭" and any(re.search(r"\b20\d{2}\b", x) for x in v["item"]["dated_notes"]) and v["item"]["open_now"] is None, "source event identity/date year/Japanese alias"))

run("marathon", ["guide", "inspect", "e2264"],
    lambda v, _: require(v["item"]["kind"] == "event" and any(x["kind"] == "annual_recurrence" and x["explicit_year"] is None and "first Sunday of March" in x["statement"] for x in v["item"]["event_calendar"]), "source event recurrence without invented year"))

run("jidai", ["guide", "inspect", "e3960"],
    lambda v, _: require(v["item"]["kind"] == "event" and any("October 22" in x["statement"] and x["explicit_year"] is None for x in v["item"]["event_calendar"]) and v["item"]["visit_information"][0]["facility"] == "Paid Seating" and v["item"]["visit_information"][0]["admission"] == "5000-7500 yen" and any("September 8, 2026" in x for x in v["item"]["dated_notes"]), "scoped narrative seating fee and dated release qualifier"))

run("interest-detail", ["guide", "inspect", "e2058"],
    lambda v, _: require(v["item"]["kind"] == "interest", "interest page versus destination"))

run("itinerary-index", ["guide", "itineraries", "--limit", "5"],
    lambda v, _: require(any(x["id"] == "e2400_kanto" for x in v["items"]) and all(x["kind"] == "source_itinerary" for x in v["items"]), "regional source plan index"))

run("local-itineraries", ["guide", "itineraries", "--destination", "e2164", "--limit", "5"],
    lambda v, _: require(any(x["id"] == "e3051_west_tokyo_full" and x["duration"] == "1 Day" for x in v["items"]), "canonical local plan discovery"))

run("regional-plan", ["guide", "itinerary", "e2400_kanto"],
    lambda v, _: require(len(v["stops"]) >= 10 and "Tokyo" in v["stops"][0]["label"] and any(any("winter" in n and "closed" in n for n in x["planning_notes"]) for x in v["stops"]) and v["source_updated"], "source regional plan/winter closure"))

run("local-plan", ["guide", "itinerary", "e3051_west_tokyo_full"],
    lambda v, _: require(len(v["stops"]) == 5 and v["stops"][0]["label"] == "Meiji Shrine" and v["stops"][0]["source_visit_duration"] == "1 hour" and all(x["source_links"] is not None for x in v["stops"]), "local stop sequence/durations"))

run("compare-partial", ["guide", "compare", "e3001", "e999999"],
    lambda v, _: require(v["partial"] and v["successful"] == 1 and len(v["items"]) == 1 and len(v["fetch_failures"]) == 1 and v["metrics"]["upstream_requests"] == 2, "live partial failure accounting"))

run("compare-quoted", ["guide", "compare", "e3001 e3002", "--rate-limit", "0"],
    lambda v, _: require(v["requested"] == v["successful"] == 2 and not v["partial"] and v["metrics"]["upstream_requests"] == 2, "space-separated comparison contract"))

for row in REPORT:
    data = json.loads((ROOT / row["evidence"]).read_text())
    value = data.get("results", data)
    encoded = json.dumps(value)
    require("booking.com" not in encoded and "user_rating" not in encoded, "single-source planning facts only")

measured = subprocess.run(["/usr/bin/time", "-l", str(BINARY), "guide", "destinations", "--region", "kanto", "--limit", "5", "--agent", "--no-learn"],cwd=ROOT,capture_output=True,text=True,timeout=40)
require(measured.returncode == 0, measured.stderr[-600:])
(EVIDENCE / "resource-memory.txt").write_text(measured.stderr)
rss = re.search(r"(\d+)\s+maximum resident set size", measured.stderr)
memory = {"representative_command": "guide destinations --region kanto --limit 5 --agent", "maximum_resident_bytes": int(rss.group(1)) if rss else None, "json_bytes": len(measured.stdout.encode()), "metrics": json.loads(measured.stdout)["results"]["metrics"]}
report = {"status": "PASS", "source": "https://www.japan-guide.com/", "checked_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "case_count": len(REPORT), "cases": REPORT, "resource_measurement": memory}
(EVIDENCE / "live-acceptance.json").write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps({"status": "PASS", "cases": len(REPORT), "resource_measurement": memory}), flush=True)
