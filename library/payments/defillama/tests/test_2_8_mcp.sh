#!/bin/bash
# test_2_8_mcp.sh -- MCP stdio server.
source "$(dirname "$0")/runner.sh"

# MCP server reads stdin and exits on EOF. We rely on natural EOF instead of
# `timeout` (which isn't installed on macOS by default).
mcp_request() {
  local req="$1"
  printf '%s\n' "$req" | "$CLI" mcp 2>/dev/null
}

mcp_requests() {
  local payload="$1"
  printf '%s\n' "$payload" | "$CLI" mcp 2>/dev/null
}

test_01_initialize() {
  out=$(mcp_request '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}')
  assert_json "$out" "initialize response valid JSON"
  assert_contains "$out" "capabilities" "has capabilities"
  assert_contains "$out" "serverInfo" "has serverInfo"
}

test_02_tools_list() {
  payload=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
  out=$(mcp_requests "$payload")
  for t in defillama_top defillama_compare defillama_yields defillama_stables defillama_fees defillama_dexs defillama_profile defillama_price defillama_sql defillama_sync; do
    assert_contains "$out" "$t" "tools/list contains $t"
  done
}

test_03_call_top() {
  payload=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"defillama_top","arguments":{"chain":"ethereum","limit":3}}}'
  out=$(mcp_requests "$payload")
  assert_contains "$out" "TVL|tvl" "top response has tvl data"
}

test_04_sql_readonly_guard() {
  payload=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"defillama_sql","arguments":{"query":"DROP TABLE protocols"}}}'
  out=$(mcp_requests "$payload")
  assert_contains "$out" "SELECT|read-only|only SELECT" "DROP rejected by guard"
}

test_05_call_price() {
  payload=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"defillama_price","arguments":{"coins":"coingecko:ethereum"}}}'
  out=$(mcp_requests "$payload")
  assert_contains "$out" "ETH|ethereum|price" "price response contains data"
}

dlpp_run_tests
