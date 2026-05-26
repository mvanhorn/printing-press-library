#!/bin/bash
# test_1_8_empty_status.sh -- empty DB shows all domains with "never".
source "$(dirname "$0")/runner.sh"

test_empty_status_shows_never() {
  emptydir=$(mktemp -d)
  out=$(DEFILLAMA_PP_HOME="$emptydir" "$CLI" sync --status --no-sync 2>/dev/null)
  assert_exit_0 $? "empty status exits 0"
  assert_contains "$out" "DOMAIN" "header present"
  assert_contains "$out" "never" "shows 'never' for unsynced"
  assert_contains "$out" "protocols" "lists protocols domain"
  assert_contains "$out" "pools|yields" "lists pools/yields domain"
  rm -rf "$emptydir"
}

dlpp_run_tests
