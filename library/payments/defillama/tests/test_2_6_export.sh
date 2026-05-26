#!/bin/bash
# test_2_6_export.sh -- export writes SQL output to a file.
source "$(dirname "$0")/runner.sh"

EXPORT_TMP=$(mktemp -d)
cleanup() { rm -rf "$EXPORT_TMP"; }
trap cleanup EXIT

test_01_export_csv() {
  out=$(cd "$EXPORT_TMP" && "$CLI" export --format csv --query "SELECT name, tvl FROM protocols LIMIT 5" 2>&1)
  assert_exit_0 $? "export csv exits 0"
  # output should mention a filename
  f=$(printf '%s\n' "$out" | grep -oE '[^ ]+\.csv' | head -1)
  if [ -z "$f" ]; then f=$(ls "$EXPORT_TMP"/*.csv 2>/dev/null | head -1); fi
  if [ -n "$f" ] && [ -f "$f" ]; then
    lines=$(wc -l < "$f")
    if [ "$lines" -ge 6 ]; then
      PASS=$((PASS+1))
    else
      ERRORS+=("csv file too short: $lines lines (expected >=6)")
      FAIL=$((FAIL+1))
    fi
  else
    ERRORS+=("no csv file produced; out=$out")
    FAIL=$((FAIL+1))
  fi
}

test_02_export_json() {
  out=$(cd "$EXPORT_TMP" && "$CLI" export --format json --query "SELECT name, tvl FROM protocols LIMIT 5" 2>&1)
  assert_exit_0 $? "export json exits 0"
  f=$(printf '%s\n' "$out" | grep -oE '[^ ]+\.json' | head -1)
  if [ -z "$f" ]; then f=$(ls "$EXPORT_TMP"/*.json 2>/dev/null | head -1); fi
  if [ -n "$f" ] && [ -f "$f" ]; then
    body=$(cat "$f")
    assert_json "$body" "exported json valid"
  else
    ERRORS+=("no json file produced; out=$out")
    FAIL=$((FAIL+1))
  fi
}

test_03_export_custom_path() {
  path="$EXPORT_TMP/custom-output.csv"
  "$CLI" export --format csv --query "SELECT name, tvl FROM protocols LIMIT 3" --output "$path" >/dev/null 2>&1
  if [ -f "$path" ]; then
    lines=$(wc -l < "$path")
    if [ "$lines" -ge 4 ]; then
      PASS=$((PASS+1))
    else
      ERRORS+=("custom-output csv too short: $lines")
      FAIL=$((FAIL+1))
    fi
  else
    ERRORS+=("--output path not honored")
    FAIL=$((FAIL+1))
  fi
}

test_04_export_rejects_write() {
  "$CLI" export --format csv --query "DELETE FROM protocols" >/dev/null 2>&1
  assert_exit_ne0 $? "export rejects DML"
}

dlpp_run_tests
