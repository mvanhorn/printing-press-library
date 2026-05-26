#!/bin/bash
# test_2_1_historical.sh -- sync --protocol, sync --chain, --backfill, period parser.
# Functions are run alphabetically, so numeric prefixes encode order.
source "$(dirname "$0")/runner.sh"

test_01_sync_protocol_aave() {
  out=$("$CLI" sync --protocol aave 2>&1)
  assert_exit_0 $? "sync --protocol aave"
}

test_02_protocol_tvl_hist_rows() {
  count=$("$CLI" --no-sync sql "SELECT COUNT(*) FROM protocol_tvl_hist" --csv --no-header 2>/dev/null | tr -d ' "')
  if [ "${count:-0}" -gt 0 ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("protocol_tvl_hist empty after sync --protocol aave")
    FAIL=$((FAIL+1))
  fi
}

test_03_fees_hist_rows() {
  count=$("$CLI" --no-sync sql "SELECT COUNT(*) FROM fees_hist" --csv --no-header 2>/dev/null | tr -d ' "')
  if [ "${count:-0}" -gt 0 ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("fees_hist empty after sync --protocol aave")
    FAIL=$((FAIL+1))
  fi
}

test_04_sync_chain_ethereum() {
  out=$("$CLI" sync --chain Ethereum 2>&1)
  assert_exit_0 $? "sync --chain Ethereum"
}

test_05_chain_tvl_hist_rows() {
  count=$("$CLI" --no-sync sql "SELECT COUNT(*) FROM chain_tvl_hist WHERE LOWER(chain) = 'ethereum'" --csv --no-header 2>/dev/null | tr -d ' "')
  if [ "${count:-0}" -gt 30 ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("chain_tvl_hist short after sync --chain (got ${count:-0})")
    FAIL=$((FAIL+1))
  fi
}

test_06_date_range_30d_plus() {
  count=$("$CLI" --no-sync sql "SELECT COUNT(DISTINCT date) FROM protocol_tvl_hist" --csv --no-header 2>/dev/null | tr -d ' "')
  if [ "${count:-0}" -ge 30 ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("date range too small (count=$count)")
    FAIL=$((FAIL+1))
  fi
}

test_07_backfill_goes_back_years() {
  out=$("$CLI" sync --protocol aave --backfill 2>&1)
  assert_exit_0 $? "sync --protocol aave --backfill"
  min_date=$("$CLI" --no-sync sql "SELECT MIN(date) FROM protocol_tvl_hist" --csv --no-header 2>/dev/null | tr -d ' "')
  if [ -n "$min_date" ] && [ "$min_date" \< "2024-01-01" ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("backfill min_date not pre-2024: $min_date")
    FAIL=$((FAIL+1))
  fi
}

test_08_sync_protocol_nonexistent() {
  out=$("$CLI" sync --protocol not_a_real_protocol_xyz999 2>&1)
  assert_exit_ne0 $? "bogus --protocol rejected"
}

dlpp_run_tests
