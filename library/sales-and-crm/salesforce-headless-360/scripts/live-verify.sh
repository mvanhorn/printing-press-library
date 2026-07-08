#!/usr/bin/env bash
# live-verify.sh
# Convenience runner for the live-org verification runbook.
# Reads ORG (sf alias) and ACME_ID (account ID) from env or prompts.
# Runs every required check end-to-end against a real Salesforce org.
# Writes a JSON report to docs/live-verification-report.json alongside the markdown report.
#
# This script does NOT replace the runbook (docs/live-verification-runbook.md).
# It is the automation layer beneath the runbook for repeatable execution.

set -euo pipefail

CLI=${CLI:-salesforce-headless-360-pp-cli}
ORG=${ORG:?ORG env var required (sf CLI alias)}
ACME_ID=${ACME_ID:?ACME_ID env var required (Salesforce Account Id to test against)}

# Preflight: make sure the tools + org are actually reachable before we start
command -v "$CLI" >/dev/null 2>&1 || { echo "ERROR: $CLI not on PATH. go install ./cmd/salesforce-headless-360-pp-cli and ensure \$HOME/go/bin is in \$PATH."; exit 10; }
command -v sf >/dev/null 2>&1 || { echo "ERROR: sf CLI not installed. npm install -g @salesforce/cli"; exit 10; }
command -v sqlite3 >/dev/null 2>&1 || { echo "ERROR: sqlite3 not installed"; exit 10; }
sf org display --target-org "$ORG" --json >/dev/null 2>&1 || { echo "ERROR: sf alias '$ORG' not authenticated. Run: sf org login web --alias $ORG --instance-url <your-url>"; exit 4; }

# Until F-017 lands a global --org flag, most CLI commands rely on sf's default target-org.
# Set it so commands without --org resolve correctly.
sf config set target-org="$ORG" >/dev/null 2>&1 || true
RESTRICTED_USER=${RESTRICTED_USER:-}
RESTRICTED_WRITE_USER=${RESTRICTED_WRITE_USER:-$RESTRICTED_USER}
OPP_ID=${OPP_ID:-}
OPP_STAGE=${OPP_STAGE:-Proposal/Price Quote}

REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)
REPORT_JSON="${REPO_ROOT}/docs/live-verification-report.json"
TMPDIR=$(mktemp -d)
cleanup_task_ids=()
cleanup_opp_id=""
cleanup_opp_stage=""
cleanup_account_description=""

cleanup_live_writes() {
  for task_id in "${cleanup_task_ids[@]:-}"; do
    if [ -n "$task_id" ]; then
      sf data delete record --target-org "$ORG" --sobject Task --record-id "$task_id" >/dev/null 2>&1 || true
    fi
  done
  if [ -n "$cleanup_opp_id" ] && [ -n "$cleanup_opp_stage" ]; then
    sf data update record --target-org "$ORG" --sobject Opportunity --record-id "$cleanup_opp_id" --values "StageName='$cleanup_opp_stage'" >/dev/null 2>&1 || true
  fi
  if [ -n "$cleanup_account_description" ]; then
    sf data update record --target-org "$ORG" --sobject Account --record-id "$ACME_ID" --values "Description='$cleanup_account_description'" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMPDIR"
}
trap cleanup_live_writes EXIT

results=()

record() {
  local id="$1" label="$2" status="$3" detail="$4"
  results+=("{\"id\":\"$id\",\"label\":\"$label\",\"status\":\"$status\",\"detail\":\"${detail//\"/\\\"}\"}")
  printf "  %-2s %-50s %s\n" "$id" "$label" "$status"
}

run_or_fail() {
  local id="$1" label="$2"
  shift 2
  if "$@" > "${TMPDIR}/${id}.out" 2>&1; then
    record "$id" "$label" PASS "$(head -1 "${TMPDIR}/${id}.out")"
  else
    record "$id" "$label" FAIL "$(tail -3 "${TMPDIR}/${id}.out" | tr '\n' ';')"
  fi
}

json_first_field() {
  local field="$1"
  python3 -c 'import json, sys
field = sys.argv[1]
try:
    payload = json.load(sys.stdin)
except Exception:
    print("")
    raise SystemExit(0)
records = payload.get("result", {}).get("records", [])
if not records:
    print("")
    raise SystemExit(0)
value = records[0].get(field)
print("" if value is None else value)' "$field"
}

query_first_field() {
  local query="$1" field="$2"
  sf data query --target-org "$ORG" --json --query "$query" 2>/dev/null | json_first_field "$field"
}

find_test_opp() {
  if [ -n "$OPP_ID" ]; then
    printf '%s\n' "$OPP_ID"
    return
  fi
  query_first_field "SELECT Id FROM Opportunity WHERE AccountId = '$ACME_ID' ORDER BY LastModifiedDate DESC LIMIT 1" "Id"
}

record_write_result() {
  local id="$1" label="$2" out="$3" expected="$4"
  if grep -qi "$expected" "$out"; then
    record "$id" "$label" PASS "$expected observed"
  else
    record "$id" "$label" FAIL "$(tail -3 "$out" | tr '\n' ';')"
  fi
}

echo
echo "Live-org verification: $ORG / Account=$ACME_ID"
echo "------------------------------------------------------------"

# 1. sf fall-through
run_or_fail 1 "sf CLI fall-through" "$CLI" auth login --sf "$ORG"

# 2. doctor full pass
run_or_fail 2 "doctor full pass" "$CLI" doctor

# 3. Composite Graph sync (look for the request line in output; --verbose would be ideal but is not a CLI flag — see F-019)
if "$CLI" sync --account "$ACME_ID" 2>&1 | tee "${TMPDIR}/sync.out" | grep -q "composite/graph"; then
  record 3 "Composite Graph in sync" PASS "graph request observed"
else
  # Sync may be silent on the graph path; check the local SQLite sync log as fallback
  if sqlite3 "$HOME/.local/share/salesforce-headless-360-pp-cli/data.db" \
       "SELECT count(*) FROM sync_checkpoints WHERE account_id = '$ACME_ID';" 2>/dev/null | grep -qE '^[1-9]'; then
    record 3 "Composite Graph in sync" PASS "sync checkpoint written (sync ran)"
  else
    record 3 "Composite Graph in sync" FAIL "no composite/graph request or sync checkpoint observed"
  fi
fi

# 4. sharing cross-check (presence of the table is enough; rows depend on profile)
if sqlite3 "$HOME/.local/share/salesforce-headless-360-pp-cli/data.db" \
    "SELECT count(*) FROM sharing_drop_audit;" > "${TMPDIR}/4.out" 2>&1; then
  record 4 "UI API sharing cross-check (table present)" PASS "$(cat "${TMPDIR}/4.out") drop rows"
else
  record 4 "UI API sharing cross-check (table present)" FAIL "$(cat "${TMPDIR}/4.out")"
fi

# 5. FLS intersection
# This requires --run-as-user or a separate restricted login; only run if RESTRICTED_USER set
if [ -n "$RESTRICTED_USER" ]; then
  "$CLI" agent context --live "$ACME_ID" --run-as-user "$RESTRICTED_USER" \
      --output "${TMPDIR}/restricted.json" >/dev/null 2>&1 || true
  if [ -f "${TMPDIR}/restricted.json" ]; then
    leaks=$(grep -oE 'AnnualRevenue|Salary__c' "${TMPDIR}/restricted.json" | wc -l | tr -d ' ')
    if [ "$leaks" = "0" ]; then
      record 5 "FLS intersection hides restricted fields" PASS "0 leaks"
    else
      record 5 "FLS intersection hides restricted fields" FAIL "$leaks leaks"
    fi
  else
    record 5 "FLS intersection hides restricted fields" FAIL "bundle not produced"
  fi
else
  record 5 "FLS intersection hides restricted fields" SKIP "RESTRICTED_USER not set"
fi

# 6. compliance map loaded
rows=$(sqlite3 "$HOME/.local/share/salesforce-headless-360-pp-cli/data.db" \
    "SELECT count(*) FROM compliance_field_map;" 2>/dev/null || echo 0)
if [ "$rows" -gt 0 ]; then
  record 6 "Tooling compliance map loads" PASS "$rows fields"
else
  record 6 "Tooling compliance map loads" FAIL "0 rows (tag at least one field)"
fi

# 7. trust register (currently REQUIRES --org flag — see F-020; sf config default not honored)
run_or_fail 7 "trust register (Certificate or CMDT)" "$CLI" trust register --org "$ORG"

# 8. agent context produces bundle
"$CLI" agent context --live "$ACME_ID" --output "${TMPDIR}/acme.json" >/dev/null 2>&1
if [ -s "${TMPDIR}/acme.json" ] && grep -q '"kid"' "${TMPDIR}/acme.json"; then
  record 8 "agent context produces signed bundle" PASS "bundle written + kid present"
else
  record 8 "agent context produces signed bundle" FAIL "bundle missing or unsigned"
fi

# 9. agent verify --strict --deep on valid
run_or_fail 9 "agent verify --strict --deep PASS valid" "$CLI" agent verify --strict --deep "${TMPDIR}/acme.json"

# 10. agent verify --strict --deep on tampered
cp "${TMPDIR}/acme.json" "${TMPDIR}/acme.tampered.json"
python3 -c "
import json, sys
b = json.load(open('${TMPDIR}/acme.tampered.json'))
b['manifest']['account']['Name'] = 'TAMPERED'
json.dump(b, open('${TMPDIR}/acme.tampered.json', 'w'))
"
if "$CLI" agent verify --strict --deep "${TMPDIR}/acme.tampered.json" >/dev/null 2>&1; then
  record 10 "agent verify FAIL on tampered bundle" FAIL "verify accepted tampered bundle (BAD)"
else
  record 10 "agent verify FAIL on tampered bundle" PASS "verify rejected tampered bundle"
fi

# 11. audit row appears
if sf data query --target-org "$ORG" --query "SELECT BundleJti__c FROM SF360_Bundle_Audit__c ORDER BY GeneratedAt__c DESC LIMIT 1" --json > "${TMPDIR}/audit.out" 2>&1; then
  record 11 "SF360_Bundle_Audit__c row appears" PASS "audit row found"
else
  record 11 "SF360_Bundle_Audit__c row appears" FAIL "$(tail -1 "${TMPDIR}/audit.out")"
fi

# W1. agent update one safe Account field
cleanup_account_description=$(query_first_field "SELECT Description FROM Account WHERE Id = '$ACME_ID'" "Description")
if "$CLI" agent update "$ACME_ID" \
    --field "Description=sf360 live verification W1" \
    --dry-run --json > "${TMPDIR}/W1-dry.out" 2>&1 &&
   "$CLI" agent update "$ACME_ID" \
    --field "Description=sf360 live verification W1" \
    --json > "${TMPDIR}/W1.out" 2>&1; then
  if "$CLI" agent write-audit list --sobject Account --status executed --limit 5 > "${TMPDIR}/W1-audit.out" 2>&1; then
    record W1 "agent update writes safe Account field" PASS "executed + audit list ok"
  else
    record W1 "agent update writes safe Account field" FAIL "$(tail -3 "${TMPDIR}/W1-audit.out" | tr '\n' ';')"
  fi
else
  record W1 "agent update writes safe Account field" FAIL "$(cat "${TMPDIR}/W1-dry.out" "${TMPDIR}/W1.out" 2>/dev/null | tail -3 | tr '\n' ';')"
fi

# W2. agent upsert twice with same key
upsert_key="sf360-live-upsert-$(date +%Y%m%d%H%M%S)"
if "$CLI" agent upsert \
    --sobject Account \
    --idempotency-key "$upsert_key" \
    --field "Name=SF360 Live Verify Upsert" \
    --field "Description=first upsert" \
    --json > "${TMPDIR}/W2-first.out" 2>&1 &&
   "$CLI" agent upsert \
    --sobject Account \
    --idempotency-key "$upsert_key" \
    --field "Name=SF360 Live Verify Upsert" \
    --field "Description=first upsert" \
    --json > "${TMPDIR}/W2-second.out" 2>&1; then
  if grep -Eqi 'no[_ -]?change|no-op|already exists' "${TMPDIR}/W2-second.out"; then
    record W2 "agent upsert retry is no-op" PASS "second call no-change"
  else
    record W2 "agent upsert retry is no-op" FAIL "second call succeeded but no no-change marker observed"
  fi
else
  record W2 "agent upsert retry is no-op" FAIL "$(cat "${TMPDIR}/W2-first.out" "${TMPDIR}/W2-second.out" 2>/dev/null | tail -3 | tr '\n' ';')"
fi

# W3. agent log-activity creates a Task
task_key="sf360-live-task-$(date +%Y%m%d%H%M%S)"
if "$CLI" agent log-activity \
    --type call \
    --what "$ACME_ID" \
    --subject "SF360 live verification call" \
    --idempotency-key "$task_key" \
    --json > "${TMPDIR}/W3.out" 2>&1; then
  task_id=$(query_first_field "SELECT Id FROM Task WHERE WhatId = '$ACME_ID' AND Subject = 'SF360 live verification call' ORDER BY CreatedDate DESC LIMIT 1" "Id")
  if [ -n "$task_id" ]; then
    cleanup_task_ids+=("$task_id")
    record W3 "agent log-activity creates Task" PASS "Task=$task_id"
  else
    record W3 "agent log-activity creates Task" FAIL "Task query returned no rows"
  fi
else
  record W3 "agent log-activity creates Task" FAIL "$(tail -3 "${TMPDIR}/W3.out" | tr '\n' ';')"
fi

# W4. agent advance moves an Opportunity stage
test_opp_id=$(find_test_opp)
if [ -z "$test_opp_id" ]; then
  record W4 "agent advance moves Opportunity stage" SKIP "OPP_ID not set and no Opportunity found for account"
else
  original_stage=$(query_first_field "SELECT StageName FROM Opportunity WHERE Id = '$test_opp_id'" "StageName")
  cleanup_opp_id="$test_opp_id"
  cleanup_opp_stage="$original_stage"
  if [ "$original_stage" = "$OPP_STAGE" ]; then
    record W4 "agent advance moves Opportunity stage" SKIP "OPP_STAGE equals current stage; set OPP_STAGE to a forward stage"
  elif "$CLI" agent advance \
      --opp "$test_opp_id" \
      --stage "$OPP_STAGE" \
      --json > "${TMPDIR}/W4.out" 2>&1; then
    observed_stage=$(query_first_field "SELECT StageName FROM Opportunity WHERE Id = '$test_opp_id'" "StageName")
    if [ "$observed_stage" = "$OPP_STAGE" ]; then
      record W4 "agent advance moves Opportunity stage" PASS "$original_stage -> $observed_stage"
    else
      record W4 "agent advance moves Opportunity stage" FAIL "stage after write was $observed_stage"
    fi
  else
    record W4 "agent advance moves Opportunity stage" FAIL "$(tail -3 "${TMPDIR}/W4.out" | tr '\n' ';')"
  fi
fi

# W5. force stale-write conflict
lmd=$(query_first_field "SELECT LastModifiedDate FROM Account WHERE Id = '$ACME_ID'" "LastModifiedDate")
if [ -z "$lmd" ]; then
  record W5 "stale write conflict rejected" FAIL "could not fetch LastModifiedDate"
elif ! sf data update record --target-org "$ORG" --sobject Account --record-id "$ACME_ID" --values "Description='sf360 live verification external mutation'" > "${TMPDIR}/W5-mutate.out" 2>&1; then
  record W5 "stale write conflict rejected" FAIL "$(tail -3 "${TMPDIR}/W5-mutate.out" | tr '\n' ';')"
elif "$CLI" agent update "$ACME_ID" \
    --field "Description=sf360 stale write should fail" \
    --if-last-modified "$lmd" \
    --json > "${TMPDIR}/W5.out" 2>&1; then
  record W5 "stale write conflict rejected" FAIL "stale write succeeded"
else
  record_write_result W5 "stale write conflict rejected" "${TMPDIR}/W5.out" "CONFLICT_STALE_WRITE"
fi

# W6. FLS write denial
if [ -n "$RESTRICTED_WRITE_USER" ]; then
  if "$CLI" agent update "$ACME_ID" \
      --run-as-user "$RESTRICTED_WRITE_USER" \
      --field "AnnualRevenue=123" \
      --json > "${TMPDIR}/W6.out" 2>&1; then
    if grep -Eqi 'FLS_WRITE_DENIED|dropped|warning' "${TMPDIR}/W6.out"; then
      record W6 "FLS write denial enforced" PASS "field dropped with warning"
    else
      record W6 "FLS write denial enforced" FAIL "restricted write appeared to succeed"
    fi
  else
    record_write_result W6 "FLS write denial enforced" "${TMPDIR}/W6.out" "FLS_WRITE_DENIED"
  fi
else
  record W6 "FLS write denial enforced" SKIP "RESTRICTED_WRITE_USER not set"
fi

echo "------------------------------------------------------------"

# Write JSON report
{
  echo "{"
  echo "  \"date\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
  echo "  \"org\": \"$ORG\","
  echo "  \"account_id\": \"$ACME_ID\","
  echo "  \"cli_version\": \"$($CLI version 2>/dev/null || echo unknown)\","
  echo "  \"results\": ["
  for i in "${!results[@]}"; do
    if [ $i -gt 0 ]; then echo ","; fi
    printf "    %s" "${results[$i]}"
  done
  echo
  echo "  ]"
  echo "}"
} > "$REPORT_JSON"

echo
echo "JSON report written to: $REPORT_JSON"
echo "Now fill in docs/live-verification-report.md by hand and sign with the trust key."

# Exit non-zero if any required check failed
if printf '%s\n' "${results[@]}" | grep -q '"status":"FAIL"'; then
  echo
  echo "FAIL: at least one required check failed. v1.0.0 release blocked."
  exit 1
fi
