#!/bin/bash
# test_1_5_sql_formatting.sh -- no scientific notation; $K/M/B for monetary in tables.
source "$(dirname "$0")/runner.sh"

test_sql_table_no_scinote() {
  out=$("$CLI" sql "SELECT name, circulating FROM stablecoins ORDER BY circulating DESC LIMIT 3" 2>/dev/null)
  assert_exit_0 $? "sql exits 0"
  assert_not_contains "$out" "e\+" "no e+ in table output"
  assert_not_contains "$out" "e-" "no e- in table output"
}

test_sql_table_monetary_formatting() {
  out=$("$CLI" sql "SELECT name, tvl FROM protocols ORDER BY tvl DESC LIMIT 3" 2>/dev/null)
  assert_contains "$out" '\$' "tvl displays as $-formatted"
  assert_contains "$out" "B|M|K" "tvl uses K/M/B suffix"
}

test_sql_json_raw_numbers() {
  out=$("$CLI" sql "SELECT name, circulating FROM stablecoins ORDER BY circulating DESC LIMIT 3" --json 2>/dev/null)
  assert_json "$out" "sql --json valid"
  assert_not_contains "$out" "e\+" "no e+ in JSON"
  assert_not_contains "$out" "e-" "no e- in JSON"
  assert_numeric_json "$out" "CIRCULATING" "CIRCULATING is numeric in JSON"
}

test_sql_csv_raw_numbers() {
  out=$("$CLI" sql "SELECT name, circulating FROM stablecoins ORDER BY circulating DESC LIMIT 3" --csv 2>/dev/null)
  assert_exit_0 $? "sql --csv exits 0"
  assert_not_contains "$out" "e\+" "no scientific notation in CSV"
}

dlpp_run_tests
