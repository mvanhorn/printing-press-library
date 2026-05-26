#!/bin/bash
# test_2_4_tier2.sh -- tvl, fees, dexs, options Tier 2 commands.
source "$(dirname "$0")/runner.sh"

# --- tvl
test_01_tvl_protocol() {
  out=$("$CLI" tvl aave 2>/dev/null)
  assert_exit_0 $? "tvl aave"
  assert_contains "$out" '\$' "tvl shows dollar value"
}

test_02_tvl_protocol_history() {
  "$CLI" sync --protocol aave >/dev/null 2>&1
  out=$("$CLI" tvl aave --history --period 30d 2>/dev/null)
  assert_exit_0 $? "tvl --history exits 0"
  assert_contains "$out" "20[0-9][0-9]-[0-1][0-9]-[0-3][0-9]" "history has dates"
}

test_03_tvl_chain() {
  out=$("$CLI" tvl --chain ethereum --limit 10 2>/dev/null)
  assert_exit_0 $? "tvl --chain ethereum"
  assert_line_count_gte "$out" 5 "multiple rows"
}

# --- fees
test_04_fees_protocol() {
  out=$("$CLI" fees aave 2>/dev/null)
  assert_exit_0 $? "fees aave"
  assert_contains "$out" "FEES|REV" "fees output"
}

test_05_fees_history() {
  out=$("$CLI" fees aave --history --period 30d 2>/dev/null)
  assert_exit_0 $? "fees --history"
  assert_contains "$out" "20[0-9][0-9]-[0-1][0-9]" "fees history dates"
}

test_06_fees_chain() {
  out=$("$CLI" fees --chain arbitrum --sort revenue --limit 10 2>/dev/null)
  assert_exit_0 $? "fees --chain arbitrum"
  assert_line_count_gte "$out" 3 "fees --chain rows"
}

# --- dexs
test_07_dexs_overview() {
  out=$("$CLI" dexs --limit 10 2>/dev/null)
  assert_exit_0 $? "dexs overview"
  assert_line_count_gte "$out" 5 "dexs rows"
}

test_08_dexs_protocol_history() {
  "$CLI" sync --protocol uniswap >/dev/null 2>&1
  out=$("$CLI" dexs uniswap --history --period 30d 2>/dev/null)
  assert_exit_0 $? "dexs uniswap --history"
  assert_contains "$out" "20[0-9][0-9]-[0-1][0-9]" "dexs history dates"
}

test_09_dexs_chain() {
  out=$("$CLI" dexs --chain arbitrum --limit 10 2>/dev/null)
  assert_exit_0 $? "dexs --chain arbitrum"
}

test_10_dexs_market_share() {
  out=$("$CLI" dexs --sort volume_24h --limit 5 --with market-share 2>/dev/null)
  assert_exit_0 $? "dexs --with market-share"
  assert_contains "$out" "MARKET_SHARE|SHARE" "has market share column"
}

# --- options
test_11_options() {
  out=$("$CLI" options 2>/dev/null)
  assert_exit_0 $? "options"
}

test_12_options_chain() {
  out=$("$CLI" options --chain arbitrum 2>/dev/null)
  assert_exit_0 $? "options --chain arbitrum"
}

dlpp_run_tests
