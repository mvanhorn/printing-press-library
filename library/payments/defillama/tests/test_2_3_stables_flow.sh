#!/bin/bash
# test_2_3_stables_flow.sh -- per-chain stablecoin supply deltas.
source "$(dirname "$0")/runner.sh"

test_01_flow_runs() {
  out=$("$CLI" stables flow --period 30d 2>&1)
  assert_exit_0 $? "stables flow --period 30d"
}

test_02_contains_ethereum() {
  out=$("$CLI" stables flow --period 30d 2>/dev/null)
  assert_contains "$out" "Ethereum" "ethereum present"
}

test_03_has_many_chains() {
  out=$("$CLI" stables flow --period 30d 2>/dev/null)
  count=$(printf '%s\n' "$out" | wc -l)
  if [ "$count" -ge 10 ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("expected >=10 rows in stables flow, got $count")
    FAIL=$((FAIL+1))
  fi
}

test_04_has_change_column() {
  out=$("$CLI" stables flow --period 30d 2>/dev/null)
  assert_contains "$out" "CHANGE" "CHANGE column present"
}

test_05_limit_works() {
  out=$("$CLI" stables flow --period 30d --limit 5 2>/dev/null)
  lines=$(printf '%s\n' "$out" | grep -c .)
  # 5 rows + 1 header = 6
  if [ "$lines" -le 7 ] && [ "$lines" -ge 5 ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("expected ~6 lines with --limit 5, got $lines")
    FAIL=$((FAIL+1))
  fi
}

test_06_json_numeric_change() {
  out=$("$CLI" stables flow --period 30d --limit 5 --json 2>/dev/null)
  assert_json "$out" "stables flow --json valid"
  assert_numeric_json "$out" "CHANGE" "CHANGE is numeric in JSON"
}

dlpp_run_tests
