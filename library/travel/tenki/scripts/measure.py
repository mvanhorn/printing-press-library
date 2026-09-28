#!/usr/bin/env python3
"""Measure a finite, serial tenki-pp-cli workflow without changing user caches.

Example (from the workspace):
  python3 scripts/measure.py --binary ./tenki-pp-cli \
    --output-dir .press/proofs/measure-20260928 --cache-dir .press/measure-cache

The output directory must be new or empty. CacheDir is a parent directory: each
run creates its own new child, keeps it for inspection, and never deletes cache.
Run --self-test for local helper checks without executing the CLI or any network.
"""

import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
import uuid


JST = dt.timezone(dt.timedelta(hours=9), "Asia/Tokyo")
TOKYO = "https://tenki.jp/forecast/3/16/4410/13101/"
KYOTO = "https://tenki.jp/forecast/6/29/6110/26103/"
SAPPORO = "https://tenki.jp/forecast/1/2/1400/1100/"
MURODO = "https://tenki.jp/kouyou/4/19/30314.html"
TIME_BIN = Path("/usr/bin/time")
RSS_LINE = re.compile(r"^\s*(\d+)\s+maximum resident set size\s*$", re.MULTILINE)
TIMER_START = re.compile(r"^\s*[\d.]+\s+real\s+[\d.]+\s+user\s+[\d.]+\s+sys\s*$")
METRIC_NAMES = ("http_requests", "cache_hits", "response_bytes")


def workspace_root():
    """Printing Press staging lives under the original workspace's .press."""
    script = Path(__file__).resolve()
    for parent in script.parents:
        if parent.name == ".press":
            return parent.parent
    return script.parents[1]


def checked_path(value, workspace, label):
    path = Path(value).expanduser().resolve()
    if not path.is_relative_to(workspace):
        raise ValueError(f"{label} must stay inside workspace {workspace}")
    return path


def split_time_stderr(raw):
    """Keep CLI diagnostics distinct from macOS time's final resource block."""
    lines = raw.splitlines(keepends=True)
    start = next((i for i, line in enumerate(lines) if TIMER_START.match(line.rstrip())), None)
    if start is None:
        return raw, "", None
    timing = "".join(lines[start:])
    match = RSS_LINE.search(timing)
    return "".join(lines[:start]), timing, int(match.group(1)) if match else None


def probe_timer(output_dir):
    if sys.platform != "darwin" or not TIME_BIN.is_file():
        return False, "macOS /usr/bin/time -l is unavailable on this platform"
    try:
        probe = subprocess.run([str(TIME_BIN), "-l", "/usr/bin/true"],
                               capture_output=True, timeout=10, env={**os.environ, "LC_ALL": "C"})
    except (OSError, subprocess.TimeoutExpired) as exc:
        return False, f"resource probe failed: {exc}"
    raw = probe.stderr.decode("utf-8", errors="replace")
    (output_dir / "time-probe.stderr.txt").write_text(raw, encoding="utf-8")
    _, _, rss = split_time_stderr(raw)
    if probe.returncode != 0 or rss is None:
        reason = " ".join(raw.split())[:240] or "maximum resident set size was absent"
        return False, f"resource probe unavailable (exit {probe.returncode}): {reason}"
    return True, ""


def cases(next_date, year):
    daily = ["forecast", "daily", "--place", TOKYO, "--days", "3"]
    compare = ["compare", "--place", TOKYO, "--place", SAPPORO, "--from", next_date,
               "--days", "2", "--max-pop", "40", "--max-temp", "30"]
    return [
        ("chiyoda_daily_cold", daily),
        ("chiyoda_daily_warm", daily),
        ("chiyoda_daily_refresh", daily + ["--refresh"]),
        ("chiyoda_daily_projected_warm", daily + ["--select",
          "meta.metrics,results.periods.date,results.periods.weather,results.source.url"]),
        ("kyoto_hourly_window", ["forecast", "hourly", "--place", KYOTO, "--date", next_date,
          "--hours", "09:00-17:00", "--limit", "8"]),
        ("murodo_foliage", ["seasonal", "show", "--kind", "kouyou", "--place", MURODO,
          "--year", str(year)]),
        ("tokyo_sapporo_compare", compare),
        ("tokyo_sapporo_compare_warm", compare),
    ]


def read_metrics(value):
    if not isinstance(value, dict) or "results" not in value:
        raise ValueError("JSON output lacks the results envelope")
    meta = value.get("meta")
    metrics = meta.get("metrics") if isinstance(meta, dict) else None
    if not isinstance(metrics, dict):
        raise ValueError("JSON output lacks meta.metrics")
    result = {}
    for name in METRIC_NAMES:
        number = metrics.get(name)
        if type(number) is not int or number < 0:
            raise ValueError(f"meta.metrics.{name} is not a nonnegative integer")
        result[name if name != "response_bytes" else "decoded_response_bytes"] = number
    return result


def run_case(binary, name, args, output_dir, cache_dir, home_dir, use_timer, timer_reason):
    stdout_path = output_dir / f"{name}.stdout.json"
    stderr_path = output_dir / f"{name}.stderr.raw.txt"
    cli_args = [str(binary), *args, "--agent", "--data-source", "auto",
                "--cache-dir", str(cache_dir), "--home", str(home_dir)]
    command = [str(TIME_BIN), "-l", *cli_args] if use_timer else cli_args
    record = {"name": name, "command": cli_args, "exit_code": None,
              "stdout_file": stdout_path.name, "stderr_raw_file": stderr_path.name,
              "stderr_file": f"{name}.stderr.txt", "time_file": f"{name}.time.txt",
              "peak_rss_bytes": None, "rss_status": "not_measured", "rss_reason": timer_reason,
              "valid_json": False, "failures": []}
    began = time.monotonic()
    try:
        with stdout_path.open("wb") as stdout, stderr_path.open("wb") as stderr:
            proc = subprocess.Popen(command, stdout=stdout, stderr=stderr, start_new_session=True,
                                    env={**os.environ, "LC_ALL": "C"}, cwd=workspace_root())
            try:
                proc.wait(timeout=120)
            except subprocess.TimeoutExpired:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
                raise
        record["exit_code"] = proc.returncode
    except (OSError, subprocess.TimeoutExpired) as exc:
        record["failures"].append(f"process execution failed: {exc}")
    record["wall_seconds"] = round(time.monotonic() - began, 6)
    record["stdout_bytes"] = stdout_path.stat().st_size if stdout_path.exists() else 0
    raw_stderr = stderr_path.read_text(encoding="utf-8", errors="replace") if stderr_path.exists() else ""
    cli_stderr, time_stderr, rss = split_time_stderr(raw_stderr) if use_timer else (raw_stderr, "", None)
    (output_dir / record["stderr_file"]).write_text(cli_stderr, encoding="utf-8")
    (output_dir / record["time_file"]).write_text(time_stderr, encoding="utf-8")
    if rss is not None:
        record.update(peak_rss_bytes=rss, rss_status="measured", rss_reason="")
    elif use_timer:
        record["rss_reason"] = "macOS time output did not contain maximum resident set size"
    if record["exit_code"] != 0:
        record["failures"].append(f"command exited {record['exit_code']}; inspect {record['stderr_file']}")
    try:
        value = json.loads(stdout_path.read_bytes())
        record["valid_json"] = True
        record.update(read_metrics(value))
    except (OSError, ValueError) as exc:
        record["failures"].append(f"JSON/metrics validation failed: {exc}")
    return record


def assertions(records):
    indexed = {row["name"]: row for row in records}
    failures = [f"{row['name']}: {message}" for row in records for message in row["failures"]]
    for name in ("chiyoda_daily_warm", "chiyoda_daily_projected_warm", "tokyo_sapporo_compare_warm"):
        row = indexed[name]
        if row.get("http_requests") != 0:
            failures.append(f"{name}: warm HTTP requests must equal 0 (measured {row.get('http_requests')})")
        if row.get("cache_hits", 0) < 1:
            failures.append(f"{name}: warm result must report at least one cache hit")
    for name in ("chiyoda_daily_cold", "chiyoda_daily_refresh"):
        row = indexed[name]
        if row.get("http_requests", 0) < 1:
            failures.append(f"{name}: cold/refresh must perform at least one measured HTTP request")
    full = indexed["chiyoda_daily_warm"]["stdout_bytes"]
    projected = indexed["chiyoda_daily_projected_warm"]["stdout_bytes"]
    if not 0 < projected < full:
        failures.append(f"field projection must be smaller than full output ({projected} vs {full} bytes)")
    return failures


def markdown(summary):
    lines = [f"Measurement status: **{summary['status']}**.", "",
             f"JST date anchor: {summary['started_at']}; next-day window: {summary['next_date']}.",
             "Serial finite commands. Inter-case politeness pauses are outside the measured wall time.",
             "Response bytes are decoded source bodies consumed by the reader, not transfer bytes.", "",
             "| Case | Exit | Wall (s) | Stdout bytes | HTTP | Cache hits | Decoded response bytes | Peak RSS bytes |",
             "|---|---:|---:|---:|---:|---:|---:|---:|"]
    for row in summary["cases"]:
        rss = row["peak_rss_bytes"] if row["rss_status"] == "measured" else "not measured"
        lines.append(f"| {row['name']} | {row['exit_code']} | {row['wall_seconds']:.3f} | {row['stdout_bytes']} | "
                     f"{row.get('http_requests', 'unknown')} | {row.get('cache_hits', 'unknown')} | "
                     f"{row.get('decoded_response_bytes', 'unknown')} | {rss} |")
    if summary["failures"]:
        lines += ["", "Failed assertions:", ""] + [f"- {failure}" for failure in summary["failures"]]
    reasons = sorted({row["rss_reason"] for row in summary["cases"] if row["rss_status"] != "measured"})
    if reasons:
        lines += ["", "RSS limitations:", ""] + [f"- {reason}" for reason in reasons]
    return "\n".join(lines) + "\n"


def self_test():
    raw = "CLI warning\n        0.01 real         0.02 user         0.00 sys\n  123456 maximum resident set size\n"
    assert split_time_stderr(raw) == ("CLI warning\n", raw[len("CLI warning\n"):], 123456)
    assert split_time_stderr("time: sysctl kern.clockrate: Operation not permitted\n")[2] is None
    assert split_time_stderr("        0.00 real         0.00 user         0.00 sys\n")[2] is None
    assert read_metrics({"meta": {"metrics": {"http_requests": 1, "cache_hits": 0, "response_bytes": 42}}, "results": {}}) == {
        "http_requests": 1, "cache_hits": 0, "decoded_response_bytes": 42}
    for bad in ({}, {"meta": {"metrics": {}}, "results": {}},
                {"meta": {"metrics": {"http_requests": True, "cache_hits": 0, "response_bytes": 0}}, "results": {}}):
        try:
            read_metrics(bad)
        except ValueError:
            pass
        else:
            raise AssertionError("invalid metric/envelope accepted")
    selections = cases("2027-01-01", 2026)
    assert len(selections) == 8 and "meta.metrics" in selections[3][1][-1]
    assert selections[4][1][selections[4][1].index("--date") + 1] == "2027-01-01"
    fake = []
    for name, _ in selections:
        warm = "warm" in name
        fake.append({"name": name, "failures": [], "http_requests": 0 if warm else 1,
                     "cache_hits": 1 if warm else 0,
                     "stdout_bytes": 50 if "projected" in name else 100})
    assert assertions(fake) == []
    fake[1]["http_requests"] = 1
    assert any("warm HTTP requests" in failure for failure in assertions(fake))
    try:
        checked_path("/", Path("/tmp/example-workspace"), "output directory")
    except ValueError:
        pass
    else:
        raise AssertionError("outside-workspace output accepted")
    print("PASS: local RSS, diagnostics, JSON metrics, finite case, warm/refresh/projection and path helpers")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--binary", help="Existing built tenki-pp-cli executable")
    parser.add_argument("--output-dir", help="New or empty workspace directory for captures and reports")
    parser.add_argument("--cache-dir", help="Workspace parent directory for a new isolated run cache")
    parser.add_argument("--self-test", action="store_true", help="Run only local helper assertions; no CLI/network")
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return 0
    if not args.binary or not args.output_dir or not args.cache_dir:
        parser.error("--binary, --output-dir and --cache-dir are required")
    try:
        workspace = workspace_root()
        binary = Path(args.binary).expanduser().resolve()
        if not binary.is_file() or not os.access(binary, os.X_OK):
            raise ValueError("--binary must identify an existing executable")
        output = checked_path(args.output_dir, workspace, "output directory")
        cache_parent = checked_path(args.cache_dir, workspace, "cache directory")
        if output.exists() and (not output.is_dir() or any(output.iterdir())):
            raise ValueError("output directory already contains files; choose a new directory to preserve earlier proofs")
        output.mkdir(parents=True, exist_ok=True)
        cache_parent.mkdir(parents=True, exist_ok=True)
        run_id = dt.datetime.now(JST).strftime("%Y%m%d-%H%M%S") + "-" + uuid.uuid4().hex[:8]
        with (output / ".measure-run.json").open("x", encoding="utf-8") as claim:
            json.dump({"run_id": run_id, "owner": "scripts/measure.py"}, claim)
            claim.write("\n")
        cache = cache_parent / run_id
        cache.mkdir()
        home = output / "cli-home"
        home.mkdir()
    except (OSError, ValueError) as exc:
        parser.error(str(exc))
    started = dt.datetime.now(JST)
    next_date = (started.date() + dt.timedelta(days=1)).isoformat()
    use_timer, timer_reason = probe_timer(output)
    measured = []
    selected_cases = cases(next_date, started.year)
    for i, (name, command_args) in enumerate(selected_cases):
        row = run_case(binary, name, command_args, output, cache, home, use_timer, timer_reason)
        measured.append(row)
        if row.get("http_requests") != 0 and i + 1 < len(selected_cases):
            time.sleep(1)  # Politeness pause is deliberately outside run_case's timed interval.
    failures = assertions(measured)
    summary = {"schema_version": 1, "status": "PASS" if not failures else "FAIL",
               "started_at": started.isoformat(), "timezone": "Asia/Tokyo", "next_date": next_date,
               "binary": str(binary), "workspace": str(workspace), "output_dir": str(output),
               "cache_dir": str(cache), "cli_home": str(home), "cases": measured, "failures": failures,
               "response_bytes_meaning": "decoded source response bodies, not network transfer bytes",
               "rss_meaning": "macOS /usr/bin/time -l maximum resident set size in bytes"}
    (output / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (output / "summary.md").write_text(markdown(summary), encoding="utf-8")
    print(f"{summary['status']}: {len(measured)} serial cases; report {output / 'summary.md'}")
    if failures:
        for failure in failures:
            print(f"FAIL: {failure}", file=sys.stderr)
    return 0 if not failures else 1


if __name__ == "__main__":
    raise SystemExit(main())
