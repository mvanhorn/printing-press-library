#!/bin/bash
# test_full_suite.sh -- run every test_*.sh in order, print a summary table.
set -u

DLPP_TESTS_DIR="$(cd "$(dirname "$0")" && pwd)"

phase_header() {
  echo
  echo "===== $1 ====="
}

run_file() {
  local f="$1"
  local out
  out=$(bash "$f" 2>&1)
  local rc=$?
  # The last "X passed, Y failed" line is our scoreboard.
  local summary
  summary=$(printf '%s\n' "$out" | grep -E '^[0-9]+ passed, [0-9]+ failed$' | tail -1)
  if [ -z "$summary" ]; then
    summary="0 passed, 0 failed (no summary line found)"
  fi
  local pass fail
  pass=$(printf '%s\n' "$summary" | awk '{print $1}')
  fail=$(printf '%s\n' "$summary" | awk '{print $4}' | sed 's/[^0-9]//g')
  : "${fail:=0}"
  printf '  %-40s %3s/%-3s %s\n' "$(basename "$f")" "$pass" "$((pass+fail))" \
    "$([ "$fail" -eq 0 ] && echo PASS || echo FAIL)"
  if [ "$fail" -ne 0 ]; then
    # Re-emit the failure block so the operator can fix it.
    printf '%s\n' "$out" | grep -E '^  - ' || true
  fi
  TOTAL_PASS=$((TOTAL_PASS + pass))
  TOTAL_FAIL=$((TOTAL_FAIL + fail))
  return $rc
}

TOTAL_PASS=0
TOTAL_FAIL=0

phase_header "Phase 1: Bug Fixes"
for f in "$DLPP_TESTS_DIR"/test_1_*.sh; do
  [ -e "$f" ] || continue
  run_file "$f" || true
done

phase_header "Phase 2: Expanded Build"
for f in "$DLPP_TESTS_DIR"/test_2_*.sh; do
  [ -e "$f" ] || continue
  run_file "$f" || true
done

echo
printf 'TOTAL: %d/%d %s\n' "$TOTAL_PASS" "$((TOTAL_PASS+TOTAL_FAIL))" \
  "$([ "$TOTAL_FAIL" -eq 0 ] && echo PASS || echo FAIL)"

[ "$TOTAL_FAIL" -eq 0 ]
