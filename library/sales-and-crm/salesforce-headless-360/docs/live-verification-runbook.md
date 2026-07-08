# Live-org verification runbook

This runbook is the bridge between code-complete and Benioff outreach. Every required PASS row must be green before a v1.1.0 release tag goes out.

Pair on this for 30-60 minutes with someone who has admin access to a Salesforce Developer Edition (or Enterprise/Unlimited) org.

## Prereqs

Tester needs:
- Salesforce org with admin profile (Developer Edition is fine; sandbox of Enterprise/Unlimited preferred for FLS coverage)
- `sf` CLI version >= 2.60.0 installed and authed: `sf org login web --alias sf360-test`
- `salesforce-headless-360-pp-cli` installed: `go install github.com/mvanhorn/salesforce-headless-360-pp-cli/...@latest`
- A second Salesforce User on the test org with a restricted profile (no read on `Account.AnnualRevenue` and no read on a custom `Salary__c` field on Contact). This proves FLS intersection works.
- A writable test Account (`ACME_ID`) with a safe field such as `Description`, at least one related Opportunity for stage checks, and permission to create/delete Tasks.
- A restricted write-profile user for W6 with no write access to a selected field such as `Account.AnnualRevenue`.
- Optional: Slack workspace with a test channel containing 2-3 members whose emails match SF Users in the test org. Skip if not available.

## How to use this runbook

For each numbered check:
1. Run the exact command shown.
2. Compare the observed output to the "expected" line.
3. Mark PASS or FAIL in `docs/live-verification-report.md`.
4. If FAIL: file an issue, fix, re-run. No skipping FAIL → PASS by handwaving.
5. If SKIP (because a feature is not provisioned in this org): record explicit reason.

The runbook is keyed to `docs/README-claim-map.md`. Every load-bearing claim has a corresponding check here.

---

## Required checks (block release if any FAIL)

### 1. sf CLI fall-through

```bash
salesforce-headless-360-pp-cli auth login --sf sf360-test
salesforce-headless-360-pp-cli auth list-orgs
```

Expected:
- `auth login --sf` returns success without prompting for a password.
- `list-orgs` shows `sf360-test` with `authMethod: sf_fallthrough` and a non-empty `instance_url`.

### 2. doctor full pass

```bash
salesforce-headless-360-pp-cli --org sf360-test doctor
```

Expected:
- Exit code 0.
- REST row green.
- Trust key store row green.
- Local store row green.
- sf CLI passthrough green with version >= 2.60.0.

### 3. Composite Graph in sync

```bash
salesforce-headless-360-pp-cli --org sf360-test sync --account <ACME_ID> --verbose
```

Expected:
- Verbose log shows exactly one POST to `/services/data/v63.0/composite/graph`.
- Pagination fallback only triggers if a child relationship has > 2000 rows.
- SQLite `data.db` populated across all Customer 360 tables.
- No 401, no Sforce-Limit-Info above 80%.

### 4. UI API sharing cross-check

```bash
sqlite3 ~/.local/share/salesforce-headless-360-pp-cli/data.db \
  "SELECT count(*) FROM sharing_drop_audit WHERE account_id = '<ACME_ID>';"
```

Expected:
- Count is 0 (admin profile sees everything) OR > 0 with a clear sobject + reason for each row if a restricted profile syncs the same account.

### 5. FLS intersection actually hides a field

Switch to the restricted-profile user (or use `--run-as-user <restricted_user_id>` if testing via JWT):

```bash
salesforce-headless-360-pp-cli --org sf360-test agent context --live <ACME_ID> --output /tmp/acme-restricted.json
grep -c 'AnnualRevenue\|Salary__c' /tmp/acme-restricted.json
```

Expected:
- grep returns 0 (zero matches). The restricted fields are not present in the bundle.
- Bundle provenance includes `"FLS": <count>` with count > 0.

### 6. Tooling compliance map loads

```bash
salesforce-headless-360-pp-cli --org sf360-test sync --account <ACME_ID>
sqlite3 ~/.local/share/salesforce-headless-360-pp-cli/data.db \
  "SELECT count(*) FROM compliance_field_map;"
```

Expected:
- Count > 0. If your org has no fields tagged with ComplianceGroup, tag at least one Contact field as PII before this check.

### 7. trust register writes a Certificate or CMDT record

```bash
salesforce-headless-360-pp-cli --org sf360-test trust register
```

Expected:
- Output reports either `path: certificate` with a Certificate Id, OR `path: cmdt` with a CMDT record Id and a non-empty receipt hash.
- If `path: cmdt`, verify in Setup → Custom Metadata Types → SF360_Bundle_Key__mdt that the row exists with a non-empty `Receipt__c`.

### 8. agent context produces a bundle

```bash
salesforce-headless-360-pp-cli --org sf360-test agent context --live <ACME_ID> --output /tmp/acme.bundle.json
jq '.signature.kid, .provenance.sources_used' /tmp/acme.bundle.json
```

Expected:
- File written.
- `signature.kid` matches the kid from check 7.
- `sources_used` includes at least `rest`.

### 9. agent verify --strict --deep PASS on valid bundle

```bash
salesforce-headless-360-pp-cli agent verify --strict --deep /tmp/acme.bundle.json
```

Expected:
- Exit code 0.
- Output includes `signature: ok`, `exp: ok`, `file_bytes: ok` (for any ContentDocumentLinks).

### 10. agent verify --strict --deep FAIL on tampered bundle

```bash
cp /tmp/acme.bundle.json /tmp/acme.bundle.tampered.json
# Flip one byte in the manifest (use a hex editor or Python):
python3 -c "
import json
b = json.load(open('/tmp/acme.bundle.tampered.json'))
b['manifest']['account']['Name'] = 'TAMPERED'
json.dump(b, open('/tmp/acme.bundle.tampered.json', 'w'))
"
salesforce-headless-360-pp-cli agent verify --strict --deep /tmp/acme.bundle.tampered.json
```

Expected:
- Exit code non-zero.
- Error code `SIGNATURE_INVALID`.

### 11. SF360_Bundle_Audit__c row appears

```bash
sf data query --query "SELECT Id, BundleJti__c, AccountId__c, GeneratedAt__c FROM SF360_Bundle_Audit__c ORDER BY GeneratedAt__c DESC LIMIT 1" --target-org sf360-test
```

Expected:
- One row, `BundleJti__c` matches the `jti` claim in the JWS from check 8 (decode the JWS payload to confirm).

---

## Write path checks (block v1.1.0 release if any FAIL)

### W1. agent update writes one safe Account field

```bash
salesforce-headless-360-pp-cli --org sf360-test agent update <ACME_ID> \
  --field Description="sf360 live verification W1" \
  --dry-run --json
salesforce-headless-360-pp-cli --org sf360-test agent update <ACME_ID> \
  --field Description="sf360 live verification W1" \
  --json
salesforce-headless-360-pp-cli agent write-audit list --sobject Account --status executed --limit 5
```

Expected:
- Dry-run reports `dry_run: true` and does not create a write-audit row.
- Real run returns an executed write envelope.
- `write-audit list` shows an executed Account update with a non-empty `jti` and acting `kid`.

### W2. agent upsert is retry-safe with the same key

```bash
KEY="sf360-live-upsert-$(date +%Y%m%d%H%M%S)"
salesforce-headless-360-pp-cli --org sf360-test agent upsert \
  --sobject Account \
  --idempotency-key "$KEY" \
  --field Name="SF360 Live Verify Upsert" \
  --field Description="first upsert" \
  --json
salesforce-headless-360-pp-cli --org sf360-test agent upsert \
  --sobject Account \
  --idempotency-key "$KEY" \
  --field Name="SF360 Live Verify Upsert" \
  --field Description="first upsert" \
  --json
salesforce-headless-360-pp-cli agent write-audit list --sobject Account --limit 10
```

Expected:
- First call creates or updates the record.
- Second call returns `NoChange=true` or equivalent no-op/no-change provenance.
- Write audit contains two attempts tied to the same idempotency key.

### W3. agent log-activity creates a Task

```bash
KEY="sf360-live-task-$(date +%Y%m%d%H%M%S)"
salesforce-headless-360-pp-cli --org sf360-test agent log-activity \
  --type call \
  --what <ACME_ID> \
  --subject "SF360 live verification call" \
  --idempotency-key "$KEY" \
  --json
sf data query --target-org sf360-test \
  --query "SELECT Id, Subject, WhatId FROM Task WHERE WhatId = '<ACME_ID>' AND Subject = 'SF360 live verification call' ORDER BY CreatedDate DESC LIMIT 1"
```

Expected:
- CLI returns executed.
- Salesforce query returns one Task.
- Cleanup deletes the Task after the run.

### W4. agent advance moves an Opportunity stage

```bash
OPP_ID="<OPP_ID>"
ORIGINAL_STAGE="$(sf data query --target-org sf360-test --json --query "SELECT StageName FROM Opportunity WHERE Id = '$OPP_ID'" | jq -r '.result.records[0].StageName')"
salesforce-headless-360-pp-cli --org sf360-test agent advance \
  --opp "$OPP_ID" \
  --stage "Proposal/Price Quote" \
  --json
sf data query --target-org sf360-test \
  --query "SELECT Id, StageName FROM Opportunity WHERE Id = '$OPP_ID'"
```

Expected:
- CLI returns executed.
- Opportunity `StageName` is the requested target stage.
- Cleanup advances or updates the Opportunity back to `ORIGINAL_STAGE`.

### W5. stale write conflict is rejected

```bash
LMD="$(sf data query --target-org sf360-test --json --query "SELECT LastModifiedDate FROM Account WHERE Id = '<ACME_ID>'" | jq -r '.result.records[0].LastModifiedDate')"
sf data update record --target-org sf360-test --sobject Account --record-id <ACME_ID> --values "Description='sf360 live verification external mutation'"
salesforce-headless-360-pp-cli --org sf360-test agent update <ACME_ID> \
  --field Description="sf360 stale write should fail" \
  --if-last-modified "$LMD" \
  --json
```

Expected:
- Final command exits non-zero.
- Error envelope code is `CONFLICT_STALE_WRITE`.
- Write audit marks the attempt as `conflict` or rejected with the conflict code.

### W6. FLS write denial is enforced

Switch to the restricted-profile user, or run JWT with `--run-as-user <restricted_user_id>`:

```bash
salesforce-headless-360-pp-cli --org sf360-test agent update <ACME_ID> \
  --run-as-user <RESTRICTED_USER_ID> \
  --field AnnualRevenue=123 \
  --json
```

Expected:
- Command exits non-zero with `FLS_WRITE_DENIED`, OR the field is silently dropped with a warning and no persisted field mutation.
- Write audit shows a rejected attempt or a warning in provenance.

## Write cleanup

After W1-W6:

1. Restore the Account `Description` to its original value if the test org needs a clean fixture.
2. Delete Tasks created by W3:

```bash
sf data query --target-org sf360-test --json \
  --query "SELECT Id FROM Task WHERE WhatId = '<ACME_ID>' AND Subject = 'SF360 live verification call'" \
  | jq -r '.result.records[].Id' \
  | xargs -I{} sf data delete record --target-org sf360-test --sobject Task --record-id {}
```

3. Restore the W4 Opportunity stage to `ORIGINAL_STAGE`.
4. Delete or mark the W2 upsert fixture Account if your org does not keep test data.

---

## Optional checks (skip allowed with recorded reason)

### O1. Apex companion deploy

```bash
salesforce-headless-360-pp-cli --org sf360-test trust install-apex
sf apex run test --target-org sf360-test --class SF360SafeRead_Test --wait 10
sf apex run test --target-org sf360-test --class SF360SafeWrite_Test --wait 10
sf apex run test --target-org sf360-test --class SF360SafeUpsert_Test --wait 10
```

Expected: deploy succeeds, `SF360SafeRead`, `SF360SafeWrite`, and `SF360SafeUpsert` are installed, and all Apex tests pass.

Skip reasons: org restricts deploy; non-admin profile; Apex tests blocked by org policy.

### O2. Bulk fallback path

```bash
# On an account with > 10,000 Tasks:
salesforce-headless-360-pp-cli --org sf360-test sync --account <BIG_ACCOUNT_ID> --allow-bulk-fls-unsafe
```

Expected: warning printed about FLS-unsafe Bulk path; sync completes.

Skip reasons: no account exceeds the threshold in the test org.

### O3. Data Cloud profile

```bash
salesforce-headless-360-pp-cli --org sf360-test agent context --live <ACME_ID> --output /tmp/dc.bundle.json
jq '.provenance.sources_used' /tmp/dc.bundle.json
```

Expected: `sources_used` includes `data_cloud`.

Skip reasons: org not provisioned for Data Cloud (most common).

### O4. Slack linkage

```bash
sqlite3 ~/.local/share/salesforce-headless-360-pp-cli/data.db \
  "SELECT count(*) FROM slack_relations WHERE entity_id = '<ACME_ID>';"
```

Expected: count > 0 if Slack Sales Elevate is installed and a channel is linked to Acme.

Skip reasons: workspace lacks Slack Sales Elevate.

### O5. Slack inject end-to-end

```bash
salesforce-headless-360-pp-cli --org sf360-test agent inject \
  --slack '#test-channel' \
  --bundle /tmp/acme.bundle.json
```

Expected:
- Channel members enumerated.
- Audience FLS intersection computed.
- Markdown summary posted via chat.postMessage (or aborted with clear message if external members and `--allow-external-channel-members` not passed).

Skip reasons: no test Slack workspace; no SLACK_BOT_TOKEN.

---

## Wrap-up

1. Fill out `docs/live-verification-report.md` with PASS/FAIL/SKIP and observed output for every row.
2. Sign the report with Matt's key: `agent verify --deep` over the report file embedded in a small bundle.
3. Commit the report to the repo.
4. Tag v1.1.0.
5. Send to Benioff.

If any required PASS is FAIL: open an issue, fix, re-run the failing check (and any downstream checks). No FAILs at v1.1.0.
