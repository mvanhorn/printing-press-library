#!/usr/bin/env python3
"""Measure four read-only stay commands against cold and warm observations.

Usage:
  python3 scripts/measure.py --cli bin/jalan-pp-cli --output /tmp/jalan-measure.json

Uses the sibling live-e2e.py subprocess instrumentation. All eight invocations
share one fresh temporary --home, which is deleted afterward. The script makes
no source-witness requests, retries, or external deliveries. Expect five normal
cold upstream requests; allow ten attempts including the CLI's bounded retry.
"""
import argparse
import datetime as dt
import importlib.util
import json
import pathlib
import sys
import tempfile

NORMAL_UPSTREAM_REQUESTS = 5
MAX_UPSTREAM_ATTEMPTS = 10
MAX_INVOCATIONS = 8


def load_instrumentation():
    source = pathlib.Path(__file__).resolve().with_name("live-e2e.py")
    spec = importlib.util.spec_from_file_location("jalan_live_e2e_instrumentation", source)
    module = importlib.util.module_from_spec(spec)
    # Importing a local helper should not leave a __pycache__ artifact.
    old_setting = sys.dont_write_bytecode
    try:
        sys.dont_write_bytecode = True
        spec.loader.exec_module(module)
    finally:
        sys.dont_write_bytecode = old_setting
    module.MAX_COMMANDS = MAX_INVOCATIONS
    # Runner reserves four attempts before invoking. This benchmark checks its
    # tighter ten-attempt bound after each invocation and stops on any failure.
    module.MAX_CLI_REQUESTS = MAX_UPSTREAM_ATTEMPTS + 4
    return module


def observation_times(payload):
    return [(entry.get("url"), entry.get("observed_at"))
            for entry in payload["meta"].get("observations", [])]


def cases(check_in):
    next_date = (dt.date.fromisoformat(check_in) + dt.timedelta(days=1)).isoformat()
    party = ["--check-in", check_in, "--adults", "2", "--rooms", "1", "--nights", "1"]
    return [
        ("search-limit5", ["search", "--destination", "Hakone", *party, "--limit", "5"], 1),
        ("offers-limit5", ["offers", "385995", *party, "--limit", "5"], 1),
        ("exact-plan", ["plan", "385995", "--plan-id", "03912759", "--room-id", "0576806", *party], 1),
        ("compare-two-dates", ["compare", "385995", "--dates", check_in+","+next_date,
                               "--adults", "2", "--rooms", "1", "--nights", "1", "--limit", "5"], 2),
    ]


def extracted_facts(value):
    # Compare cells carry freshness observations alongside their facts. A warm
    # hit changes cache_status/cache_age_ms there, not the extracted results.
    if isinstance(value, dict):
        return {key: extracted_facts(item) for key, item in value.items() if key != "observations"}
    if isinstance(value, list):
        return [extracted_facts(item) for item in value]
    return value


def benchmark(module, runner, command, normal_requests):
    cold_arguments = [*command, "--max-age", "5m", "--refresh"]
    warm_arguments = [*command, "--max-age", "5m"]
    cold, cold_metrics = runner.invoke(cold_arguments)
    module.require(runner.cli_requests <= MAX_UPSTREAM_ATTEMPTS, "upstream attempt budget exceeded")
    module.require(cold_metrics["upstream_requests"] >= normal_requests,
                   "cold invocation did not observe each requested source page")
    module.require(cold["meta"].get("cache_status") == "live", "cold invocation reused cached observations")
    warm, warm_metrics = runner.invoke(warm_arguments)
    module.require(runner.cli_requests <= MAX_UPSTREAM_ATTEMPTS, "upstream attempt budget exceeded")
    same_timestamp = cold["meta"].get("observed_at") == warm["meta"].get("observed_at")
    same_observations = observation_times(cold) == observation_times(warm)
    runner.current["cold"] = cold_metrics
    runner.current["warm"] = warm_metrics
    runner.current["observed_at_equal"] = same_timestamp
    runner.current["source_observation_times_equal"] = same_observations
    runner.current["same_query"] = cold["meta"].get("query") == warm["meta"].get("query")
    runner.current["same_extracted_results"] = extracted_facts(cold["results"]) == extracted_facts(warm["results"])
    runner.current["result_comparison_omits"] = ["observations"]
    runner.current["request_reduction"] = cold_metrics["upstream_requests"] - warm_metrics["upstream_requests"]
    runner.current["latency_reduction_ms"] = round(cold_metrics["elapsed_ms"] - warm_metrics["elapsed_ms"], 2)
    module.require(warm_metrics["upstream_requests"] == 0, "warm invocation made an upstream request")
    module.require(warm["meta"].get("cache_status") == "hit", "warm invocation did not hit the exact cache")
    module.require(same_timestamp and same_observations, "warm cache changed original observation timestamps")
    module.require(runner.current["same_query"] and runner.current["same_extracted_results"],
                   "warm cache altered the query or extracted facts")
    module.require(cold_metrics["peak_rss_kib"] is not None and warm_metrics["peak_rss_kib"] is not None,
                   "per-process peak RSS measurement is unavailable")


def main():
    base = pathlib.Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli", type=pathlib.Path, default=base / "bin" / "jalan-pp-cli")
    parser.add_argument("--output", type=pathlib.Path, required=True, help="Local JSON proof path")
    parser.add_argument("--check-in", default="2026-11-10", help="Source fixture check-in; compare also checks the following day")
    args = parser.parse_args()
    args.cli = args.cli.resolve()
    if not args.cli.is_file():
        parser.error("build the CLI first, or provide its path with --cli")
    try:
        matrix = cases(args.check_in)
    except ValueError:
        parser.error("--check-in must be a valid YYYY-MM-DD date")
    module = load_instrumentation()
    started = dt.datetime.now(dt.timezone.utc).isoformat()
    with tempfile.TemporaryDirectory(prefix="jalan-measure-") as home:
        runner = module.Runner(args, home)
        stopped = False
        for name, command, normal_requests in matrix:
            if stopped:
                runner.rows.append({"name": name, "status": "skipped", "reason": "stopped after an earlier failed pair", "invocations": []})
                continue
            runner.case(name, lambda command=command, normal_requests=normal_requests:
                        benchmark(module, runner, command, normal_requests))
            last = runner.rows[-1]
            for metric in last["invocations"]:
                if metric["exit_code"] != 0 and "observed_at" not in metric:
                    # Total compare failures have no response metadata, and may
                    # exhaust both dates' two attempts. Keep a truthful upper
                    # bound rather than the generic runner's two-attempt guess.
                    upper = normal_requests * 2
                    runner.cli_requests += upper - metric["upstream_requests"]
                    metric["upstream_requests"] = None
                    metric["upstream_attempts_upper_bound"] = upper
            stopped = last["status"] != "pass" or runner.cli_requests > MAX_UPSTREAM_ATTEMPTS
        passed = sum(row["status"] == "pass" for row in runner.rows)
        failed = sum(row["status"] == "fail" for row in runner.rows)
        skipped = sum(row["status"] == "skipped" for row in runner.rows)
        proof = {
            "schema_version": 1, "kind": "stay_efficiency_benchmark",
            "started_at": started, "finished_at": dt.datetime.now(dt.timezone.utc).isoformat(),
            "verdict": "PASS" if passed == len(matrix) else "FAIL",
            "passed": passed, "failed": failed, "skipped": skipped,
            "check_in": args.check_in, "read_only": True, "private_temporary_home": True,
            "raw_pages_persisted": False, "delivery": "local_file",
            "bounds": {"normal_cold_upstream_requests": NORMAL_UPSTREAM_REQUESTS,
                       "worst_case_upstream_attempts": MAX_UPSTREAM_ATTEMPTS,
                       "invocations_max": MAX_INVOCATIONS, "script_retries": 0,
                       "cli_retry_per_request_max": 1, "subprocess_timeout_seconds": 50,
                       "independent_source_requests": 0},
            "observed": {"upstream_requests_or_failure_upper_bound": runner.cli_requests, "invocations": runner.calls},
            "request_metric_note": "Success and partial-response counts come from CLI meta.upstream_requests; total failures without JSON record two attempts per required source URL as an explicit upper bound. Any failure stops later pairs.",
            "rows": runner.rows,
        }
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(proof, ensure_ascii=False, indent=2, allow_nan=False)+"\n")
        print(json.dumps({"verdict": proof["verdict"], "passed": passed, "failed": failed,
                          "skipped": skipped, "proof": str(args.output)}, separators=(",", ":")))
        return 0 if proof["verdict"] == "PASS" else 1


if __name__ == "__main__":
    sys.exit(main())
