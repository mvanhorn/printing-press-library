#!/usr/bin/env python3
"""Measure actual cold/warm Walkerplus commands; never substitute cached figures."""
import argparse
import hashlib
import json
import pathlib
import re
import subprocess
import sys
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--binary", default="./walkerplus-pp-cli")
parser.add_argument("--output", default="./measurements")
args = parser.parse_args()
binary = str(pathlib.Path(args.binary).resolve())
with open(binary, "rb") as stream:
    binary_sha256 = hashlib.file_digest(stream, "sha256").hexdigest()
output = pathlib.Path(args.output).resolve()
output.mkdir(parents=True, exist_ok=True)
if sys.platform != "darwin":
    parser.error("peak RSS recording uses macOS /usr/bin/time -l")
cases = {
    "search": ["search", "--prefecture", "kyoto", "--category", "festival", "--from", "2026-10-01", "--to", "2026-10-31", "--limit", "3", "--max-pages", "1"],
    "shortlist": ["shortlist", "--prefecture", "kyoto", "--category", "festival", "--from", "2026-10-01", "--to", "2026-10-31", "--limit", "3", "--max-pages", "1", "--max-details", "3"],
    "event": ["event", "ar0313e603640"],
}
records = []
run = str(time.time_ns())
for name, tokens in cases.items():
    cache = output / ("cache-" + run) / name
    for temperature in ("cold", "warm"):
        command = [binary] + tokens + ["--cache-dir", str(cache), "--timeout", "60s"]
        start = time.perf_counter()
        completed = subprocess.run(["/usr/bin/time", "-l"] + command, capture_output=True)
        wall_ms = round((time.perf_counter() - start) * 1000, 2)
        artifact = output / (name + "-" + temperature)
        artifact.with_suffix(".json").write_bytes(completed.stdout)
        artifact.with_suffix(".stderr").write_bytes(completed.stderr)
        if completed.returncode:
            sys.exit(f"{name} {temperature} failed ({completed.returncode}): " + completed.stderr.decode(errors="replace"))
        payload = json.loads(completed.stdout)
        coverage = payload["coverage"]
        if completed.stdout.count(b"\n") != 1:
            sys.exit(f"{name} {temperature}: stdout is not compact single-line JSON")
        rss = re.search(rb"(\d+)\s+maximum resident set size", completed.stderr)
        if rss is None:
            sys.exit(f"{name} {temperature}: /usr/bin/time peak RSS unavailable")
        record = {
            "command": name,
            "cache": temperature,
            "stdout_bytes": len(completed.stdout),
            "request_count": coverage["request_count"],
            "cache_hits": coverage["cache_hits"],
            "wall_ms": wall_ms,
            "source_elapsed_ms": coverage["elapsed_ms"],
            "peak_rss_bytes": int(rss.group(1)),
            "returned_count": coverage["returned_count"],
            "scanned_pages": coverage["scanned_pages"],
            "detail_count": coverage["detail_count"],
            "argv": command,
        }
        if temperature == "cold" and record["request_count"] == 0:
            sys.exit(f"{name}: cold run made no source requests")
        if temperature == "warm" and (record["request_count"] != 0 or record["cache_hits"] == 0):
            sys.exit(f"{name}: warm run did not reuse fresh source HTML")
        records.append(record)
        print(json.dumps(record, ensure_ascii=False, separators=(",", ":")), flush=True)
(output / "measurements.json").write_text(json.dumps({"recorded_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "binary_sha256": binary_sha256, "runs": records}, ensure_ascii=False, indent=2) + "\n")
