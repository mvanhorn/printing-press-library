#!/bin/bash
# test_2_2_history_flags.sh -- --history / --period on compare, profile, yields.
source "$(dirname "$0")/runner.sh"

test_01_setup_sync_protocols() {
  "$CLI" sync --protocol aave >/dev/null 2>&1
  "$CLI" sync --protocol uniswap >/dev/null 2>&1
  assert_exit_0 $? "setup syncs ok"
}

test_02_compare_period_has_dates() {
  out=$("$CLI" compare aave uniswap --period 30d 2>/dev/null)
  assert_exit_0 $? "compare --period 30d exits 0"
  assert_line_count_gte "$out" 10 "compare period has >=10 lines"
  # date row -- something like 2026-04-01 or similar
  assert_contains "$out" "20[0-9][0-9]-[0-1][0-9]-[0-3][0-9]" "compare contains date strings"
}

test_03_profile_period_has_history() {
  shortlen=$("$CLI" profile aave 2>/dev/null | wc -l)
  longlen=$("$CLI" profile aave --period 90d 2>/dev/null | wc -l)
  if [ "$longlen" -gt "$shortlen" ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("profile --period not longer than profile (long=$longlen short=$shortlen)")
    FAIL=$((FAIL+1))
  fi
}

test_04_profile_period_has_dates() {
  out=$("$CLI" profile aave --period 90d 2>/dev/null)
  assert_contains "$out" "20[0-9][0-9]-[0-1][0-9]-[0-3][0-9]" "profile period has date strings"
}

test_05_yields_pool_history() {
  pool=$("$CLI" --no-sync sql "SELECT pool_id FROM pools WHERE chain = 'Ethereum' AND tvl_usd > 100000000 ORDER BY tvl_usd DESC LIMIT 1" --csv --no-header 2>/dev/null | tr -d ' "')
  if [ -z "$pool" ]; then
    ERRORS+=("could not pick a pool_id from DB")
    FAIL=$((FAIL+1))
    return
  fi
  out=$("$CLI" yields top --pool "$pool" --history --period 30d 2>&1)
  assert_exit_0 $? "yields --history exits 0 (pool=$pool)"
  assert_contains "$out" "20[0-9][0-9]-[0-1][0-9]-[0-3][0-9]" "yields history has date strings"
}

dlpp_run_tests
