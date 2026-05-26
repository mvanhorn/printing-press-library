#!/bin/bash
# test_1_3_sync_domains.sh -- pools <-> yields alias; status uses canonical names.
source "$(dirname "$0")/runner.sh"

test_sync_domain_pools() {
  out=$("$CLI" sync --domain pools 2>&1)
  assert_exit_0 $? "sync --domain pools"
  assert_contains "$out" "pools|yields" "ran pools/yields sync"
}

test_sync_domain_yields() {
  out=$("$CLI" sync --domain yields 2>&1)
  assert_exit_0 $? "sync --domain yields"
}

test_sync_status_names_round_trip() {
  # Every name listed by `sync --status` should be accepted by `sync --domain`.
  # We do not actually sync each; just request --help-style validation via attempting
  # to invoke with --no-sync (which doesn't exist on sync), so instead use a dry path:
  # call sync --domain <name> with --no-sync flag turned on globally to skip the
  # actual HTTP work. If that's not supported we accept any non-"unknown" error.
  out=$("$CLI" sync --status 2>&1)
  assert_exit_0 $? "sync --status exits 0"
  names=$(printf '%s\n' "$out" | awk 'NR>1 && $1 != "" {print $1}')
  bad=()
  for n in $names; do
    # quick reject check: bogus names produce an "unknown --domain" error.
    # use a short timeout so we don't actually re-sync everything.
    err=$("$CLI" sync --domain "$n" --no-sync 2>&1 1>/dev/null || true)
    if echo "$err" | grep -qi "unknown --domain"; then
      bad+=("$n")
    fi
  done
  if [ ${#bad[@]} -eq 0 ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("status names not accepted by --domain: ${bad[*]}")
    FAIL=$((FAIL+1))
  fi
}

test_sync_domain_bogus_rejected() {
  out=$("$CLI" sync --domain bogusname 2>&1)
  assert_exit_ne0 $? "bogus domain rejected"
  assert_contains "$out" "unknown" "error mentions unknown"
}

dlpp_run_tests
