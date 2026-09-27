#!/usr/bin/env python3
"""Shared, bounded stdlib helpers for the opt-in TableCheck planning harnesses."""
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request

MAX_PAYLOAD = 4 * 1024 * 1024
TOKYO = dt.timezone(dt.timedelta(hours=9), "Asia/Tokyo")
VENUE = "sushi-tokyo81"
VENUE_ID = "67e657634474874e35785280"
KNOWN_COURSE_ID = "68da546fcde865308c33e7f9"


def date_range(value=None):
    day = dt.date.fromisoformat(value) if value else dt.datetime.now(TOKYO).date() + dt.timedelta(days=2)
    if day.isoformat() != (value or day.isoformat()):
        raise ValueError("--date must use YYYY-MM-DD")
    return day.isoformat(), (day + dt.timedelta(days=1)).isoformat()


def save_json(path, data):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return str(path)


def stop_process(process):
    try:
        if os.name == "posix":
            os.killpg(process.pid, signal.SIGKILL)
        else:
            process.kill()
    except ProcessLookupError:
        pass


def bounded_process(argv, timeout, input_bytes=None, max_stdout=MAX_PAYLOAD):
    """Drain both pipes with size caps; kill the whole process group on timeout."""
    started = time.monotonic()
    process = subprocess.Popen(argv, stdin=subprocess.PIPE if input_bytes is not None else subprocess.DEVNULL,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               start_new_session=(os.name == "posix"))
    buffers = {"stdout": bytearray(), "stderr": bytearray()}
    errors = []
    lock = threading.Lock()

    def drain(name, pipe, limit):
        try:
            while True:
                chunk = pipe.read(8192)
                if not chunk:
                    break
                if len(buffers[name]) + len(chunk) > limit:
                    with lock:
                        errors.append(name + " exceeds bounded payload")
                    stop_process(process)
                    break
                buffers[name].extend(chunk)
        finally:
            pipe.close()

    threads = [threading.Thread(target=drain, args=("stdout", process.stdout, max_stdout), daemon=True),
               threading.Thread(target=drain, args=("stderr", process.stderr, MAX_PAYLOAD), daemon=True)]
    for thread in threads:
        thread.start()
    if input_bytes is not None:
        try:
            process.stdin.write(input_bytes)
            process.stdin.close()
        except BrokenPipeError:
            pass
    try:
        process.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        errors.append("subprocess exceeded %.0fs wall deadline" % timeout)
        stop_process(process)
        process.wait(timeout=5)
    for thread in threads:
        thread.join(timeout=5)
    return {"exit_code": process.returncode, "wall_time_ms": round((time.monotonic() - started) * 1000, 3),
            "stdout": bytes(buffers["stdout"]), "stderr": bytes(buffers["stderr"]), "errors": errors}


def peak_rss(stderr):
    if sys.platform == "darwin":
        match = re.search(r"(\d+)\s+maximum resident set size", stderr)
        if match:
            value = int(match.group(1))
            return {"value": value, "unit": "bytes", "bytes": value, "tool": "/usr/bin/time -l"}
    else:
        match = re.search(r"Maximum resident set size \(kbytes\):\s*(\d+)", stderr)
        if match:
            value = int(match.group(1))
            return {"value": value, "unit": "KiB", "bytes": value * 1024, "tool": "/usr/bin/time -v"}
    return {"value": None, "unit": None, "bytes": None, "tool": "unavailable"}


class Budget:
    def __init__(self, limit=40):
        self.limit = limit
        self.charged = 0
        self.known_cli_requests = 0
        self.direct_requests = 0
        self.unknown_cli_invocations = 0

    def reserve(self, count):
        if self.charged + count > self.limit:
            raise RuntimeError("global request budget exhausted: %d + %d exceeds %d" % (self.charged, count, self.limit))
        self.charged += count

    def reconcile(self, allowance, actual):
        if isinstance(actual, int) and not isinstance(actual, bool) and 0 <= actual <= allowance:
            self.charged -= allowance - actual
            self.known_cli_requests += actual
        else:
            self.unknown_cli_invocations += 1

    def report(self):
        return {"limit": self.limit, "charged_attempt_upper_bound": self.charged,
                "observed_cli_attempts": self.known_cli_requests, "direct_attempts": self.direct_requests,
                "unknown_cli_invocations": self.unknown_cli_invocations,
                "actual_attempts": self.known_cli_requests + self.direct_requests if not self.unknown_cli_invocations else None}


class Runner:
    def __init__(self, binary, output, budget):
        self.binary = str(Path(binary).resolve())
        self.output = Path(output).resolve()
        self.output.mkdir(parents=True, exist_ok=True)
        self.budget = budget
        self.records = []
        self.halted = False
        if not Path(self.binary).is_file() or not os.access(self.binary, os.X_OK):
            raise ValueError("--binary must name an executable built TableCheck CLI")

    def cli(self, name, args, cache, allowance, expected_exit=0):
        if self.halted:
            raise RuntimeError("suite stopped after an invocation with unknown request count")
        allowance = min(allowance, 20, self.budget.limit - self.budget.charged)
        if allowance < 1:
            raise RuntimeError("global request budget exhausted before next CLI invocation")
        self.budget.reserve(allowance)
        command = [self.binary] + list(args) + ["--cache-dir", str(cache), "--max-requests", str(allowance),
                                               "--retries", "1", "--timeout", "10s", "--json"]
        timed = Path("/usr/bin/time").is_file() and sys.platform in ("darwin", "linux")
        argv = ["/usr/bin/time", "-l" if sys.platform == "darwin" else "-v"] + command if timed else command
        raw = bounded_process(argv, 35)
        stderr = raw["stderr"].decode("utf-8", errors="replace")
        record = {"name": name, "kind": "cli", "command": command, "exit_code": raw["exit_code"],
                  "wall_time_ms": raw["wall_time_ms"], "stdout_bytes": len(raw["stdout"]),
                  "peak_rss": peak_rss(stderr), "reserved_attempts": allowance, "errors": list(raw["errors"])}
        data = None
        try:
            data = json.loads(raw["stdout"])
            if not isinstance(data, dict):
                raise ValueError("planning output must be a JSON object")
        except (ValueError, UnicodeDecodeError) as error:
            record["errors"].append("stdout is not one planning JSON object: " + str(error))
        meta = data.get("meta", {}) if isinstance(data, dict) else {}
        requests = meta.get("requests") if isinstance(meta, dict) else None
        self.budget.reconcile(allowance, requests)
        record["requests"] = requests
        record["response_bytes"] = meta.get("response_bytes") if isinstance(meta, dict) else None
        record["source_latency_ms"] = meta.get("latency_ms") if isinstance(meta, dict) else None
        if requests is None or not isinstance(requests, int) or isinstance(requests, bool) or not 0 <= requests <= allowance:
            record["errors"].append("missing or invalid actual meta.requests; full allowance remains charged")
            self.halted = True
        if expected_exit == "nonzero":
            if raw["exit_code"] == 0:
                record["errors"].append("expected nonzero exit for preserved partial failure")
        elif raw["exit_code"] != expected_exit:
            record["errors"].append("unexpected exit %s; expected %s" % (raw["exit_code"], expected_exit))
        if data is not None:
            record["output_path"] = save_json(self.output / (name + ".json"), data)
        if stderr:
            # No cookies, tokens, environment dumps or HTTP headers are captured.
            # Public CLI diagnostics and resource-usage lines are bounded here.
            record["stderr_preview"] = stderr[:4000]
        self.records.append(record)
        return data, record

    def direct(self, name, path, body):
        if self.halted:
            raise RuntimeError("suite stopped after an invocation with unknown request count")
        self.budget.reserve(1)
        self.budget.direct_requests += 1
        request = json.dumps({"path": path, "body": body}, ensure_ascii=False).encode("utf-8")
        raw = bounded_process([sys.executable, str(Path(__file__).resolve()), "--run-live", "--http-worker"], 10,
                              input_bytes=request, max_stdout=MAX_PAYLOAD + 4096)
        record = {"name": name, "kind": "independent_http", "method": "POST", "path": path,
                  "request_body": body, "wall_time_ms": raw["wall_time_ms"], "requests": 1,
                  "errors": list(raw["errors"])}
        data = None
        try:
            result = json.loads(raw["stdout"])
            record["status"] = result.get("status")
            record["response_bytes"] = result.get("response_bytes")
            if raw["exit_code"] != 0 or result.get("error") or result.get("status") != 200:
                record["errors"].append(result.get("error") or "independent source returned HTTP %s" % result.get("status"))
            else:
                data = result.get("data")
        except (ValueError, UnicodeDecodeError) as error:
            record["errors"].append("independent source worker failed: " + str(error))
        self.records.append(record)
        return data, record


class Assertions:
    def __init__(self, name, records=None):
        self.name = name
        self.records = records or []
        self.checks = []
        self.errors = []

    def check(self, name, condition, observed=None, expected=None):
        self.checks.append({"name": name, "passed": bool(condition), "observed": observed, "expected": expected})

    def result(self):
        errors = self.errors + [error for record in self.records for error in record.get("errors", [])]
        result = {"name": self.name, "passed": not errors and all(check["passed"] for check in self.checks),
                  "assertions": self.checks, "errors": errors,
                  "invocations": [record["name"] for record in self.records]}
        print("%s: %s" % (self.name, "passed" if result["passed"] else "failed"), file=sys.stderr)
        return result


def freshness_tokens(data, path=""):
    tokens = {}
    if isinstance(data, dict):
        for key, value in data.items():
            child = path + "." + key
            if key in ("freshness", "venue_freshness") and isinstance(value, dict):
                tokens[child] = {"fetched_at": value.get("fetched_at"), "cache_hit": value.get("cache_hit"),
                                 "age_seconds": value.get("age_seconds")}
            else:
                tokens.update(freshness_tokens(value, child))
    elif isinstance(data, list):
        for index, value in enumerate(data):
            tokens.update(freshness_tokens(value, path + "[%d]" % index))
    return tokens


def cached_menu_item(cache_dir, body, course_id):
    """Read only the exact Go request-fingerprint file, never scan unrelated cache."""
    payload = json.dumps(body, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    prefix = b"POST\nhttps://production.tablecheck.com/v2/hub/menu_items\n"
    fingerprint = hashlib.sha256(prefix + payload).hexdigest()
    path = Path(cache_dir) / (fingerprint + ".json")
    if path.is_symlink():
        raise ValueError("request-keyed cache proof must not follow a symlink")
    with path.open("rb") as source:
        raw = source.read(MAX_PAYLOAD + 1025)
    if len(raw) > MAX_PAYLOAD + 1024:
        raise ValueError("request-keyed cache proof exceeds bounded entry size")
    entry = json.loads(raw)
    if entry.get("version") != 1:
        raise ValueError("request-keyed cache proof has unexpected cache version")
    items = entry.get("body", {}).get("menu_items")
    if not isinstance(items, list):
        raise ValueError("request-keyed cache proof is not a raw menu response")
    item = next((item for item in items if item.get("id") == course_id), None)
    if item is None:
        raise ValueError("selected course is absent from its request-keyed raw cache payload")
    return item, {"request_fingerprint": fingerprint, "cache_file": str(path),
                  "fetched_at": entry.get("fetched_at")}


def cutoff_comparison(cli_value, independent_value):
    """One field's independent observations may differ by one second at most."""
    if cli_value is None or independent_value is None:
        return {"cli_exact": cli_value, "independent_exact": independent_value,
                "delta_seconds": None, "within_one_second": cli_value is None and independent_value is None,
                "dynamic_subsecond_difference_observed": False,
                "comparison_rule": "both null, or offset-aware timestamps within <=1 second"}
    left = dt.datetime.fromisoformat(cli_value.replace("Z", "+00:00"))
    right = dt.datetime.fromisoformat(independent_value.replace("Z", "+00:00"))
    if left.tzinfo is None or right.tzinfo is None:
        raise ValueError("cutoff comparison requires offset-bearing source timestamps")
    delta = abs(left - right)
    within = delta <= dt.timedelta(seconds=1)
    return {"cli_exact": cli_value, "independent_exact": independent_value,
            "delta_seconds": delta.total_seconds(), "within_one_second": within,
            "dynamic_subsecond_difference_observed": cli_value != independent_value and dt.timedelta(0) < delta < dt.timedelta(seconds=1),
            "comparison_rule": "only min_time_cutoff_at: independent source observations within <=1 second; CLI/cache preservation remains exact"}


def calendar_projection(data, dates):
    calendar = data.get("availability_calendar", {}) if isinstance(data, dict) else {}
    projection = {"type": calendar.get("type"), "time_zone": calendar.get("time_zone"),
                  "closed_dates": [day for day in calendar.get("closed_dates", []) if day in dates], "data": {}}
    for day in dates:
        if day in calendar.get("data", {}):
            projection["data"][day] = {stamp: {"is_available": slot.get("is_available")}
                                       for stamp, slot in calendar["data"][day].items() if isinstance(slot, dict)}
    return projection


def source_slots(data, day):
    calendar = data.get("availability_calendar", {})
    if calendar.get("type") != "success" or calendar.get("time_zone") != "Asia/Tokyo":
        raise ValueError("independent calendar is not a success in verified Asia/Tokyo timezone")
    slots = []
    for stamp, value in calendar.get("data", {}).get(day, {}).items():
        parsed = dt.datetime.fromisoformat(stamp.replace("Z", "+00:00"))
        if parsed.tzinfo is None:
            raise ValueError("source timestamp lacks UTC offset")
        local = parsed.astimezone(TOKYO)
        if local.date().isoformat() != day:
            continue
        available = value.get("is_available") if isinstance(value, dict) else None
        if isinstance(available, bool):
            slots.append({"local_time": local.strftime("%H:%M"), "starts_at": parsed.astimezone(dt.timezone.utc),
                          "is_available": available})
    return slots


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, file_pointer, code, message, headers, new_url):
        return None


def http_worker():
    """Private child mode: an outer process kills this entire read at ten seconds."""
    result = {}
    try:
        request = json.loads(sys.stdin.buffer.read(65537))
        path = request["path"]
        if path not in ("/v2/hub/menu_items", "/v2/hub/availability_calendar_v2"):
            raise ValueError("endpoint is outside the fixed anonymous read allowlist")
        payload = json.dumps(request["body"]).encode("utf-8")
        req = urllib.request.Request("https://production.tablecheck.com" + path, data=payload, method="POST",
                                     headers={"Accept": "application/json", "Content-Type": "application/json",
                                              "User-Agent": "tablecheck-planning-contract-harness/0.1"})
        opener = urllib.request.build_opener(NoRedirect(), urllib.request.ProxyHandler({}))
        with opener.open(req, timeout=10) as response:
            raw = response.read(MAX_PAYLOAD + 1)
            result = {"status": response.status, "response_bytes": len(raw)}
            if len(raw) > MAX_PAYLOAD:
                raise ValueError("independent source exceeds 4 MiB payload bound")
            result["data"] = json.loads(raw)
    except urllib.error.HTTPError as error:
        result = {"status": error.code, "response_bytes": 0, "error": "independent HTTP read failed (%d)" % error.code}
    except Exception as error:
        # Never dump response headers, cookies, proxy credentials or environment.
        result["error"] = "%s during independent bounded HTTP read" % type(error).__name__
    print(json.dumps(result, ensure_ascii=False))
    return 1 if result.get("error") else 0


if __name__ == "__main__":
    if sys.argv[1:] != ["--run-live", "--http-worker"]:
        raise SystemExit("This helper is used by the opt-in planning harnesses.")
    raise SystemExit(http_worker())
