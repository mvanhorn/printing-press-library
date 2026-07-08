---
title: Fix SF360 Live-Verify Findings (v1.1.0 Readiness)
type: fix
status: active
date: 2026-04-24
origin: /tmp/sf360-findings/README.md
companion: 2026-04-24-002-feat-printing-press-machine-upgrades-plan.md
---

# Fix SF360 Live-Verify Findings

Live verification of `salesforce-headless-360-pp-cli` v1.1 against NFC's `TrentDev1` Developer sandbox surfaced **20 concrete findings** across metadata deployment, CLI flag schema, documentation accuracy, and test automation. This plan resolves the **local-to-this-CLI** subset, drives a passing end-to-end live-verify run plus signed report, and unblocks the `v1.1.0` tag and Benioff outreach.

Systemic issues that would affect **any** CLI produced by the CLI Printing Press generator are documented separately in [`2026-04-24-002-feat-printing-press-machine-upgrades-plan.md`](2026-04-24-002-feat-printing-press-machine-upgrades-plan.md) — those are **advisory**, not to be implemented in this repo.

## Problem Statement

The current plan runbook (`docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md`) instructs a tester to clone the branch, deploy metadata, pick fixtures, and run `scripts/live-verify.sh`. From a fresh clone against a fresh sandbox, **every step fails**:

1. **Metadata deploy fails** — 8 distinct XML/layout bugs prevent `sf project deploy start --source-dir metadata` from reaching even 50% completion (F-007 through F-014).
2. **Seed fixture step has no script** — plan assumes pre-existing "rich-data Account"; Developer sandboxes start empty and real prod-derived orgs have custom validation rules that break naive Apex inserts (F-015).
3. **Verify script crashes immediately** — every `$CLI --org $ORG <verb>` invocation errors with `unknown flag: --org`. The CLI binary has **no global `--org` flag**. Only `trust register` has a per-command `--org` (which is required) (F-017, F-020).
4. **Cleanup trap crashes during crash** — `set -u` + `${cleanup_task_ids[@]}` on an empty array (F-018).
5. **README + SKILL.md examples don't work** — every documented command that shows `salesforce-headless-360-pp-cli --org prod <verb> …` fails. That's 10+ commands in README, all of SKILL.md's "Killer Commands," and every `agent *` example.

Tester (Trent Matthias, NFC) brute-forced through all of this against a real sandbox — workarounds are applied in the working tree and documented in `/tmp/sf360-findings/README.md`.

## Scope Split: LOCAL vs SYSTEMIC

Each finding is categorized as **L** (local to this CLI — fix in this plan) or **S** (systemic to the Printing Press generator — deferred to companion artifact).

| ID | Severity | Summary | Category | Phase |
|----|----------|---------|----------|-------|
| F-001 | doc gap | Plan auth cmd wrong for sandbox/DE | **L+S** | 4 |
| F-002 | doc gap | Sandbox password snapshot gotcha | **L** (doc) | 4 |
| F-003 | blocker | Metadata deploy step missing from plan | **L+S** | 4 |
| F-004 | code bug | `FLS.AllowFieldWrite(user)` discards `user` param | **L** (code) | 1 |
| F-005 | UX | "Log In" shortcut from prod doesn't carry SSO | **L** (doc) | 4 |
| F-006 | UX | NFC-style orgs require Sandbox Access public group | **L** (doc) | 4 |
| F-007 | blocker | Flat `fields/` metadata layout not SFDX-deployable | **L+S** | 7 |
| F-008 | blocker | Task/Event idempotency fields fail (Activity objects) | **L** (metadata design) | 7 |
| F-009 | blocker | CMDT declares invalid `<deploymentStatus>` | **L+S** | 7 |
| F-010 | blocker | CMDT at wrong root-level location | **L+S** | 7 |
| F-011 | blocker | `ActingUser__c` lookup to User rejects `deleteConstraint` | **L** (metadata design) | 7 |
| F-012 | blocker | `ExecutionStatus__c` has invalid top-level `defaultValue` | **L+S** | 7 |
| F-013 | blocker | LongTextArea fields declared `required` | **L+S** | 7 |
| F-014 | blocker | Required-field FLS declarations rejected in PermSet | **L+S** | 7 |
| F-015 | blocker | Plan assumes vanilla org; real orgs break seeding | **L+S** | 5 |
| F-016 | high | W3 Task idempotency broken by F-008 | **L** (scope) | 8 |
| F-017 | CRITICAL | CLI has no `--org` flag; script+docs assume it does | **L+S** | 1, 2, 3, 6 |
| F-018 | medium | Cleanup trap crashes on unbound arrays | **L+S** | 6 |
| F-019 | doc | `sync --verbose` flag doesn't exist | **L+S** | 1, 6 |
| F-020 | CRITICAL | `trust register` requires `--org`, inconsistent with rest of CLI | **L+S** | 1 |

**L+S** items have both a local fix (in this plan) and a systemic generator-level fix (in the companion artifact). The local fix is what ships in this PR; the systemic fix prevents the same class of bug in future Printing Press outputs.

## Proposed Solution

Ten-phase execution. Phases 0-9 strictly sequential to avoid rework (e.g., cannot run live-verify until metadata deploys, cannot fill report until verify passes). Phase 10 (PR) depends on everything.

## Technical Approach

### Phase 0: Commit current working-tree state

The live-verify session already restructured metadata and patched the script. These changes reflect working reality against a real sandbox and should be committed as the baseline.

**Files to commit:**
- `metadata/objects/Account/fields/SF360_Idempotency_Key__c.field-meta.xml` (new, moved from `metadata/fields/`)
- `metadata/objects/Contact/fields/SF360_Idempotency_Key__c.field-meta.xml` (new)
- `metadata/objects/Case/fields/SF360_Idempotency_Key__c.field-meta.xml` (new)
- `metadata/objects/Opportunity/fields/SF360_Idempotency_Key__c.field-meta.xml` (new)
- `metadata/objects/SF360_Bundle_Key__mdt/**` (moved from `metadata/SF360_Bundle_Key__mdt/`)
- `metadata/objects/SF360_Write_Audit__c/fields/ActingUser__c.field-meta.xml` (modified)
- `metadata/objects/SF360_Write_Audit__c/fields/ExecutionStatus__c.field-meta.xml` (modified)
- `metadata/permissionsets/SF360_Key_Registrar.permissionset-meta.xml` (modified)
- `scripts/live-verify.sh` (modified: `--org` stripped, `${arr[@]:-}` fix)
- `sfdx-project.json` (new — required for `sf project deploy start`)

**Commit message:**
```
chore(salesforce-headless-360): checkpoint live-verify workarounds

Captures the metadata restructure and script patches applied during the
live-verify session against NFC TrentDev1 sandbox. Each change corresponds
to one or more findings in /tmp/sf360-findings/README.md (F-007 through F-018).
Subsequent commits in this PR branch turn these workarounds into proper fixes.
```

### Phase 1: CLI Go source fixes

**F-004 — `internal/security/fls.go:136`**: `_ = user` discards the `user` parameter, making `--run-as-user` a no-op in the Go FLS layer.

Decision: **remove the `user` parameter from `AllowFieldWrite` signature entirely.** Downstream callers should use the Apex companion for run-as-user enforcement; the Go layer should not imply it enforces something it doesn't. Update all callers. Add a test verifying the signature change.

If tests expect `AllowFieldWrite(ctx, fieldName, user)`, update them to `(ctx, fieldName)`. Update `docs/security.md` if it references the old signature.

**F-017 / F-020 — `--org` flag schema inconsistency**: The CLI has **no global `--org`**, but `trust register` has **required per-command `--org`**. Every README/SKILL.md example uses `--org prod`.

Decision: **add `--org` as a persistent global flag on root command** (not required, but recognized). Default resolution order:
1. Explicit `--org <alias>` flag (new)
2. `SF_TARGET_ORG` env var
3. `sf config get target-org` (sf CLI default)
4. Error: `authentication required (4)`

This keeps every existing README/SKILL.md example valid AND matches the sf CLI's own ergonomics. `trust register`'s per-command `--org` becomes an accepted override (same resolution).

Implementation: add `--org` to `rootFlags` in `cmd/salesforce-headless-360-pp-cli/root.go`. Thread `flags.Org` into auth resolution. Remove per-command `--org` declaration from `trust register` since it becomes a global.

**F-019 — `sync --verbose` missing**: `scripts/live-verify.sh` line 113 does `$CLI sync --account $ACME_ID --verbose 2>&1 | grep "composite/graph"`. The `--verbose` flag doesn't exist.

Decision: **add `--verbose` as a persistent global flag** that emits request trace lines (URL, method, status, elapsed) to stderr. Wire into the HTTP client. Document in `--help`.

### Phase 2: README.md rewrite

Current README (286 lines) is prose-heavy and uses `--org prod` in ≥10 code blocks that all currently fail. Also structurally diverges from the repo's HubSpot/Linear style.

**Approach:** After Phase 1 lands `--org` as a real global flag, all existing examples become correct. So README keeps its command examples — they just need to be **verified** to run. Structural improvements layer on top:

1. Replace standalone "Killer Commands" section with a **Quick Start** section matching HubSpot's numbered-steps format.
2. Keep "Unique Features," rename "Killer Commands" to "Cookbook" (match HubSpot).
3. Add an **Authentication** matrix covering prod / sandbox / Developer Edition login forms (F-001 evidence):

   | Target | Command |
   |---|---|
   | Production | `sf org login web --alias prod` |
   | Sandbox (generic) | `sf org login web --alias sandbox --instance-url https://test.salesforce.com` |
   | Sandbox (My Domain) | `sf org login web --alias sandbox --instance-url https://<mydomain>--<sandbox>.sandbox.my.salesforce.com` |
   | Developer Edition | `sf org login web --alias de` |

4. Add a **Troubleshooting** section (matching HubSpot's) covering F-002 (sandbox password snapshot), F-005 ("Log In" shortcut SSO gap), F-006 (Sandbox Access public group).
5. Remove the "Where is the live verification report?" FAQ item since the report now exists.
6. Add **Exit Codes** table (HubSpot-style).
7. Validate every command in README runs without "unknown flag" or "unknown command" errors — automated via a Phase 2 smoke-test.

**Smoke test (new file `scripts/smoke-readme-commands.sh`):**
Extract every `salesforce-headless-360-pp-cli ...` code block from README.md, run each with `--mock` (no sandbox needed), assert exit code is not 2 (usage error).

### Phase 3: SKILL.md audit

SKILL.md has the same `--org prod` examples as README. After Phase 1 adds `--org` as a global, examples become valid. Additional work:

1. Verify every example in `## Killer Commands` runs with `--mock`.
2. Tighten "Safety Notes" wording based on what we learned in live verification (e.g., note that `--dry-run` is real on this CLI — confirmed via F-004 investigation).
3. Add a "Known Limitations" block under "Safety Notes":
   - Task/Event writes do not currently persist idempotency keys (F-008, F-016). Agents should treat `log-activity` retries as potentially duplicating Tasks until v1.2.
4. Verify `metadata.openclaw.requires.bins` and install command still resolve on macOS / Linux.

### Phase 4: Update runbook plan doc (`docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md`)

The runbook plan is the canonical "testing instructions Matt hands to a friend." Current gaps (F-001, F-002, F-003, F-005, F-006):

1. **Step 0.5 (new): Deploy the CLI's metadata** — `sf project deploy start --source-dir metadata --target-org <alias>` + `sf org assign permset --name SF360_Key_Registrar --target-org <alias>`. Required before any CLI command runs.
2. **Step 1 updates:**
   - Add the auth matrix from Phase 2.
   - Add seed-helper reference (Phase 5).
3. **New "Troubleshooting" appendix:**
   - F-002: sandbox password reset via prod inbox
   - F-005: "Log In" shortcut doesn't carry SSO on some orgs; use Forgot Password
   - F-006: some orgs require Sandbox Access public group preflight
4. **Update ACME_ID / OPP_ID / restricted-user selection guidance** — mention the seed helper first, then "or use existing fixtures if your org has rich Accounts."

### Phase 5: Seed helpers for real orgs

New files:
- `scripts/seed-minimal.apex` — creates an Account with minimum universal fields (Name, Industry='Other', Description) and 5 Contacts + 2 Opportunities + 1 Case + 1 Task. Accepts overrides via `System.Debug`-captured env-like variables (Apex doesn't have env vars natively; use top-of-file constants with a comment explaining customization).
- `scripts/seed-discover.sh` — queries `sf sobject describe` for Account / Contact / Opportunity / Case / Task. Emits a table of NON-NULLABLE + CREATEABLE + NOT-DEFAULTED fields per sobject. Tester edits `seed-minimal.apex` to satisfy discovered constraints before running.
- `scripts/seed-run.sh` — wrapper: runs discover, prints instructions, waits for edit, runs seed, emits ACCOUNT_ID + OPP_ID environment export lines ready to paste.

Plan's Step 1 references these.

### Phase 6: `scripts/live-verify.sh` hardening

Current working-tree version has Phase 0 workarounds. Proper fixes:

1. **F-018** — `${cleanup_task_ids[@]:-}` pattern already applied in workspace. Also apply to any other `${arr[@]}` in the script (audit all).
2. **F-017 / F-020** — keep `--org "$ORG"` on every CLI invocation after Phase 1 makes it a global flag. Revert the blanket `sed` strip.
3. **F-019** — keep `--verbose` on `sync` after Phase 1 adds it.
4. **New: `sf config set target-org=$ORG` preflight** — redundant once `--org` works, but belt-and-suspenders for the case where a tester runs the script without `sf` auth.
5. **New: strict-mode guards** — start the script with explicit assertions:
   ```bash
   command -v salesforce-headless-360-pp-cli >/dev/null || { echo "CLI not on PATH"; exit 10; }
   command -v sf >/dev/null || { echo "sf CLI not installed"; exit 10; }
   sf org display --target-org "$ORG" --json >/dev/null 2>&1 || { echo "sf alias $ORG not authenticated"; exit 4; }
   ```
6. **New: Skip W3 idempotency assertion when Task.SF360_Idempotency_Key__c isn't deployed** (F-016). Check metadata via `sf sobject describe --sobject Task --json | jq '.result.fields[] | select(.name == "SF360_Idempotency_Key__c")'`. If absent, record W3 as SKIP with reason; test Task creation/deletion separately without idempotency assertion.

### Phase 7: Metadata source-of-truth fixes

Turn the working-tree workarounds into the canonical source:

1. **F-007 — delete `metadata/fields/` directory**. The per-object `objects/<SObject>/fields/` layout is now canonical (SFDX source format, deploys cleanly with `sf project deploy start --source-dir metadata`).
2. **F-008 — remove Task and Event idempotency fields from source** (already deleted in working tree). Document in SKILL.md and README "Known Limitations." This is a LOCAL decision (not systemic) because Activity-object metadata rules are Salesforce-specific and the generator may legitimately want to emit these fields for orgs that support them; the question of when to emit them belongs in the generator's Salesforce emitter, not the Printing Press core.
3. **F-009 — remove `<deploymentStatus>` from SF360_Bundle_Key__mdt.object-meta.xml** (already done in working tree).
4. **F-010 — SF360_Bundle_Key__mdt under `metadata/objects/`** (already done in working tree).
5. **F-011 — ActingUser__c as optional lookup without deleteConstraint** (already done in working tree). Document the design choice in `docs/security.md` (why ActingUser is optional: enforcement at Apex layer, not object-schema layer).
6. **F-012 — remove top-level `<defaultValue>` from ExecutionStatus__c** (already done in working tree). `<default>true</default>` inside the picklist valueSet is sufficient.
7. **F-013 — remove `<required>true</required>` from LongTextArea fields in SF360_Bundle_Key__mdt** (already done in working tree). If these values are semantically required, enforce in Apex or application layer.
8. **F-014 — strip FLS declarations for required fields from permissionset** (already done in working tree). Salesforce auto-grants FLS on required fields; explicit declaration is rejected.
9. **Update `metadata/README.md`** — rewrite to describe the new per-object layout as canonical (not optional). Remove the "Admins who prefer SFDX object folders can move each file" paragraph — that layout IS the required layout.
10. **Add `sfdx-project.json`** at CLI root (already done in working tree). `packageDirectories: [{ path: "metadata", default: true }]`. Note: `sourceApiVersion` should be `66.0` to match target sandbox default API version.

### Phase 8: Green-light live-verify end-to-end

After Phase 7, from a **fresh checkout of this branch plus Phase 0-7 commits**, execute:

```bash
# 1. Build CLI
cd library/sales-and-crm/salesforce-headless-360
go install ./cmd/salesforce-headless-360-pp-cli
go install ./cmd/salesforce-headless-360-pp-mcp

# 2. Auth
sf org login web --alias sf360-test --instance-url <sandbox-url>
sf config set target-org=sf360-test

# 3. Deploy metadata
sf project deploy start --source-dir metadata --target-org sf360-test
sf org assign permset --name SF360_Key_Registrar --target-org sf360-test

# 4. Seed (pick ACME_ID + OPP_ID from output)
bash scripts/seed-run.sh

# 5. Export env vars (output from seed)
export ORG=sf360-test
export ACME_ID=<from seed>
export OPP_ID=<from seed>
export OPP_STAGE='Qualifying'  # or any forward stage on the test Opp
export RESTRICTED_USER=<non-admin user>
export RESTRICTED_WRITE_USER=$RESTRICTED_USER

# 6. Verify
bash scripts/live-verify.sh
```

**Acceptance:** PASS on **all 11 required reads** (checks 1-11) AND **all 6 required writes** (W1-W6), OR a documented SKIP with reason (e.g., W3 idempotency if Task.SF360_Idempotency_Key__c is a deferred feature).

If any FAIL, iterate in place: fix → re-run `live-verify.sh` → commit. Each fix is its own commit for PR review granularity.

### Phase 9: Fill `docs/live-verification-report.md`

The report template exists in the repo. After Phase 8 passes:

1. Fill the PASS/FAIL/SKIP grid with real observed evidence (not "looked good"):
   - Check 1: `auth login --sf sf360-test` → observed: "Successfully authorized trent@nfchq.com.trentdev1 with org ID 00DEk..."
   - Check 2: `doctor` → observed: `sf CLI passthrough: green`, `Local store: yellow (needs sync)`, ...
   - (through Check 11 + W1-W6)
2. Fill the Sign-off block:
   - Tester: Trent Matthias
   - Date: 2026-04-24
   - Org ID: (from `sf org display`)
   - Org Type: Developer Sandbox
   - Kid: (from `trust register`)
   - JWS: (from `agent context --live $ACME_ID --output /tmp/acme.bundle.json`)
3. Commit with message: `verify(salesforce-headless-360): live-org v1.1 verification pass`.

### Phase 10: PR readiness

1. Rebase/squash where sensible, but keep one commit per phase for reviewability (Matt's team reviews by commit).
2. Update PR #111 description to reference this plan + companion systemic artifact.
3. Add PR comment: *"Verification pass complete against sandbox (NFC TrentDev1). 11/11 required reads PASS, 6/6 required writes PASS, 1 documented SKIP (W3 Task idempotency — F-016, deferred to v1.2). Report signed and committed. Also attached: 14 systemic findings for the Printing Press generator at `2026-04-24-002-feat-printing-press-machine-upgrades-plan.md`. Ready for review."*
4. Leave PR as Ready for Review (not merging — per Matt's plan, he tags v1.1.0 himself).

## System-Wide Impact

### Interaction Graph

A Phase 1 change to `rootFlags` ripples into:
- `cmd/salesforce-headless-360-pp-cli/root.go` — add persistent flag
- Every subcommand that reads org alias — swap from `flags.Org` (per-command, in `trust register` only) to root-level `rootFlags.Org` with per-command override precedence
- `internal/auth/resolve.go` (or equivalent) — new resolution order
- `internal/http/client.go` (or equivalent) — `--verbose` tracing hook
- README, SKILL.md, runbook plan, live-verify.sh — downstream doc/script consumers
- MCP server `cmd/salesforce-headless-360-pp-mcp/` — verify MCP tool schemas don't regress when Go CLI flag schema changes

### Error Propagation

Adding `--org` as global changes the error surface for commands that previously used it as per-command (`trust register`). The per-command flag is deprecated but still honored. Error mapping:
- `--org` absent AND `SF_TARGET_ORG` unset AND no sf default → exit code `4` (authentication required) with clear hint.
- `--org <bad-alias>` → passthrough whatever `sf` returns (already handled).

### State Lifecycle Risks

Phase 7 restructures the metadata source tree. Anyone with an in-flight deploy from the old structure (none except Trent's sandbox at this point) would need to redeploy from the new tree. Low risk — nobody else has deployed this yet per PR #111 status.

### API Surface Parity

`--org` becomes a new public CLI contract. Once released in v1.1.0, consumers (future agents, scripts) will assume it exists. Document in both README and SKILL.md so agent wrappers get it right.

### Integration Test Scenarios

Not covered by unit tests:
1. **Fresh-clone happy path** — clone branch → follow README Quick Start → Cookbook commands all work against `--mock`.
2. **Deploy-to-real-org** — fresh Developer sandbox → `sf project deploy start --source-dir metadata` exits 0 on first try (no manual intervention).
3. **Live-verify end-to-end** — scripts/live-verify.sh from fresh clone passes against the seed output.
4. **SKILL.md → MCP parity** — agent calling MCP server via Claude Desktop can execute each SKILL.md example.
5. **Flag schema regression** — `--org`, `--verbose`, `--agent`, and other persistent flags recognized on every subcommand.

## Acceptance Criteria

### Functional Requirements

- [ ] All ~20 local findings resolved, deferred with documentation, or split to systemic artifact
- [ ] `scripts/live-verify.sh` passes 11/11 required reads + 6/6 required writes against a real Developer sandbox
- [ ] Every command in README.md executable without "unknown flag" or "unknown command" error (verified via `scripts/smoke-readme-commands.sh`)
- [ ] Every command in SKILL.md executable without flag errors
- [ ] `docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md` updated with Step 0.5 + troubleshooting appendix + seed helper reference
- [ ] `docs/live-verification-report.md` filled with observed evidence + signed
- [ ] Two new plan artifacts committed (this one + systemic companion)
- [ ] `metadata/fields/` flat layout removed; `metadata/objects/<SObject>/fields/` canonical
- [ ] `sfdx-project.json` committed at CLI root
- [ ] `scripts/seed-minimal.apex`, `scripts/seed-discover.sh`, `scripts/seed-run.sh` committed

### Non-Functional Requirements

- [ ] CLI builds cleanly with `go install ./...`
- [ ] No new test failures
- [ ] Deploy time for metadata ≤ 30s (current: ~18s)
- [ ] Live-verify runtime ≤ 10 min (current script targets 3-7 min; with seed discover + Apex, extend to 10)

### Quality Gates

- [ ] Each commit message references a specific finding ID (F-NNN)
- [ ] PR description cross-references both plan artifacts
- [ ] Trent signs the live-verification-report.md block with his trust key
- [ ] Matt reviews PR without encountering "this step doesn't work" in a cold-run reproduction

## Success Metrics

- `scripts/live-verify.sh` exit code 0 after fresh-clone reproduction on a different operator's machine (if feasible to verify, e.g., second Developer sandbox)
- PR merged by Matt; v1.1.0 tagged; Benioff outreach unblocked

## Dependencies & Risks

**Dependencies:**
- TrentDev1 sandbox authenticated (done)
- Metadata already deployed to TrentDev1 (done; post-Phase-7 source changes are superset-compatible)
- Fixtures seeded in TrentDev1 (done: ACME_ID=001Ek00001sldZQIAY, OPP_ID=006Ek00000j9ZO2IAM)

**Risks:**
- **R1 — Flag schema change breaks external callers**: no external callers exist (pre-release v1.1). Low risk.
- **R2 — W3 Task idempotency can't be fully verified without resolving F-008**: Acceptable SKIP with documented reason; agents treat Task retries as potentially duplicating. Low risk for verification pass goal.
- **R3 — NFC-specific validation rules leak into generic seed script**: Mitigated by Phase 5's `seed-discover.sh` preflight that describes per-sobject required fields, plus generic `seed-minimal.apex` that uses only universally-safe field values.
- **R4 — Metadata layout change invalidates existing deploys**: only one deploy exists (TrentDev1). Re-deploy is idempotent.
- **R5 — `--verbose` flag conflicts with subcommand flags**: audit existing per-command flag declarations; rename if collision exists.

## Sources & References

### Origin

- **Origin document:** [`/tmp/sf360-findings/README.md`](/tmp/sf360-findings/README.md) — 20 findings F-001 through F-020 from live-verification run 2026-04-22 through 2026-04-24 by Trent Matthias against NFC TrentDev1 Developer sandbox.
- Key decisions carried forward from origin:
  1. Metadata layout must match SFDX source format — 8 metadata bugs (F-007 through F-014) resolved
  2. CLI flag schema must be internally consistent AND match documentation — F-017, F-019, F-020
  3. Runbook plan must cover prod/sandbox/DE auth, metadata deploy prereq, seed helpers, and troubleshooting — F-001, F-002, F-003, F-005, F-006, F-015

### Internal References

- [`docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md`](docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md) — existing runbook (target for Phase 4 updates)
- [`docs/live-verification-report.md`](docs/live-verification-report.md) — report template (target for Phase 9)
- [`docs/security.md`](docs/security.md) — reference for F-004 impact + ActingUser design rationale
- [`scripts/live-verify.sh`](scripts/live-verify.sh) — current script (target for Phase 6)
- [`metadata/README.md`](metadata/README.md) — target for Phase 7 doc update
- [`README.md`](README.md) — Phase 2 target
- [`SKILL.md`](SKILL.md) — Phase 3 target
- [`../hubspot/README.md`](../hubspot/README.md) — reference style (Phase 2)
- [`../../project-management/linear/README.md`](../../project-management/linear/README.md) — reference style (Phase 2)

### External References

- [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) — generator that produced this CLI
- [Salesforce SFDX source format](https://developer.salesforce.com/docs/atlas.en-us.sfdx_dev.meta/sfdx_dev/sfdx_dev_source_file_format.htm) — metadata layout spec
- [Salesforce metadata coverage report](https://developer.salesforce.com/docs/metadata-coverage) — for Activity-object metadata limitations (F-008)

### Related Work

- PR #111 — existing branch that this plan extends
- Companion: `docs/plans/2026-04-24-002-feat-printing-press-machine-upgrades-plan.md` — systemic fixes (advisory for Printing Press generator, not for this CLI)

## Implementation Phases Summary

| Phase | Title | Primary findings | Est. effort |
|-------|-------|------------------|-------------|
| 0 | Checkpoint workspace workarounds | F-007–F-018 (partial) | 5 min |
| 1 | CLI Go source fixes | F-004, F-017, F-019, F-020 | 60 min |
| 2 | README.md rewrite | F-017 (doc aspect), structure | 45 min |
| 3 | SKILL.md audit | F-017 (doc), F-008 (known limit) | 20 min |
| 4 | Runbook plan updates | F-001, F-002, F-003, F-005, F-006 | 30 min |
| 5 | Seed helpers | F-015 | 45 min |
| 6 | live-verify.sh hardening | F-017, F-018, F-019, F-016 | 30 min |
| 7 | Metadata source-of-truth | F-007–F-014 | 20 min (mostly file moves) |
| 8 | End-to-end green-light | All | 30 min |
| 9 | Report fill + sign | verification artifact | 15 min |
| 10 | PR readiness | wrap-up | 15 min |
| **Total** | | | **~5 hours** |
