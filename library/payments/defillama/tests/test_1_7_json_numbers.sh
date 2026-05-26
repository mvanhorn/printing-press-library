#!/bin/bash
# test_1_7_json_numbers.sh -- all command JSON output emits raw numbers.
source "$(dirname "$0")/runner.sh"

test_top_json_tvl_numeric() {
  out=$("$CLI" top --limit 3 --json 2>/dev/null)
  assert_json "$out" "top --json valid"
  assert_numeric_json "$out" "TVL" "top: TVL numeric in JSON"
}

test_yields_json_apy_numeric() {
  out=$("$CLI" yields top --limit 3 --json 2>/dev/null)
  assert_json "$out" "yields --json valid"
  assert_numeric_json "$out" "APY" "yields: APY numeric in JSON"
  assert_numeric_json "$out" "TVL" "yields: TVL numeric in JSON"
}

test_stables_json_circulating_numeric() {
  out=$("$CLI" stables --limit 3 --json 2>/dev/null)
  assert_json "$out" "stables --json valid"
  assert_numeric_json "$out" "CIRCULATING" "stables: CIRCULATING numeric"
}

test_chains_json_tvl_numeric() {
  out=$("$CLI" chains --limit 3 --json 2>/dev/null)
  assert_json "$out" "chains --json valid"
  assert_numeric_json "$out" "TVL" "chains: TVL numeric"
}

dlpp_run_tests
