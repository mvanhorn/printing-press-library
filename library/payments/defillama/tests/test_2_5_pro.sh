#!/bin/bash
# test_2_5_pro.sh -- pro commands gate on DEFILLAMA_PRO_KEY.
source "$(dirname "$0")/runner.sh"

# Force key absent for these checks. Subshell-only override.
unset_pro() { unset DEFILLAMA_PRO_KEY; }

test_01_bridges_gated() {
  unset_pro
  out=$("$CLI" bridges 2>&1)
  assert_exit_ne0 $? "bridges without pro key fails"
  assert_contains "$out" "DEFILLAMA_PRO_KEY|pro key|pro-key" "mentions pro key"
}

test_02_emissions_gated() {
  unset_pro
  out=$("$CLI" emissions aave 2>&1)
  assert_exit_ne0 $? "emissions without pro key"
  assert_contains "$out" "DEFILLAMA_PRO_KEY|pro key|pro-key" "mentions pro key"
}

test_03_hacks_gated() {
  unset_pro
  out=$("$CLI" hacks 2>&1)
  assert_exit_ne0 $? "hacks gated"
}

test_04_raises_gated() {
  unset_pro
  out=$("$CLI" raises 2>&1)
  assert_exit_ne0 $? "raises gated"
}

test_05_treasuries_gated() {
  unset_pro
  out=$("$CLI" treasuries 2>&1)
  assert_exit_ne0 $? "treasuries gated"
}

test_06_etfs_gated() {
  unset_pro
  out=$("$CLI" etfs 2>&1)
  assert_exit_ne0 $? "etfs gated"
}

test_07_rwa_gated() {
  unset_pro
  out=$("$CLI" rwa 2>&1)
  assert_exit_ne0 $? "rwa gated"
}

test_08_narratives_gated() {
  unset_pro
  out=$("$CLI" narratives 2>&1)
  assert_exit_ne0 $? "narratives gated"
}

test_09_derivatives_gated() {
  unset_pro
  out=$("$CLI" derivatives 2>&1)
  assert_exit_ne0 $? "derivatives gated"
}

test_10_sync_pro_gated() {
  unset_pro
  out=$("$CLI" sync --pro 2>&1)
  assert_exit_ne0 $? "sync --pro gated"
  assert_contains "$out" "DEFILLAMA_PRO_KEY|pro key|pro-key" "mentions pro key"
}

test_11_errors_clean_no_stacktrace() {
  unset_pro
  out=$("$CLI" bridges 2>&1)
  assert_not_contains "$out" "goroutine" "no stack trace in error"
  assert_not_contains "$out" "panic" "no panic"
}

dlpp_run_tests
