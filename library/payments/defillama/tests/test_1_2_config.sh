#!/bin/bash
# test_1_2_config.sh -- canonical hyphenated config keys; accept hyphen + underscore.
source "$(dirname "$0")/runner.sh"

test_config_show_uses_hyphenated_keys() {
  out=$("$CLI" config show 2>/dev/null)
  assert_exit_0 $? "config show exits 0"
  assert_contains "$out" "stale-threshold" "show uses stale-threshold"
  assert_contains "$out" "stale-historical" "show uses stale-historical"
  assert_contains "$out" "pro-key" "show uses pro-key"
  assert_not_contains "$out" "stale_overview" "no stale_overview underscore"
}

test_config_set_hyphenated() {
  "$CLI" config set stale-threshold 30m >/dev/null 2>&1
  assert_exit_0 $? "set stale-threshold 30m"
  out=$("$CLI" config show 2>/dev/null)
  assert_contains "$out" "30m" "show reflects 30m"
}

test_config_set_underscored_alias() {
  "$CLI" config set stale_threshold 45m >/dev/null 2>&1
  assert_exit_0 $? "set stale_threshold (underscore alias)"
  out=$("$CLI" config show 2>/dev/null)
  assert_contains "$out" "45m" "show reflects 45m"
}

test_config_reset() {
  "$CLI" config set stale-threshold 1h >/dev/null 2>&1
  assert_exit_0 $? "reset stale-threshold to 1h"
}

test_config_set_bogus_key() {
  "$CLI" config set bogus-key value >/dev/null 2>&1
  assert_exit_ne0 $? "bogus key rejected"
}

dlpp_run_tests
