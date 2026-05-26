#!/bin/bash
# test_1_6_compare_metrics.sh -- compare --metrics filters which rows are shown.
source "$(dirname "$0")/runner.sh"

test_compare_metrics_subset() {
  full=$("$CLI" compare aave uniswap 2>/dev/null)
  tvl_only=$("$CLI" compare aave uniswap --metrics tvl 2>/dev/null)
  # default includes tvl, fees, revenue rows (multiple metric rows)
  # tvl-only should contain TVL and nothing else
  assert_contains "$tvl_only" "TVL" "tvl-only contains TVL row"
  assert_not_contains "$tvl_only" "FEES_24H" "tvl-only excludes FEES_24H"
  assert_not_contains "$tvl_only" "REV_24H" "tvl-only excludes REV_24H"
}

test_compare_metrics_fees_only() {
  out=$("$CLI" compare aave uniswap --metrics fees 2>/dev/null)
  assert_contains "$out" "FEES_24H" "fees-only contains FEES_24H"
  assert_not_contains "$out" "^TVL " "fees-only excludes plain TVL row"
}

test_compare_metrics_unknown_rejected() {
  "$CLI" compare aave uniswap --metrics bogusmetric >/dev/null 2>&1
  assert_exit_ne0 $? "unknown metric rejected"
}

dlpp_run_tests
