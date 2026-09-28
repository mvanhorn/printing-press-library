#!/usr/bin/env python3
"""Measure the actual executable using replay HTTP and an isolated tokenizer tool."""
import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.metadata
import json
import math
import os
from pathlib import Path
import platform
import re
import subprocess
import tempfile
import threading
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--binary", required=True, type=Path)
parser.add_argument("--fixtures", type=Path, default=Path(__file__).parent / "testdata")
parser.add_argument("--output", required=True, type=Path)
parser.add_argument("--samples", type=int, default=20)
parser.add_argument("--tokenizer-cache", type=Path)
args = parser.parse_args()
if args.samples < 1 or args.samples > 100:
    parser.error("samples must be between 1 and 100")
if platform.system() != "Darwin":
    parser.error("CPU/RSS collection currently requires macOS /usr/bin/time -l")
if args.tokenizer_cache:
    os.environ["TIKTOKEN_CACHE_DIR"] = str(args.tokenizer_cache.resolve())
import tiktoken
encoding = tiktoken.get_encoding("o200k_base")

bodies = {"/en/tokyo/rstLst/": (args.fixtures / "tokyo-ranked.html").read_bytes(),
          "/en/tokyo/A1301/A130103/13294162/": (args.fixtures / "sushi-detail.html").read_bytes(),
          "/en/": (args.fixtures / "english-home.html").read_bytes()}
requests = []
lock = threading.Lock()

class Replay(BaseHTTPRequestHandler):
    def do_GET(self):
        path = self.path.split("?", 1)[0]
        body = bodies.get(path, b"unexpected measurement request")
        with lock:
            requests.append({"path": path, "bytes": len(body)})
        self.send_response(200 if path in bodies else 404)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *_):
        pass

server = ThreadingHTTPServer(("127.0.0.1", 0), Replay)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
binary = args.binary.resolve()

def environment(home):
    home.mkdir(parents=True, exist_ok=True)
    env = {k: v for k, v in os.environ.items() if not k.startswith(("TABELOG_", "XDG_")) and k not in {"HOME", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"}}
    env.update(HOME=str(home), XDG_CONFIG_HOME=str(home / "config"), XDG_DATA_HOME=str(home / "data"), XDG_CACHE_HOME=str(home / "cache"), TABELOG_TEST_MODE="1", TABELOG_TEST_BASE_URL=f"http://127.0.0.1:{server.server_port}", HTTP_PROXY="", HTTPS_PROXY="", ALL_PROXY="", NO_PROXY="127.0.0.1,localhost", LC_ALL="C")
    return env

def invoke(command, env, measured=True):
    with lock:
        before = len(requests)
    argv = (["/usr/bin/time", "-l"] if measured else []) + [str(binary)] + command
    start = time.perf_counter()
    proc = subprocess.run(argv, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=25)
    elapsed_ms = (time.perf_counter() - start) * 1000
    if proc.returncode:
        raise RuntimeError(f"{command[0]} exited {proc.returncode}: {proc.stderr.decode(errors='replace')[:700]}")
    payload = json.loads(proc.stdout)
    if not isinstance(payload, dict) or "items" not in payload or "meta" not in payload:
        raise RuntimeError("measurement received an unexpected output envelope")
    with lock:
        seen = requests[before:]
    sample = {"wall_ms": round(elapsed_ms, 3), "stdout_bytes": len(proc.stdout), "tokens": len(encoding.encode(proc.stdout.decode())), "http_requests": len(seen), "http_response_bytes": sum(x["bytes"] for x in seen)}
    if measured:
        stderr = proc.stderr.decode(errors="replace")
        cpu = re.search(r"([\d.]+)\s+real\s+([\d.]+)\s+user\s+([\d.]+)\s+sys", stderr)
        rss = re.search(r"(\d+)\s+maximum resident set size", stderr)
        if not cpu or not rss:
            raise RuntimeError("could not parse /usr/bin/time CPU/RSS output")
        sample.update(cpu_ms=round((float(cpu[2]) + float(cpu[3])) * 1000, 3), peak_rss_bytes=int(rss[1]))
    return sample, proc.stdout

def p95(values):
    return sorted(values)[math.ceil(0.95 * len(values)) - 1]

report = {"schema_version": 1, "status": "running", "host": {"platform": platform.platform(), "machine": platform.machine()}, "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(), "tokenizer": {"package": "tiktoken", "version": importlib.metadata.version("tiktoken"), "encoding": encoding.name}, "method": "20 subprocesses per profile by default; nearest-rank p95; macOS /usr/bin/time -l measures each CLI process; loopback HTTP serves sanitized public-source fixtures. Tokenizer/replay-server CPU and memory are outside the measured CLI process.", "profiles": []}
commands = {"find": ["find", "--area", "https://tabelog.com/en/tokyo/rstLst/", "--agent"], "show": ["show", "https://tabelog.com/en/tokyo/A1301/A130103/13294162/", "--agent"]}
args.output.parent.mkdir(parents=True, exist_ok=True)
try:
    for command_name, command in commands.items():
        for mode in ["cold", "warm"]:
            samples = []
            with tempfile.TemporaryDirectory(prefix="tabelog-measure-") as tmp:
                base = Path(tmp)
                warm_env = environment(base / "warm")
                seed_requests = 0
                if mode == "warm":
                    seed, _ = invoke(command, warm_env, measured=False)
                    seed_requests = seed["http_requests"]
                for index in range(args.samples):
                    env = environment(base / f"cold-{index}") if mode == "cold" else warm_env
                    sample, stdout = invoke(command, env)
                    samples.append(sample)
                    if index == 0:
                        (args.output.parent / f"measurement-{command_name}-{mode}.json").write_bytes(stdout)
            summary = {key: p95([x[key] for x in samples]) for key in ["wall_ms", "cpu_ms", "peak_rss_bytes", "stdout_bytes", "tokens"]}
            summary.update(total_http_requests=sum(x["http_requests"] for x in samples), total_http_response_bytes=sum(x["http_response_bytes"] for x in samples))
            output_limit = 1300 if command_name == "find" else 1600
            byte_limit = 5 * 1024 if command_name == "find" else 8 * 1024
            checks = {"tokens": summary["tokens"] <= output_limit, "stdout_bytes": summary["stdout_bytes"] <= byte_limit, "cpu": summary["cpu_ms"] <= 250, "rss": summary["peak_rss_bytes"] <= 64 * 1024 * 1024, "bounded_requests": all(x["http_requests"] == (1 if mode == "cold" else 0) for x in samples)}
            if mode == "warm":
                checks["cached_latency"] = summary["wall_ms"] <= 400
            report["profiles"].append({"command": command_name, "mode": mode, "samples": samples, "seed_requests": seed_requests, "p95": summary, "checks": checks})
    report["status"] = "pass" if all(all(p["checks"].values()) for p in report["profiles"]) else "fail"
except Exception as exc:
    report["status"] = "fail"
    report["failure"] = str(exc)
finally:
    server.shutdown()
    server.server_close()
    args.output.write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps({"status": report["status"], "profiles": [{"command": p["command"], "mode": p["mode"], "p95": p["p95"], "checks": p["checks"]} for p in report["profiles"]]}, separators=(",", ":")))
raise SystemExit(0 if report["status"] == "pass" else 1)
