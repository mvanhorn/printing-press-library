#!/bin/bash
# test_1_4_protocol_resolution.sh -- two-pass slug resolution incl. renames.
source "$(dirname "$0")/runner.sh"

test_profile_resolves_eigenlayer() {
  out=$("$CLI" profile eigenlayer 2>&1)
  assert_exit_0 $? "profile eigenlayer exits 0"
  assert_contains "$out" "PROTOCOL" "profile returns protocol record"
}

test_compare_resolves_eigenlayer() {
  out=$("$CLI" compare lido eigenlayer 2>&1)
  assert_exit_0 $? "compare lido eigenlayer exits 0"
  assert_contains "$out" "METRIC" "compare returns table"
}

test_profile_uppercase_aave() {
  out=$("$CLI" profile AAVE 2>&1)
  assert_exit_0 $? "profile AAVE exits 0"
}

test_profile_lowercase_aave() {
  out=$("$CLI" profile aave 2>&1)
  assert_exit_0 $? "profile aave exits 0"
}

test_profile_notfound() {
  out=$("$CLI" profile this_is_not_a_real_protocol_xyz123 2>&1)
  assert_exit_ne0 $? "bogus protocol exits non-zero"
  assert_contains "$out" "not found" "clean not-found error"
}

dlpp_run_tests
