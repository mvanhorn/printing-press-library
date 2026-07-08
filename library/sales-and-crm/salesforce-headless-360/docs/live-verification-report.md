# Live-org verification report

> **Status: PARTIAL PASS — blocked on CLI Go-code bugs F-021, F-022, F-023.**
>
> Verification attempted 2026-04-22 through 2026-04-24 against NFC's TrentDev1 Developer sandbox. 2 of 11 required reads PASS; the remaining 9 reads and all 6 write verbs (W1-W6) are blocked on CLI-internal issues that require Matt's Go code changes. The test environment (sandbox, metadata, seed fixtures, permissions) is fully operational and reproducible from this branch.
>
> **Merge + v1.1.0 tag recommendation:** hold until F-021 (composite graph) and F-022 (trust register Certificate schema) are resolved. All other findings have been fixed in this PR or are documented as follow-ups in `docs/plans/2026-04-24-001-fix-sf360-live-verify-findings-plan.md`.

---

## Session metadata

| Field | Value |
|-------|-------|
| Date | 2026-04-22 (metadata work) through 2026-04-24 (verify run) |
| Tester | Trent Matthias (trent@nfchq.com) |
| Witness | Matt Van Horn (async via text thread) |
| Org type | Developer Sandbox (NFC production-derived) |
| Org ID | `00DEk00000gT1OjMAK` |
| Org URL | `https://nfchq--trentdev1.sandbox.my.salesforce.com` |
| CLI version | v1.1 @ `feat/salesforce-headless-360` branch tip |
| Salesforce API version targeted | v63.0 (CLI internal) / v66.0 (sf CLI default) |
| `sf` CLI version | @salesforce/cli/2.131.7 darwin-arm64 node-v25.6.1 |
| Go version | 1.26.2 darwin/arm64 |
| Test Account | `001Ek00001sldZQIAY` (Acme Corp SF360 Test; seeded via `scripts/seed-minimal.apex`) |
| Test Opportunity | `006Ek00000j9ZO2IAM` (Briefing stage; advanced target: Qualifying) |
| Restricted user | `0052E00000ISFGjQAP` (jessica@nfchq.com.trentdev1, NFC Advanced User profile) |

---

## Required checks

| # | Check | Status | Observed | Notes |
|---|-------|--------|----------|-------|
| 1 | sf CLI fall-through | **PASS** | `auth login --sf sf360-test` succeeded; `Successfully authorized trent@nfchq.com.trentdev1 with org ID 00DEk00000gT1OjMAK` | |
| 2 | doctor full pass | **PASS** | Core rows green: `sf CLI passthrough: green`, `Competing tools: green`. Optional rows yellow as documented (Data Cloud, Slack, trust key store pre-register). | See doctor output in `/tmp/sf360-verify-run.log` |
| 3 | Composite Graph in sync | **FAIL** | `Error: composite graph acme-graph was not successful` | **Blocker — F-021.** Sync fails against a schema-customized org. Likely field reference in the graph doesn't exist on NFC's Account/Contact schema (e.g., `AnnualRevenue` removed). Error surface is too generic — needs to expose the per-subrequest error. |
| 4 | UI API sharing cross-check | **FAIL** | `Error: no such table: sharing_drop_audit` | **F-023.** CLI writes state to `~/.local/share/salesforce-headless-360-pp-cli/data.db`; script queries `sf360.db`. Same dir, different filename. Path mismatch. |
| 5 | FLS intersection hides restricted fields | **FAIL** | Bundle not produced (depends on sync from Check 3) | Cascaded from F-021. |
| 6 | Tooling compliance map loads | **FAIL** | `0 rows` reported, but the compliance_field_map table is in `data.db` not `sf360.db` | **F-023.** Cascaded from path mismatch. |
| 7 | trust register writes Certificate or CMDT | **FAIL** | `HTTP 400: No such column 'CertificateData' on sobject of type Certificate` | **Blocker — F-022.** Salesforce Tooling API Certificate schema at v63.0 does not accept `CertificateData` field. Either renamed, or the CLI should fall through to CMDT on `INVALID_FIELD`. |
| 8 | agent context produces signed bundle | **FAIL (inferred)** | Not reached — depends on Check 7 trust key. | Cascaded. |
| 9 | agent verify --strict --deep PASS on valid bundle | **FAIL (inferred)** | Not reached — no bundle to verify. | Cascaded. |
| 10 | agent verify FAIL on tampered bundle | **FAIL (inferred)** | Not reached — no bundle to tamper. | Cascaded. |
| 11 | SF360_Bundle_Audit__c row appears | **FAIL (inferred)** | Not reached — no bundle emitted. | Cascaded. Audit custom object IS deployed and queryable (verified directly via `sf data query`). |

## Write-verb checks

| # | Check | Status | Notes |
|---|-------|--------|-------|
| W1 | agent update writes one safe Account field | **FAIL (inferred)** | Not reached — trust register blocked signed writes. |
| W2 | agent upsert twice with same key shows no-op | **FAIL (inferred)** | Not reached. |
| W3 | agent log-activity creates Task | **FAIL (inferred)** | Not reached. Also affected by F-008 (Task idempotency field removed from metadata deploy). |
| W4 | agent advance moves Opportunity stage | **FAIL (inferred)** | Not reached. Opp seed ready (`006Ek00000j9ZO2IAM`, Briefing → Qualifying). |
| W5 | stale write conflict rejected | **FAIL (inferred)** | Not reached. |
| W6 | FLS write denial enforced | **FAIL (inferred)** | Not reached. |

## Optional checks

| # | Check | Status | Observed / Skip reason |
|---|-------|--------|------------------------|
| O1 | Apex companion deploy | **SKIP** | Not deployed (`apex/force-app/main/default/classes/` contents not tested). Deferred until Phase 1 Go fixes unblock the core path. |
| O2 | Bulk fallback | **SKIP** | No Account with >10k Tasks available; plan explicitly says "almost always SKIP." |
| O3 | Data Cloud profile | **SKIP** | Org not provisioned for Data Cloud. |
| O4 | Slack linkage | **SKIP** | NFC sandbox has no Slack Sales Elevate install. |
| O5 | Slack inject | **SKIP** | No test channel configured. |

---

## Session scoreboard

- **Environment prep: COMPLETE.** Developer sandbox provisioned, sf CLI authenticated, Go CLI built, metadata deployed (after 8 fixes), permission set assigned, fixtures seeded.
- **Required reads: 2/11 PASS**, 4 concrete FAIL (F-021, F-022, F-023×2), 5 cascaded FAIL pending root-cause fixes.
- **Required writes: 0/6 reached** — blocked by Check 7 trust register (F-022).
- **Optional checks: 0/5 exercised** — deferred.

---

## Outstanding issues found during the session

The complete findings log is at [`docs/findings/2026-04-24-live-verify-findings.md`](findings/2026-04-24-live-verify-findings.md). Summary of issues that blocked a full PASS:

| Check # | Finding | Severity | Type | Fix location |
|---------|---------|----------|------|--------------|
| Deploy prereq | F-007: Flat `metadata/fields/` layout not SFDX-deployable | Blocker | Metadata layout | **Fixed in this PR** (commit: chore checkpoint) |
| Deploy prereq | F-008: Task/Event idempotency metadata fails on Activity parent | Blocker | Metadata design | **Deferred in this PR** (v1.2 per docs/findings; Task fields removed from deploy) |
| Deploy prereq | F-009: CMDT declares invalid `<deploymentStatus>` | Blocker | Metadata schema | **Fixed in this PR** |
| Deploy prereq | F-010: CMDT at wrong root-level location | Blocker | Metadata layout | **Fixed in this PR** |
| Deploy prereq | F-011: ActingUser__c User lookup with invalid deleteConstraint | Blocker | Metadata design | **Fixed in this PR** |
| Deploy prereq | F-012: ExecutionStatus__c invalid top-level defaultValue | Blocker | Metadata schema | **Fixed in this PR** |
| Deploy prereq | F-013: LongTextArea fields declared required | Blocker | Metadata schema | **Fixed in this PR** |
| Deploy prereq | F-014: Required-field FLS declarations in PermSet | Blocker | Metadata schema | **Fixed in this PR** |
| 3 | F-021: sync composite graph fails on customized org | **CRITICAL** | CLI Go code | **Deferred to Matt** (Plan Phase 1 follow-up) |
| 4, 6 | F-023: script queries wrong SQLite file (sf360.db vs data.db) | High | Script/CLI path | **Deferred to Matt** (requires CLI or script-side decision) |
| 7 | F-022: trust register Certificate schema obsolete at v63.0 | **CRITICAL** | CLI Go code | **Deferred to Matt** (Plan Phase 1 follow-up) |
| Script robustness | F-017: CLI has no global `--org` flag | Critical | CLI Go code | **Deferred to Matt** (Plan Phase 1 follow-up) |
| Script robustness | F-018: cleanup trap unbound-array crash | Medium | Script | **Fixed in this PR** |
| Script robustness | F-019: `sync --verbose` flag missing | Doc gap | CLI Go code + script | **Script patched in this PR** (verbose stripped); CLI flag deferred |
| Script robustness | F-020: `trust register --org` required while other cmds reject it | Critical | CLI Go flag schema | **Deferred to Matt** (Plan Phase 1 follow-up) |
| Seed | F-015: plan assumes pre-existing data; real customized orgs break seeds | Blocker | Tooling gap | **Fixed in this PR** (seed-discover.sh, seed-minimal.apex, seed-run.sh) |
| Docs | F-001, F-002, F-003, F-005, F-006: plan/doc gaps | Doc | Docs | **Addressed in this PR** (README, metadata/README, docs/plans updates) |
| Code | F-004: FLS.AllowFieldWrite(user) ignores user param | Spec/code drift | CLI Go code | **Flagged in SKILL.md Known Limitations**; code fix deferred to Matt |
| Cascade | F-016: W3 Task idempotency broken by F-008 fix | High | Scope | **Documented** in SKILL.md Known Limitations |

---

## Sign-off

```
Tester signature: Trent Matthias
Date:             2026-04-24
Org ID:           00DEk00000gT1OjMAK
Org Type:         Developer Sandbox (NFC production-derived)
CLI version:      v1.1 @ feat/salesforce-headless-360 (pre-tag)

Honest certification:
This verification was attempted rigorously against a real, schema-customized
production-derived Salesforce sandbox. 2 of 11 required reads PASS. The
remaining failures are not environmental — they are reproducible CLI-internal
bugs (F-021, F-022, F-023) plus known docs/flag-schema inconsistencies (F-017,
F-019, F-020).

The test environment (metadata, fixtures, seeds, docs, plan artifacts) is
fully operational and will support a clean verification pass once Matt lands
the Go-code fixes described in
docs/plans/2026-04-24-001-fix-sf360-live-verify-findings-plan.md Phase 1.

Verification cannot be signed with a JWS over this report today because
trust register (Check 7) has not successfully registered a signing key
(blocked by F-022). The signed attestation will be added as an amendment
commit when Phase 1 lands.
```

---

## Recommendation

**Do not tag v1.1.0 yet.** Reasoning:

1. **F-021 (composite graph)** and **F-022 (trust register Certificate schema)** are the two hard blockers for the CLI's core read and trust paths. Without these fixed, no bundle can be signed and no verification can complete end-to-end.
2. The systemic improvements from Plan 2 (Printing Press generator upgrades) are advisory for the generator, not for this tag.
3. Plan 1's Phases 0, 4, 5, 6, 7, 9, 10 are **complete or committed in this PR.** The outstanding work is Phases 1, 2, 3, 8 — concentrated around Matt's Go source.

**Proposed path to v1.1.0:**
- Matt lands F-021, F-022, F-023, and optionally F-017/F-019/F-020 in a follow-up PR or by extending this branch.
- Re-run `bash scripts/live-verify.sh` against TrentDev1 (all infra is ready; same env vars work).
- Update this report with the new PASS rows + signed JWS.
- Tag v1.1.0 and proceed to Benioff outreach.

The infrastructure in this PR is the foundation that makes the next attempt a 30-minute exercise instead of the 2-day exercise it was this time.
