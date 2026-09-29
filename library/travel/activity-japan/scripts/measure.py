#!/usr/bin/env python3
"""Bounded macOS benchmark of representative read-only Activity Japan commands."""
import argparse
import datetime as dt
import json
import re
import subprocess
import time


def measure(binary, label, args):
    started = time.perf_counter()
    proc = subprocess.run(["/usr/bin/time", "-l", binary, *args], capture_output=True)
    elapsed_ms = round((time.perf_counter() - started) * 1000)
    stderr = proc.stderr.decode("utf-8", "replace")
    peak = re.search(r"(\d+)\s+maximum resident set size", stderr)
    try:
        document = json.loads(proc.stdout)
    except json.JSONDecodeError:
        document = {}
    return {
        "case": label,
        "command": " ".join([binary, *args]),
        "exit_code": proc.returncode,
        "stdout_bytes": len(proc.stdout),
        "upstream_requests": document.get("meta", {}).get("upstream_requests"),
        "elapsed_ms": elapsed_ms,
        "peak_rss_bytes": int(peak.group(1)) if peak else None,
        "cache": document.get("meta", {}).get("cache"),
        "error": stderr.splitlines()[0][:180] if proc.returncode else None,
    }


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--binary", required=True)
    p.add_argument("--date", required=True, help="Future Asia/Tokyo date, YYYY-MM-DD")
    args = p.parse_args()
    cases = [
        ("inventory_refresh", ["inventory", "languages", "62375", "--refresh", "--agent"]),
        ("inventory_cached", ["inventory", "languages", "62375", "--agent"]),
        ("detail_uncached", ["experience", "detail", "62375", "--agent"]),
        ("sessions_uncached", ["experience", "sessions", "62375", "--date", args.date, "--agent"]),
        ("compare_uncached", ["experience", "compare", "62375", "2044", "--date", args.date, "--adults", "2", "--max-jpy", "7000", "--agent"]),
    ]
    observations = [measure(args.binary, label, command) for label, command in cases]
    print(json.dumps({"observed_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(), "activity_date": args.date, "measurements": observations}, indent=2))
    return 0 if all(row["exit_code"] == 0 for row in observations) else 1


if __name__ == "__main__":
    raise SystemExit(main())
