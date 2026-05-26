#!/bin/bash
# Shared test harness for defillama-pp-cli.
# Sourced by individual test_*.sh files; can also be run directly to execute
# every test_*.sh in this directory.
#
# Each test file defines functions named test_*. The runner at the bottom of
# each file iterates them. This script provides the assert helpers and counters.

# Resolve repo paths regardless of where it's sourced from.
DLPP_TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DLPP_REPO_ROOT="$(cd "$DLPP_TESTS_DIR/.." && pwd)"
CLI="$DLPP_REPO_ROOT/defillama-pp-cli"

# Use a dedicated test data dir so we don't trample the user's real mirror.
export DEFILLAMA_PP_HOME="${DEFILLAMA_PP_HOME:-$HOME/.defillama-pp-test}"

PASS=0
FAIL=0
ERRORS=()

# Compatibility with `set -e` callers: helpers return non-zero on failure but
# never call `exit` themselves.

assert_exit_0() {
  local rc="$1" name="$2"
  if [ "$rc" -ne 0 ]; then
    ERRORS+=("$name: expected exit 0, got $rc")
    FAIL=$((FAIL+1))
    return 1
  fi
  PASS=$((PASS+1))
  return 0
}

assert_exit_ne0() {
  local rc="$1" name="$2"
  if [ "$rc" -eq 0 ]; then
    ERRORS+=("$name: expected non-zero exit, got 0")
    FAIL=$((FAIL+1))
    return 1
  fi
  PASS=$((PASS+1))
  return 0
}

# assert_contains "$output" "needle" "test name" -- substring match, case-insensitive
assert_contains() {
  local out="$1" needle="$2" name="$3"
  if printf '%s\n' "$out" | grep -qiE "$needle"; then
    PASS=$((PASS+1))
    return 0
  fi
  ERRORS+=("$name: output missing '$needle'")
  FAIL=$((FAIL+1))
  return 1
}

assert_not_contains() {
  local out="$1" needle="$2" name="$3"
  if printf '%s\n' "$out" | grep -qiE "$needle"; then
    ERRORS+=("$name: output should NOT contain '$needle'")
    FAIL=$((FAIL+1))
    return 1
  fi
  PASS=$((PASS+1))
  return 0
}

assert_json() {
  local out="$1" name="$2"
  if printf '%s' "$out" | python3 -c 'import sys,json; json.load(sys.stdin)' 2>/dev/null; then
    PASS=$((PASS+1))
    return 0
  fi
  ERRORS+=("$name: invalid JSON")
  FAIL=$((FAIL+1))
  return 1
}

# Asserts the first row's $field is a number, not a string.
assert_numeric_json() {
  local out="$1" field="$2" name="$3"
  if printf '%s' "$out" \
    | FIELD="$field" python3 -c '
import sys,json,os
d=json.load(sys.stdin)
row=d[0] if isinstance(d,list) and d else d
val=row.get(os.environ["FIELD"])
assert isinstance(val,(int,float)) and not isinstance(val,bool), \
  f"expected numeric, got {type(val).__name__}: {val!r}"
' 2>/dev/null; then
    PASS=$((PASS+1))
    return 0
  fi
  ERRORS+=("$name: $field not numeric in JSON")
  FAIL=$((FAIL+1))
  return 1
}

assert_line_count_gte() {
  local out="$1" min="$2" name="$3"
  local count
  count=$(printf '%s' "$out" | grep -c '^' || true)
  if [ "$count" -ge "$min" ]; then
    PASS=$((PASS+1))
    return 0
  fi
  ERRORS+=("$name: expected >= $min lines, got $count")
  FAIL=$((FAIL+1))
  return 1
}

assert_line_count_eq() {
  local out="$1" want="$2" name="$3"
  local count
  count=$(printf '%s' "$out" | grep -c '^' || true)
  if [ "$count" -eq "$want" ]; then
    PASS=$((PASS+1))
    return 0
  fi
  ERRORS+=("$name: expected $want lines, got $count")
  FAIL=$((FAIL+1))
  return 1
}

# Runs every test_* function declared in the calling shell and prints a summary.
# Should be the last thing each test_*.sh file calls.
dlpp_run_tests() {
  local fn
  for fn in $(declare -F | awk '{print $3}' | grep '^test_'); do
    printf '  %-60s ' "$fn"
    local before_fail=$FAIL
    "$fn"
    if [ "$FAIL" -gt "$before_fail" ]; then
      printf 'FAIL\n'
    else
      printf 'PASS\n'
    fi
  done
  echo
  printf '%d passed, %d failed\n' "$PASS" "$FAIL"
  if [ "${#ERRORS[@]}" -gt 0 ]; then
    printf '  - %s\n' "${ERRORS[@]}"
  fi
  if [ "$FAIL" -gt 0 ]; then
    return 1
  fi
  return 0
}

# When invoked directly, run every test file in this directory.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  rc=0
  for f in "$DLPP_TESTS_DIR"/test_*.sh; do
    [ -e "$f" ] || continue
    echo "=== $(basename "$f") ==="
    bash "$f" || rc=1
    echo
  done
  exit $rc
fi
