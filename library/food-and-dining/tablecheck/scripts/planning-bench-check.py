#!/usr/bin/env python3
"""Opt-in cold/warm TableCheck CLI measurements with a fixed 40-attempt ceiling."""
import argparse
import datetime as dt
import importlib.util
import json
from pathlib import Path
import sys
import tempfile

spec = importlib.util.spec_from_file_location("planning_live_common", Path(__file__).with_name("planning-live-common.py"))
common = importlib.util.module_from_spec(spec)
spec.loader.exec_module(common)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-live", action="store_true", help="Explicitly authorize the bounded cold/refresh HTTP reads")
    parser.add_argument("--binary", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--date", help="Tokyo YYYY-MM-DD; default Tokyo today plus two days")
    args = parser.parse_args()
    if not args.run_live:
        print(json.dumps({"passed": False, "errors": ["--run-live is required; no network requests made"]}))
        return 2
    budget = common.Budget(40)
    runner = None
    cases, measurements, errors = [], [], []
    started = dt.datetime.now(dt.timezone.utc).isoformat()
    try:
        day, tomorrow = common.date_range(args.date)
        runner = common.Runner(args.binary, args.output_dir, budget)
        cache_parent = runner.output / "cache"
        cache_parent.mkdir(exist_ok=True)
        definitions = [
            ("search", ["venues", "search", "--lat", "35.681236", "--lon", "139.767125", "--radius", "3000",
                        "--cuisine", "sushi", "--budget-max", "20000", "--date", day, "--party", "2", "--limit", "2"], 2),
            ("venue", ["venues", "get", common.VENUE], 2),
            ("courses", ["courses", "list", common.VENUE, "--from", day, "--to", tomorrow, "--limit", "5"], 4),
            ("check", ["availability", "check", common.VENUE, "--date", day, "--time", "18:00", "--party", "2", "--limit", "10"], 4),
            ("scan", ["availability", "scan", common.VENUE, "kakidasushi-marunouchi", "--from", day, "--to", tomorrow,
                      "--time", "18:00", "--party", "2", "--limit", "5"], 8),
        ]
        for name, command, allowance in definitions:
            cache = Path(tempfile.mkdtemp(prefix=name + "-", dir=cache_parent))
            case = common.Assertions("cold_warm_" + name)
            try:
                cold, cold_record = runner.cli(name + "-cold", command, cache, allowance)
                case.records.append(cold_record)
                # Each warm read runs in a separate process. Limit one accidental
                # HTTP attempt so a broken cache cannot consume a full fanout.
                warm, warm_record = runner.cli(name + "-warm", command, cache, 1)
                case.records.append(warm_record)
                case.check("cold_contacts_source", isinstance(cold_record["requests"], int) and cold_record["requests"] > 0,
                           cold_record["requests"], ">0")
                case.check("warm_has_zero_HTTP_attempts", warm_record["requests"] == 0, warm_record["requests"], 0)
                case.check("warm_has_zero_HTTP_response_bytes", warm_record["response_bytes"] == 0, warm_record["response_bytes"], 0)
                before = common.freshness_tokens(cold or {})
                after = common.freshness_tokens(warm or {})
                case.check("visible_freshness", bool(before) and set(before) == set(after), sorted(after))
                case.check("original_observation_preserved", bool(before) and all(before[key]["fetched_at"] == after.get(key, {}).get("fetched_at") for key in before),
                           {key: value["fetched_at"] for key, value in after.items()})
                case.check("warm_cache_hits_identified", bool(after) and all(value["cache_hit"] is True for value in after.values()))
                for record in (cold_record, warm_record):
                    case.check(record["name"] + "_peak_RSS_measured", record["peak_rss"]["bytes"] is not None,
                               record["peak_rss"], "Darwin bytes or Linux KiB converted to bytes")
                    measurements.append({key: record.get(key) for key in ("name", "requests", "response_bytes", "stdout_bytes",
                                                                          "wall_time_ms", "source_latency_ms", "peak_rss", "output_path")})
                if name == "check":
                    refreshed, refresh_record = runner.cli("check-refresh", command + ["--refresh"], cache, allowance)
                    case.records.append(refresh_record)
                    case.check("refresh_rereads_source", isinstance(refresh_record["requests"], int) and refresh_record["requests"] > 0,
                               refresh_record["requests"], ">0")
                    fresh = common.freshness_tokens(refreshed or {})
                    case.check("refresh_not_a_cache_hit", bool(fresh) and all(value["cache_hit"] is False for value in fresh.values()))
                    case.check("refresh_observation_is_new", bool(before) and any(before[key]["fetched_at"] != fresh.get(key, {}).get("fetched_at") for key in before))
                    measurements.append({key: refresh_record.get(key) for key in ("name", "requests", "response_bytes", "stdout_bytes",
                                                                                  "wall_time_ms", "source_latency_ms", "peak_rss", "output_path")})
            except Exception as error:
                case.errors.append("%s: %s" % (type(error).__name__, error))
            cases.append(case.result())
            if runner.halted:
                raise RuntimeError("stopped benchmark after unknown request count; reserved ceiling retained")
    except Exception as error:
        errors.append("%s: %s" % (type(error).__name__, error))
    report = {"harness": "tablecheck-planning-benchmark", "started_at": started,
              "finished_at": dt.datetime.now(dt.timezone.utc).isoformat(),
              "passed": not errors and len(cases) == 5 and all(case["passed"] for case in cases),
              "cases": cases, "measurements": measurements, "errors": errors, "request_budget": budget.report(),
              "invocations": runner.records if runner else [],
              "units": {"stdout_bytes": "bytes emitted by CLI, including newline", "response_bytes": "actual HTTP payload bytes read by CLI",
                        "wall_time_ms": "milliseconds including process startup", "source_latency_ms": "CLI-reported milliseconds",
                        "peak_rss": "Darwin bytes; Linux KiB, also normalized to bytes"},
              "limits": {"cli_wall_seconds": 35, "payload_bytes": common.MAX_PAYLOAD, "cli_retries_max": 1,
                         "warm_accidental_HTTP_attempts_max": 1, "cache_dirs": "one fresh dedicated directory per representative case"}}
    if runner:
        common.save_json(runner.output / "bench-report.json", report)
    print(json.dumps(report, ensure_ascii=False))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
