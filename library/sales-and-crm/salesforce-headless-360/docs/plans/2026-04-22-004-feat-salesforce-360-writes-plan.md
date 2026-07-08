---
title: "salesforce-headless-360-pp-cli v1.1 — live-org verification pass"
type: feat
status: ready-for-friend
date: 2026-04-22
---

# Run this end-to-end against your real Salesforce org

Matt here. I built a Salesforce Headless 360 CLI — v1 packages Customer 360 into JWS-signed JSON bundles for any agent to consume; v1.1 lets agents WRITE back (update/upsert/create/log-activity/advance/close-case/note + plan mode for multi-agent workflows). Every action signed, audited, FLS-safe. Same trust substrate either way.

You've already got Printing Press + a real Salesforce org. **This plan is what I need you to run** — about 30-60 minutes against your real instance. Output: a signed verification report committed to PR #111 that unblocks v1.1.0 and Benioff outreach.

PR: https://github.com/mvanhorn/printing-press-library/pull/111

## Read this once before starting

The CLI is mostly read-only on your data. Writes happen ONLY when you explicitly run a write verb. The verification script (`scripts/live-verify.sh`) makes exactly these mutations and reverts every one of them via a cleanup trap:

- **W1**: updates one Account `Description` field, then restores the original
- **W2**: creates a single Account via upsert (named "SF360 Live Verify Upsert"); idempotency makes the retry a no-op
- **W3**: creates a Task linked to the test Account, then deletes it
- **W4**: advances the test Opportunity stage one step, then reverts it
- **W5/W6**: read-only conflict + FLS denial probes — no mutations

Plus 1 metadata write: `trust register` creates one Certificate (or one CMDT row) with my public key. Revocable with one command after.
Plus N audit rows in `SF360_Bundle_Audit__c` and `SF360_Write_Audit__c`. Append-only, that's the GDPR Article 30 record-of-processing — exactly what you want.

If anything looks off mid-run, stop and text me. We'll debug together.

---

## Step 0: Get the code

```bash
cd ~/code  # wherever you keep things
gh repo clone mvanhorn/printing-press-library sf360-verify
cd sf360-verify
git checkout feat/salesforce-headless-360
cd library/sales-and-crm/salesforce-headless-360
```

Install the CLI from this branch:

```bash
go install ./cmd/salesforce-headless-360-pp-cli
go install ./cmd/salesforce-headless-360-pp-mcp
which salesforce-headless-360-pp-cli
# → should print a path under $HOME/go/bin
```

If not on PATH: `export PATH=$HOME/go/bin:$PATH` and add to `~/.zshrc` or `~/.bashrc`.

---

## Step 1: Wire up your org

### Auth your org to `sf` (skip if already aliased)

```bash
sf org login web --alias sf360-test
sf org display --target-org sf360-test --json | jq '.result.instanceUrl'
```

If you already have an alias, just use that name everywhere I write `sf360-test`.

### Pick the test fixtures from your org

You need:
- An **Account** with rich data (5+ Contacts, 2+ Opps, 1+ Case, some Activities). Pick one you don't mind the script briefly editing the Description field on. The CLI restores the original.
- An **Opportunity** on that Account (the script briefly advances its stage one step then reverts).
- A **restricted-profile User Id** (any non-System-Admin user) for FLS checks.

```bash
# Account
sf data query --target-org sf360-test --query "SELECT Id, Name, Description FROM Account WHERE Name = 'YOUR_TEST_ACCOUNT_NAME'"

# Opportunity on that Account
sf data query --target-org sf360-test --query "SELECT Id, Name, StageName FROM Opportunity WHERE AccountId = 'ACME_ID' ORDER BY LastModifiedDate DESC LIMIT 5"

# Restricted user
sf data query --target-org sf360-test --query "SELECT Id, Username, Profile.Name FROM User WHERE IsActive = true AND Profile.Name != 'System Administrator' LIMIT 10"
```

Export the IDs — the script reads them from env:

```bash
export ORG=sf360-test
export ACME_ID=<18-char Account Id>
export OPP_ID=<18-char Opportunity Id>           # used for W4 advance
export OPP_STAGE='Proposal/Price Quote'          # any valid stage other than the current one; default works for most orgs
export RESTRICTED_USER=<User Id>                 # for read-side FLS check
export RESTRICTED_WRITE_USER=$RESTRICTED_USER    # same user for write-side FLS denial check
```

(If your org has Compliance Categorization on at least a few fields like `Email`/`Phone`, Check 6 will pass automatically. If not, tag one Contact field as `PII` in Setup. 30 seconds.)

---

## Step 2: Run the automated verification

```bash
cd ~/code/sf360-verify/library/sales-and-crm/salesforce-headless-360

echo "ORG=$ORG"
echo "ACME_ID=$ACME_ID"
echo "OPP_ID=$OPP_ID"
echo "RESTRICTED_USER=$RESTRICTED_USER"

bash scripts/live-verify.sh
```

**Runtime:** 3-7 min. **API cost:** ~30-50 calls + 1 Composite Graph + ~5 Tooling + 1-2 audit writes per write verb. Rate-limit safe. The script never uses `--full` — only `--account <id>`.

**The cleanup trap fires on script exit (success or failure)** and:
- Deletes any Tasks created by W3
- Reverts the Opportunity stage advanced by W4
- Restores the Account Description changed by W1

Output: PASS/FAIL/SKIP per check + `docs/live-verification-report.json` written.

**Output:** PASS/FAIL/SKIP per check + `docs/live-verification-report.json` written.

Go to Step 3 next. Running live against a real org ALWAYS surfaces something — even on full PASS there'll be observations, edge cases, or papercuts worth capturing. Step 3 is where we turn those into a fix plan.

---

## Step 3: Turn findings into a fix plan (always — FAIL or not)

Running against your real data WILL surface things: FAILs, quirky mock-vs-reality drift, rough edges, a D9 error message that's cryptic, a flag name that's wrong, a runbook step that's unclear, Apex behavior you didn't expect, anything.

**Don't text me that list. Plan it, work it, push it.** You have Compound Engineering and you're closer to the bug than I am.

### 3a. Gather findings

Skim the script output + your notes. Write a short findings list (bullets are fine). Include:
- Any `FAIL` checks with their log tail (grab from the `${TMPDIR}/*.out` files or re-run with `--verbose`)
- Any `SKIP` that the script marked but you wanted to PASS (e.g., you expected Data Cloud but got `[rest]`)
- Anything that passed but looked weird (wrong stage name in W4, verbose output noisy, an error message that wouldn't help a real user)
- Anything the runbook said would work but your org rejected (validation rules, permission denials, edition differences)

Capture each failing command's output to a file so the plan has real evidence:

```bash
# Example — capture whatever failed
mkdir -p /tmp/sf360-findings
salesforce-headless-360-pp-cli --org $ORG <the command that failed> --verbose > /tmp/sf360-findings/<check-id>.log 2>&1
```

### 3b. Run `/ce:plan` with the findings

From the scaffold directory:

```bash
cd ~/code/sf360-verify/library/sales-and-crm/salesforce-headless-360
```

Invoke ce:plan. Feed it the failure evidence + org context + origin plan reference:

```
/ce:plan fix live-org verification findings on salesforce-headless-360 v1.1

Origin plan: docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md

Org context:
- Org type: <production / full sandbox / partial sandbox / DE>
- Edition: <Enterprise / Unlimited / Professional / DE>
- Anything specific: <custom validation rules, locked-down profiles, Tooling API restrictions, PE Certificate unavailable, etc.>

Findings from today's verify run (/tmp/sf360-findings/*.log):

FAILs:
- W<N> <check name>: <one-line summary> — evidence: /tmp/sf360-findings/W<N>.log
- (repeat per FAIL)

SKIPs that should have passed:
- O<N> <check name>: expected X, got Y because <reason>

Observations worth fixing:
- <whatever else you noticed>

Constraints:
- Fix in-place in the scaffold
- Don't break the 11 passing read checks or the 6 W-checks that passed
- Maintain scorecard >= 90 grade A
- Every fix unit adds a regression test in the relevant package
- Don't expand scope beyond what the findings imply
```

This produces `docs/plans/<YYYY-MM-DD>-<NNN>-fix-sf360-<name>-plan.md`.

### 3c. Run `/ce:work` against the new plan

```
/ce:work docs/plans/<the-new-plan-file>
```

This dispatches codex through each implementation unit, commits each one with conventional messages, runs the full test suite after each change. Expect ~10-30 min per unit depending on complexity.

When work finishes and all tests green:

```bash
# Re-run verify to confirm the fixes actually fix it
bash scripts/live-verify.sh

# Push the fix plan + fix commits + updated report
git push origin feat/salesforce-headless-360
```

If the fix surfaces new findings (it sometimes does — one bug hiding behind another), loop: new `/ce:plan` → new `/ce:work` → re-verify → push. Each loop is a couple commits on the PR branch.

### 3d. If you hit a wall — hand back

If something is genuinely stuck after a plan + work pass (bug deeper than expected, needs design decisions, requires org access I don't have), stop and ping me. But try the plan-work loop at least once first — it solves more than you'd think.

---

## Step 4: Optional checks (run what applies to your org)

### O1. Apex companion deploy — please run if you can

```bash
salesforce-headless-360-pp-cli --org $ORG trust install-apex
sf apex run test --target-org $ORG --class SF360SafeRead_Test --wait 10
sf apex run test --target-org $ORG --class SF360SafeWrite_Test --wait 10
sf apex run test --target-org $ORG --class SF360SafeUpsert_Test --wait 10
```

`SF360SafeRead.cls`, `SF360SafeWrite.cls`, `SF360SafeUpsert.cls` live at `apex/force-app/main/default/classes/`. Read them first if you want — each ~60-100 lines, wraps SOQL/DML with `WITH USER_MODE` and `Database.update(..., AccessLevel.USER_MODE)`. Required for the JWT + Bulk FLS-safe paths to work; UI API path works without them.

### O3. Data Cloud profile

```bash
salesforce-headless-360-pp-cli --org $ORG agent context --live $ACME_ID --output /tmp/dc.bundle.json
jq '.provenance.sources_used' /tmp/dc.bundle.json
```

If your org has Data Cloud provisioned, `sources_used` includes `data_cloud`. If not, `[rest]` alone — both PASS (the graceful-degradation story works either way).

### O4. Slack linkage

```bash
sqlite3 ~/.local/share/salesforce-headless-360-pp-cli/sf360.db "SELECT count(*) FROM slack_relations WHERE entity_id = '$ACME_ID';"
```

> 0 if Slack Sales Elevate is installed and the account is linked. Otherwise SKIP.

### O5. Slack inject (skip unless you have a test channel)

```bash
export SLACK_BOT_TOKEN=<your bot token>
salesforce-headless-360-pp-cli --org $ORG agent inject --slack '#your-test-channel' --bundle /tmp/dc.bundle.json
```

Use a dev channel. Don't post to anything customer-facing.

### O2. Bulk fallback — almost always SKIP

Skip unless you happen to have an Account with > 10k Tasks. Rare.

---

## Step 5: Fill in the report

Open `docs/live-verification-report.md`. For every numbered + W-numbered row:
- Status: `PASS` / `FAIL` / `SKIP`
- Observed: one line of evidence (real output, not handwave)
- Notes: anything unexpected

Example row:

```markdown
| W1 | agent update writes one safe Account field | PASS | Description updated then restored; audit row jti=01J... | |
| W4 | agent advance moves Opportunity stage | PASS | Stage Prospecting → Proposal/Price Quote → reverted | |
| O1 | Apex companion deploy | PASS | All three Apex tests passed | |
| O3 | Data Cloud profile | SKIP | Org not provisioned for Data Cloud | |
```

Don't handwave. Real observed output or SKIP with reason.

---

## Step 6: Sign the report

This proves the report came from you + your org + this CLI version. It's the attestation that unblocks the v1.1.0 tag.

```bash
# trust register may already be done by Check 7 in the script, but rerun is idempotent
salesforce-headless-360-pp-cli --org $ORG trust register

# Produce a signed bundle over your test account (proves Check 8 + gives you a kid + jws to paste)
salesforce-headless-360-pp-cli --org $ORG agent context --live $ACME_ID --output /tmp/acme.bundle.json

# Grab the kid + signature
jq -r '.signature.kid' /tmp/acme.bundle.json
jq -r '.signature.jws' /tmp/acme.bundle.json
```

Paste into the Sign-off block at the bottom of `docs/live-verification-report.md`:

```
Tester signature: <your name>
Date:             <YYYY-MM-DD>
Org ID:           <from `sf org display --target-org $ORG --json | jq -r .result.id`>
Org Type:         Production / Full Sandbox / Partial Sandbox / Developer Edition
Kid:              <from jq above>
JWS (proof of key possession): <the JWS from /tmp/acme.bundle.json>
```

---

## Step 7: Commit and push the report

```bash
cd ~/code/sf360-verify
git config user.email "<your email>"
git config user.name "<your name>"

git add library/sales-and-crm/salesforce-headless-360/docs/live-verification-report.md \
        library/sales-and-crm/salesforce-headless-360/docs/live-verification-report.json

git commit -m "verify(salesforce-headless-360): live-org v1.1 verification pass

Verified against <production / sandbox> on <date>.
Required reads: PASS on all 11.
Required writes: PASS on all 6 (W1-W6).
Optional: <list PASS / SKIP with reasons>.
Signed by <your kid>.

Ready for merge + v1.1.0 tag."

git push origin feat/salesforce-headless-360
```

Then comment on the PR (https://github.com/mvanhorn/printing-press-library/pull/111):

> Verification pass complete against <prod / sandbox>. All 11 required read + 6 required write checks PASS. Optional: [list]. Report signed and committed. Ready for review.

That's it. I take it from there: mark the PR Ready for Review, merge, tag v1.1.0, send to Benioff.

---

## Cleanup

Already automated by the script's exit trap (Tasks deleted, Opp stage reverted, Account Description restored). The only persistent state in your org from this run:

- 1 trust key record (Certificate or CMDT) — list with `salesforce-headless-360-pp-cli --org $ORG trust list-keys`; revoke with `trust revoke-key --kid <kid> --reason "verification complete"`
- 1 Account record from W2 upsert ("SF360 Live Verify Upsert") — delete in Setup or via `sf data delete record --target-org $ORG --sobject Account --record-id <id>`
- N audit rows in `SF360_Bundle_Audit__c` + `SF360_Write_Audit__c` — fine to leave (audit trail) or mass-delete via Setup → Data → Mass Delete
- (If you ran O1) 3 Apex classes — `sf project delete source --metadata ApexClass:SF360SafeRead --metadata ApexClass:SF360SafeWrite --metadata ApexClass:SF360SafeUpsert --target-org $ORG`

---

## What I actually need from you

**Do**:
- Run the runbook honestly. If anything FAILs, tell me. Catching a real bug at this stage is the whole point.
- Record observed output in the report — not "looked good"
- Sign the attestation with a key registered in your own org
- Check 9, Check 10, W5, W6 are the security-critical ones — please don't skip those

**Don't**:
- Don't merge the PR yourself. Leave it Ready for Review so I can do the final pass and tag.
- Don't run the script against your most-critical Account. Pick something rich-but-low-stakes.
- Don't skip the cleanup verification — confirm Tasks are gone, Opp stage is back, Description is restored after the script exits. The trap should handle it but eyeball the org once.

---

## Confused or stuck? Ping me

Anything weird, blocked, "this looks sketchy", whatever — text me. The whole thing is built on the idea that the CLI should actually work, not just claim to. You catching anything is a huge favor.

Thanks 🙏

— Matt

---

## Quick reference

- **Runbook**: `docs/live-verification-runbook.md` (full check detail with expected output)
- **Report template**: `docs/live-verification-report.md` (you fill this in)
- **Automation**: `scripts/live-verify.sh` (drives the 11 + 6 checks with cleanup trap)
- **PR**: https://github.com/mvanhorn/printing-press-library/pull/111
- **Claim map**: `docs/README-claim-map.md` (every README claim → file path)
- **Security model**: `docs/security.md` (trust + threat model + write-path security)
- **HIPAA posture**: `docs/hipaa.md`
