#!/usr/bin/env bash
# mcp-shipcheck.sh — exercise the MCP server over its streamable-HTTP
# transport and record protocol coverage to mcp-shipcheck.json.
#
# Tenant-agnostic. Expects GORGIAS_USERNAME, GORGIAS_API_KEY, and
# GORGIAS_BASE_URL in the environment for the gorgias_execute probe.
# Without those, gorgias_execute is recorded as `skip` and the rest
# still runs (initialize, tools/list, gorgias_search are auth-free).
#
# Probes:
#   1. initialize           — protocol handshake + session capture
#   2. tools/list           — must enumerate the code-orchestration tools
#   3. gorgias_search       — endpoint discovery (no API call)
#   4. gorgias_execute      — live API call via tickets.list (creds-gated)
#
# Output:
#   stdout — human-readable summary per probe + final verdict
#   mcp-shipcheck.json — machine-readable artifact at repo root

set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO}"

BINARY="${REPO}/gorgias-pp-mcp"
if [ ! -x "${BINARY}" ]; then
  echo "==> building gorgias-pp-mcp"
  go build -o gorgias-pp-mcp ./cmd/gorgias-pp-mcp
fi

PORT=$(/usr/bin/python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()')
TMP=$(mktemp -d)
cleanup() {
  if [ -n "${MCP_PID:-}" ]; then
    kill "$MCP_PID" 2>/dev/null || true
    wait "$MCP_PID" 2>/dev/null || true
  fi
  rm -rf "${TMP}"
}
trap cleanup EXIT

# Start server
"${BINARY}" --transport http --addr ":${PORT}" > "${TMP}/mcp.log" 2>&1 &
MCP_PID=$!

# Wait for server up (max 5s)
for i in $(seq 1 50); do
  if curl -sS -o /dev/null --max-time 1 "http://localhost:${PORT}/mcp" 2>/dev/null; then
    break
  fi
  sleep 0.1
done

# Helpers ---------------------------------------------------------------------

call_mcp() {
  # $1 = endpoint URL, $2 = session header (may be empty), $3 = JSON-RPC body
  local url="$1" sid="$2" body="$3"
  local args=(-sS -X POST "$url"
    -H 'Content-Type: application/json'
    -H 'Accept: application/json, text/event-stream'
    -d "$body")
  if [ -n "$sid" ]; then
    args+=(-H "Mcp-Session-Id: $sid")
  fi
  curl "${args[@]}"
}

parse_response() {
  # Streamable HTTP may return either application/json (single payload) or
  # text/event-stream (one or more SSE events). Either way, the JSON-RPC
  # response is the first parseable JSON object in the body.
  /usr/bin/python3 -c "
import json, sys
text = sys.stdin.read().strip()
if text.startswith('event:') or '\ndata:' in text or text.startswith('data:'):
    for line in text.splitlines():
        if line.startswith('data:'):
            text = line[5:].strip()
            break
try:
    print(json.dumps(json.loads(text)))
except Exception as e:
    print(json.dumps({'error': {'message': str(e), 'raw': text[:300]}}))
"
}

PROBES_JSON="${TMP}/probes.json"
echo "[" > "${PROBES_JSON}"
FIRST=1
PASS=0; FAIL=0; SKIP=0

emit() {
  local name="$1" verdict="$2" detail="$3"
  if [ "${FIRST}" -eq 0 ]; then echo "," >> "${PROBES_JSON}"; fi
  FIRST=0
  printf '  {"name": "%s", "verdict": "%s", "detail": %s}' \
    "$name" "$verdict" \
    "$(printf '%s' "$detail" | /usr/bin/python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')" \
    >> "${PROBES_JSON}"
  printf '  %-20s %-7s %s\n' "$name" "$verdict" "$detail"
}

# Probes ---------------------------------------------------------------------

URL="http://localhost:${PORT}/mcp"
echo "==> mcp-shipcheck: 4 probes against ${URL}"
echo "  probe                verdict detail"

# 1. initialize -------------------------------------------------------------
HEADERS="${TMP}/init.headers"
INIT_OUT="${TMP}/init.json"
set +e
curl -sS -D "$HEADERS" -X POST "$URL" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"mcp-shipcheck","version":"0"}}}' \
  > "${INIT_OUT}"
RC=$?
set -e
SID=$(grep -i "Mcp-Session-Id:" "$HEADERS" 2>/dev/null | awk '{print $2}' | tr -d '\r' || true)
if [ "$RC" -eq 0 ] && [ -n "$SID" ]; then
  SERVER_NAME=$(parse_response < "$INIT_OUT" | /usr/bin/python3 -c "import json,sys;print(json.load(sys.stdin).get('result',{}).get('serverInfo',{}).get('name','?'))")
  emit "initialize" "pass" "serverInfo.name=${SERVER_NAME}"
  PASS=$((PASS+1))
else
  emit "initialize" "fail" "exit=$RC session=${SID:-none}"
  FAIL=$((FAIL+1))
  SID=""
fi

# Initialized notification (no response expected)
if [ -n "$SID" ]; then
  call_mcp "$URL" "$SID" '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null 2>&1 || true
fi

# 2. tools/list -------------------------------------------------------------
if [ -n "$SID" ]; then
  set +e
  TOOLS_OUT=$(call_mcp "$URL" "$SID" '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | parse_response)
  set -e
  N=$(echo "$TOOLS_OUT" | /usr/bin/python3 -c "import json,sys;d=json.load(sys.stdin);print(len(d.get('result',{}).get('tools',[])))")
  HAS_GW=$(echo "$TOOLS_OUT" | /usr/bin/python3 -c "
import json,sys
d=json.load(sys.stdin)
tools=[t['name'] for t in d.get('result',{}).get('tools',[])]
print('yes' if 'gorgias_search' in tools and 'gorgias_execute' in tools else 'no')
")
  if [ "$HAS_GW" = "yes" ] && [ "$N" -ge 10 ]; then
    emit "tools/list" "pass" "${N} tools, gateway present"
    PASS=$((PASS+1))
  else
    emit "tools/list" "fail" "${N} tools, gateway present: ${HAS_GW}"
    FAIL=$((FAIL+1))
  fi
else
  emit "tools/list" "skip" "initialize failed"
  SKIP=$((SKIP+1))
fi

# 3. gorgias_search ---------------------------------------------------------
if [ -n "$SID" ]; then
  set +e
  SEARCH_OUT=$(call_mcp "$URL" "$SID" '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"gorgias_search","arguments":{"query":"list tickets"}}}' | parse_response)
  set -e
  RESULT_TEXT=$(echo "$SEARCH_OUT" | /usr/bin/python3 -c "
import json,sys
d=json.load(sys.stdin)
content=d.get('result',{}).get('content',[])
for c in content:
    if c.get('type')=='text':
        print(c.get('text','')[:200])
        break
")
  if echo "$RESULT_TEXT" | grep -q 'tickets'; then
    emit "gorgias_search" "pass" "returned tickets endpoint(s)"
    PASS=$((PASS+1))
  else
    emit "gorgias_search" "fail" "no tickets endpoint in response: ${RESULT_TEXT:0:120}"
    FAIL=$((FAIL+1))
  fi
else
  emit "gorgias_search" "skip" "initialize failed"
  SKIP=$((SKIP+1))
fi

# 4. gorgias_execute (creds-gated) ------------------------------------------
if [ -z "$SID" ]; then
  emit "gorgias_execute" "skip" "initialize failed"
  SKIP=$((SKIP+1))
elif [ -z "${GORGIAS_API_KEY:-}" ] || [ -z "${GORGIAS_BASE_URL:-}" ]; then
  emit "gorgias_execute" "skip" "no credentials (set GORGIAS_API_KEY + GORGIAS_BASE_URL to enable)"
  SKIP=$((SKIP+1))
else
  set +e
  EXEC_OUT=$(call_mcp "$URL" "$SID" '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"gorgias_execute","arguments":{"endpoint_id":"tickets.list","params":{"limit":"1"}}}}' | parse_response)
  set -e
  EXEC_HAS_ENV=$(echo "$EXEC_OUT" | /usr/bin/python3 -c "
import json,sys
d=json.load(sys.stdin)
content=d.get('result',{}).get('content',[])
for c in content:
    if c.get('type')=='text':
        body=c.get('text','')
        try:
            j=json.loads(body)
            print('yes' if isinstance(j,dict) and ('data' in j or 'meta' in j) else 'no')
        except Exception:
            print('no')
        break
else:
    print('no')
")
  if [ "$EXEC_HAS_ENV" = "yes" ]; then
    emit "gorgias_execute" "pass" "tickets.list returned a list envelope"
    PASS=$((PASS+1))
  else
    emit "gorgias_execute" "fail" "tickets.list response not a list envelope"
    FAIL=$((FAIL+1))
  fi
fi

echo "" >> "${PROBES_JSON}"
echo "]" >> "${PROBES_JSON}"

TOTAL=$((PASS+FAIL))
VERDICT="PASS"
if [ "$FAIL" -gt 0 ]; then VERDICT="WARN"; fi
if [ "$TOTAL" -gt 0 ] && [ "$((FAIL*2))" -gt "$TOTAL" ]; then VERDICT="FAIL"; fi

BINARY_VERSION=$("${REPO}/gorgias-pp-cli" version 2>/dev/null | head -1 || echo "unknown")

/usr/bin/python3 - <<PY
import json
probes = json.load(open("${PROBES_JSON}"))
out = {
  "schema_version": 1,
  "verdict": "${VERDICT}",
  "executed_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "transport": "streamable-http",
  "binary_version": "${BINARY_VERSION}",
  "totals": {"pass": ${PASS}, "fail": ${FAIL}, "skip": ${SKIP}, "probes": ${TOTAL}+${SKIP}},
  "probes": probes,
}
json.dump(out, open("${REPO}/mcp-shipcheck.json", "w"), indent=2)
print()
print(f"==> verdict: {out['verdict']}  pass={out['totals']['pass']}  fail={out['totals']['fail']}  skip={out['totals']['skip']}")
print(f"==> artifact: ${REPO}/mcp-shipcheck.json")
PY
