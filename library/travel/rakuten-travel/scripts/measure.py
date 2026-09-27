#!/usr/bin/env python3
"""Measure live CLI output bytes, network/cache work, latency and per-run RSS."""
import argparse
import datetime as dt
import json
import platform
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time
from zoneinfo import ZoneInfo


def default_checkin():
    day = dt.datetime.now(ZoneInfo("Asia/Tokyo")).date() + dt.timedelta(days=42)
    return day + dt.timedelta(days=(6 - day.weekday()) % 7)


RESOURCE_WRAPPER = """
import resource, subprocess, sys
completed = subprocess.run(sys.argv[1:], check=False)
peak = resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss
sys.stderr.write("\\nPP_MEASURE_PEAK_RSS=" + str(peak) + "\\n")
raise SystemExit(completed.returncode)
"""


def measure(binary, home, evidence, label, arguments):
    command = [str(binary), *arguments, "--home", home, "--timeout", "60s", "--json"]
    darwin_time = platform.system() == "Darwin" and Path("/usr/bin/time").is_file()
    wrapped = ["/usr/bin/time", "-l", *command] if darwin_time else [sys.executable, "-c", RESOURCE_WRAPPER, *command]
    started = time.monotonic()
    try:
        process = subprocess.run(wrapped, capture_output=True, timeout=85, check=False)
        status, stdout, stderr = process.returncode, process.stdout, process.stderr
    except subprocess.TimeoutExpired as exc:
        status, stdout, stderr = 124, exc.stdout or b"", (exc.stderr or b"") + b"\nharness timeout"
    elapsed = time.monotonic() - started
    (evidence / (label + ".stdout.json")).write_bytes(stdout)
    (evidence / (label + ".stderr.txt")).write_bytes(stderr)
    try:
        payload = json.loads(stdout)
    except (ValueError, UnicodeDecodeError):
        payload = {}
    meta = payload.get("meta", {}) if isinstance(payload, dict) else {}
    source = meta.get("source_info", {}) or {}
    statistics = meta.get("requests", {}) or {}
    pattern = rb"([0-9]+)\s+maximum resident set size" if darwin_time else rb"PP_MEASURE_PEAK_RSS=([0-9.]+)"
    peak_match = re.search(pattern, stderr)
    raw_peak = float(peak_match.group(1)) if peak_match else None
    raw_unit = "bytes" if platform.system() == "Darwin" else "KiB"
    peak_bytes = int(raw_peak * (1 if raw_unit == "bytes" else 1024)) if raw_peak is not None else None
    rows = payload.get("results") if isinstance(payload, dict) else None
    return {"label": label, "arguments": arguments, "mode": "live_public_network",
            "exit_code": status, "stdout_bytes": len(stdout),
            "wall_seconds": round(elapsed, 6), "network_requests": statistics.get("requests"),
            "retries": statistics.get("retries"), "response_bytes": statistics.get("bytes"),
            "cache_hits": statistics.get("cache_hits"), "network_latency_ms": statistics.get("network_latency_ms"),
            "peak_rss_bytes": peak_bytes, "peak_rss_raw": raw_peak, "peak_rss_raw_unit": raw_unit,
            "peak_rss_method": "/usr/bin/time -l" if darwin_time else "isolated Python resource wrapper",
            "wall_includes_launcher_overhead": not darwin_time,
            "source": meta.get("source"), "source_cache_state": source.get("cache_state"),
            "observed_at": source.get("observed_at"), "source_fetched_at": source.get("fetched_at"),
            "result_status": meta.get("status"),
            "emitted_rows": len(rows) if isinstance(rows, list) else None,
            "stdout_artifact": label + ".stdout.json", "stderr_artifact": label + ".stderr.txt"}


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
    in_date = args.checkin or default_checkin()
    offer = ["offers", "search", "--hotel", "51870", "--checkin", in_date.isoformat(),
             "--checkout", (in_date + dt.timedelta(days=2)).isoformat(),
             "--rooms", "1", "--adults-per-room", "2", "--limit", "5"]
    environment = {"mode": "live_public_network", "started_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
                   "platform": platform.platform(), "python": platform.python_version(),
                   "binary": str(binary), "checkin": in_date.isoformat(), "nights": 2,
                   "rss_scope": "individual CLI invocation including its child processes",
                   "network_scope": "actual outbound attempts reported by the public-source client",
                   "fixture_benchmark": False}
    with tempfile.TemporaryDirectory(prefix="rakuten-travel-measure-") as home:
        scenarios = [
            ("metadata_cold", ["hotels", "show", "--hotel", "51870"]),
            ("metadata_cache_hit", ["hotels", "show", "--hotel", "51870"]),
            ("inventory_live", offer + ["--no-cache"]),
            ("inventory_explicit_cache_cold", offer + ["--inventory-cache-seconds", "60", "--refresh"]),
            ("inventory_explicit_cache_hit", offer + ["--inventory-cache-seconds", "60"]),
        ]
        runs = []
        last_start = 0.0
        for label, arguments in scenarios:
            remaining = 1.0 - (time.monotonic() - last_start)
            if remaining > 0:
                time.sleep(remaining)
            last_start = time.monotonic()
            runs.append(measure(binary, home, evidence, label, arguments))

        by_label = {run["label"]: run for run in runs}
        assertions = []
        def check(name, condition):
            assertions.append({"name": name, "passed": bool(condition)})
        for run in runs:
            check(run["label"] + "_success_metrics", run["exit_code"] == 0 and run["stdout_bytes"] > 0
                  and isinstance(run["network_requests"], int) and run["peak_rss_bytes"] is not None)
        for cold, hit in (("metadata_cold", "metadata_cache_hit"),
                          ("inventory_explicit_cache_cold", "inventory_explicit_cache_hit")):
            check(cold + "_network_observed", (by_label[cold]["network_requests"] or 0) > 0)
            check(hit + "_zero_network", by_label[hit]["network_requests"] == 0 and (by_label[hit]["cache_hits"] or 0) > 0)
            check(hit + "_original_observation", bool(by_label[cold]["observed_at"])
                  and by_label[cold]["observed_at"] == by_label[hit]["observed_at"])
        check("inventory_live_not_cached", (by_label["inventory_live"]["network_requests"] or 0) > 0
              and by_label["inventory_live"]["source_cache_state"] == "disabled")
        check("actual_inventory_measured", (by_label["inventory_live"]["emitted_rows"] or 0) > 0
              and (by_label["inventory_explicit_cache_cold"]["emitted_rows"] or 0) > 0)
        report = {"environment": environment, "runs": runs, "assertions": assertions,
                  "passed": all(check["passed"] for check in assertions)}
        (evidence / "live-measurements.json").write_text(json.dumps(report, indent=2) + "\n")
        lines = ["# Live public-source measurements", "",
                 f"Environment: {environment['platform']}; check-in {in_date}; two nights.",
                 "These are true public-network measurements, not fixture benchmarks. Homes are isolated and removed after the run.",
                 "Wall time includes CLI startup and parsing; RSS uses the recorded per-invocation method and units.", "",
                 "| Scenario | Stdout bytes | Requests | Cache hits | Wall seconds | Peak RSS bytes |",
                 "|---|---:|---:|---:|---:|---:|"]
        for run in runs:
            lines.append(f"| {run['label']} | {run['stdout_bytes']} | {run['network_requests']} | {run['cache_hits']} | {run['wall_seconds']} | {run['peak_rss_bytes']} |")
        failed = [check["name"] for check in assertions if not check["passed"]]
        lines += ["", f"Result: {'PASS' if report['passed'] else 'UNSATISFIED'}."]
        lines += [f"- Unsatisfied: {name}" for name in failed]
        (evidence / "live-measurements.md").write_text("\n".join(lines) + "\n")
        print(json.dumps({"passed": report["passed"], "unsatisfied": failed, "report": str(evidence / "live-measurements.json")}))
        return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
