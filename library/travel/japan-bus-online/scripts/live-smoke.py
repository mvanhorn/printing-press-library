#!/usr/bin/env python3
"""Capture six real read-only provider samples and bounded resource measurements."""
import argparse
import datetime
import json
import pathlib
import re
import subprocess
import tempfile
import time


parser = argparse.ArgumentParser()
parser.add_argument("--date", default="2026-10-10")
parser.add_argument("--binary", default="build/japan-bus-online-pp-cli")
args = parser.parse_args()
destination = pathlib.Path("evidence/live-final")
destination.mkdir(parents=True, exist_ok=True)
route = ["--route", "12200160001"]
dated = route + ["--direction", "0", "--date", args.date]
party = dated + ["--service", "0001", "--adults", "2", "--children", "1"]
workflows = [
    ("routes", ["routes", "list", "--query", "Hamamatsu", "--limit", "3"]),
    ("route", ["bus", "route"] + route),
    ("services", ["bus", "services"] + dated),
    ("quote", ["bus", "quote"] + party),
    ("quote-segment", ["bus", "quote"] + party + ["--dep-stop", "8", "--arr-stop", "9"]),
    ("conditions", ["bus", "conditions"] + route),
]
measurements = []
with tempfile.TemporaryDirectory(prefix="jbo-live-smoke-") as scoped_home:
    for name, command in workflows:
        invocation = [args.binary] + command + ["--json", "--no-learn", "--home", scoped_home]
        started = time.monotonic()
        process = subprocess.run(["/usr/bin/time", "-l"] + invocation, capture_output=True, text=True, timeout=65)
        latency = round((time.monotonic() - started) * 1000)
        (destination / (name + ".json")).write_text(process.stdout)
        (destination / (name + ".stderr.txt")).write_text(process.stderr)
        if process.returncode:
            raise RuntimeError(f"{name} failed with exit {process.returncode}; see captured evidence")
        data = json.loads(process.stdout)
        assert data["language"] == "en" and data["timezone"] == "Asia/Tokyo"
        assert data["source_url"].startswith("https://japanbusonline.com/en/")
        if name == "route":
            assert data["kind"] == "published_schedule_not_inventory"
        if name == "services":
            assert data["requested_date"] == args.date
            assert data["services"], "No requested-day services; choose an available live target before accepting"
            assert all(s["route_id"] == "12200160001" and s["direction"] == 0 and s["departure_date"] == args.date for s in data["services"])
        if name.startswith("quote"):
            assert "fare_basis" not in data and "headline_fare_basis" in data
            assert data["quote_status"] == "fare_evidence_reported", "No selected-pair fare evidence; choose an available live target before accepting"
            assert data["currency"] == "JPY" and data["trip_type"] == "one_way"
            assert data["estimated_total_jpy"] == 2 * data["fares"]["adult"]["unit_jpy"] + data["fares"]["child"]["unit_jpy"]
            assert data["party"]["requested_seats"] == 3
            if name == "quote-segment":
                assert data["boarding"]["stop_id"] == "8" and data["alighting"]["stop_id"] == "9"
                assert data["boarding"]["source_time"] == "25:00"
                expected_day = datetime.date.fromisoformat(args.date) + datetime.timedelta(days=1)
                assert data["boarding"]["timestamp_jst"] == f"{expected_day}T01:00:00+09:00"
        rss = re.search(r"^\s*(\d+)\s+maximum resident set size\s*$", process.stderr, re.MULTILINE)
        measurements.append({
            "workflow": name, "command_args": command, "fetched_at": data["fetched_at"],
            "output_bytes": len(process.stdout.encode()), "upstream_requests": data["upstream_requests"],
            "upstream_bytes": data["response_bytes"], "latency_ms": latency,
            "peak_rss_bytes": int(rss.group(1)) if rss else None,
        })
        print(json.dumps(measurements[-1]), flush=True)
pathlib.Path("evidence/live-final-metrics.json").write_text(json.dumps(measurements, indent=2) + "\n")
