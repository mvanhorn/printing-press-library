#!/usr/bin/env python3
"""Read-only stdio MCP catalog/context probe for the dedicated reviewer."""
import argparse
import datetime
import json
import pathlib
import select
import subprocess
import time


parser = argparse.ArgumentParser()
parser.add_argument("--binary", default="build/japan-bus-online-pp-mcp")
parser.add_argument("--output", default="evidence/mcp-runtime-tools.json")
parser.add_argument("--probe-output", default="evidence/mcp-runtime-probe.json")
args = parser.parse_args()
process = subprocess.Popen(
    [args.binary], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
    stderr=subprocess.PIPE, text=True, bufsize=1,
)


def request(identifier, method, params=None):
    payload = {"jsonrpc": "2.0", "id": identifier, "method": method}
    if params is not None:
        payload["params"] = params
    process.stdin.write(json.dumps(payload) + "\n")
    process.stdin.flush()
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        ready, _, _ = select.select(
            [process.stdout], [], [], max(0, deadline - time.monotonic())
        )
        if not ready:
            raise RuntimeError("MCP request timed out")
        line = process.stdout.readline()
        if not line:
            raise RuntimeError("MCP process ended before its response")
        result = json.loads(line)
        if result.get("id") == identifier:
            return result
    raise RuntimeError("MCP request timed out")


try:
    initialized = request(1, "initialize", {
        "protocolVersion": "2024-11-05", "capabilities": {},
        "clientInfo": {"name": "provider-artifact-refresh", "version": "1"},
    })
    catalog_response = request(2, "tools/list")
    catalog = catalog_response["result"]
    context = request(3, "tools/call", {"name": "context", "arguments": {}})
    dry_run = request(4, "tools/call", {
        "name": "bus_quote", "arguments": {
            "route": "12200160001", "dry-run": True, "no-learn": True,
        },
    })
    unknown = request(5, "tools/call", {
        "name": "bus_quote", "arguments": {"imaginary": True},
    })
    pathlib.Path(args.output).write_text(json.dumps(catalog, indent=2) + "\n")
    proof = {
        "binary": args.binary,
        "observed_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "initialize": initialized, "context": context,
        "dry_run_quote": dry_run, "unknown_parameter": unknown,
        "tool_count": len(catalog["tools"]),
    }
    pathlib.Path(args.probe_output).write_text(json.dumps(proof, indent=2) + "\n")
    print(json.dumps({
        "tool_count": len(catalog["tools"]), "catalog": args.output,
        "probe": args.probe_output,
    }))
finally:
    process.terminate()
    try:
        process.wait(timeout=3)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=3)
