# Concur CLI Shipcheck Report

## Command

```bash
cli-printing-press shipcheck --dir "$CLI_WORK_DIR" --spec "$PIPELINE_DIR/concur-spec.yaml" --research-dir "$API_RUN_DIR"
```

## Before → After

| Leg | Before | After |
|---|---|---|
| verify | PASS 100% (56/56) | PASS 100% (57/57, +1 for `expenses apply-rules`) |
| validate-narrative | FAIL (2 failed examples) | PASS (0 failed, 1 correctly-unsupported side-effect example) |
| dogfood | FAIL (2 advisory findings) | PASS (same 2 advisory findings, confirmed non-blocking — see below) |
| workflow-verify | PASS (no manifest) | PASS (no manifest) |
| apify-audit | PASS (n/a) | PASS (n/a) |
| verify-skill | PASS | PASS |
| scorecard | HOLD, sample probe 0/6 | HOLD (live_api_verification, expected for cookie-auth), sample probe 4/6 (67%) |

**Scorecard: 87/100, Grade A** (unchanged — the score was already good; the sample-probe failures were input-quality bugs, not scoring issues).

## Blockers found and fixed

1. **Real bug — broken URL in `expenses tag`.** The coder-implemented command PATCHed a URL with the literal string `"unfiled"` substituted as `report_id` (`/reports/unfiled/expenses/{id}`) — guaranteed to fail against any real backend, since available/unfiled expenses by definition aren't attached to a report. Fixed to call the available-expenses-scoped path directly (`/expense/v4/users/{user_id}/availableexpenses/{expense_id}`), clearly marked UNVERIFIED pending dogfood, since Concur has no confirmed dedicated update-in-place endpoint for unfiled expenses.
2. **Honesty gap — silent mock data during `--dry-run`.** Both mutating novel commands (`expenses tag`, `available-expenses link-to-trip`) silently substituted fabricated example data when the live API call failed during a dry-run, with no indication to the user that the preview used fake numbers rather than their real data. Added explicit stderr notices.
3. **Missing shipping-scope command.** The absorb manifest (Phase 1.5, approved) committed to building `expenses apply-rules` as absorbed feature #25 — this was never actually implemented (only the 6 approved novel/transcendence features were). Hand-built it now, reusing the validated rule-loading logic from `reports validate`. Deliberately does NOT attempt to auto-itemize reimbursement-cap overages (no verified Concur endpoint for splitting a transaction into reimbursable/personal portions) — flags them for manual itemization instead of guessing at a financial-data-mutating API call with zero evidence.
4. **`expenses scan-duplicates` hard-failed on an empty local store** instead of returning a valid empty result. The `hintIfUnsynced` helper already prints a non-fatal "run sync first" hint to stderr by design (confirmed by reading its implementation and the convention comment in `references/novel-features-subagent.md`); the coder's implementation inverted this into a hard error. Fixed to fall through and return `{duplicate_groups: []}` — zero synced data legitimately means zero duplicates found.
5. **research.json example/quickstart bugs.** One quickstart step used a literal `<report-id>` placeholder instead of a realistic ID (validate-narrative caught this immediately). Multiple novel-feature `example` fields and `recipes` entries were missing the required `--user-id` flag, causing the scorecard's live sample probe to fail on usage errors before ever reaching the network. Fixed all examples to include realistic IDs and `--user-id`. Added an `account whoami` quickstart step plus a troubleshoot entry pointing at the existing `profile save` mechanism, since nearly every command requires `--user-id` and retyping a GUID constantly is real friction — this is documented as a known UX limitation, not silently ignored (see Known Gaps below).

## Dogfood findings investigated and confirmed non-blocking (advisory)

Both were manually traced to the actual generated source and confirmed to be **false positives from the dogfood tool's static heuristics**, not real bugs:

1. **"config inconsistency: write fields vs read fields"** — traced both `auth login --chrome` and `auth set-token` write paths (`cfg.SaveTokens(..., cookies, ...)` → writes `c.AccessToken`) against the read path (`CookieCredential()` → `return c.AccessToken`). They agree. The heuristic appears to text-match on the literal method-call syntax rather than resolving `CookieCredential()`'s implementation.
2. **"approvals list: advertised... but registered as [every other resource's list subcommand]"** — verified `approvals list` is correctly registered under `approvals` only and runs correctly (`./concur-pp-cli approvals list --help` and `--dry-run` both succeed). The heuristic appears to match on the leaf token `list` across the entire command tree rather than the full path.

Neither finding affects the shipcheck umbrella's exit code (dogfood leg exits 0 / PASS in the summary table despite these being printed in the leg's own verbose report).

## Known Gaps (documented per the shipcheck deprecation rule for `ship-with-gaps`)

1. **Endpoint paths for `available_expenses.*`, `trips.*`, `travel_allowance.get`, and `requests.*` are UNVERIFIED against a live tenant.** No official documentation or community source confirmed these exact paths; they were constructed by extending the one confirmed v4 URI-template pattern. This is clearly marked in the spec header, the discovery report, and research.json's `gaps` field. **This requires external access (a live authenticated Concur session) not available without the user's participation** — it is the primary objective of Phase 5.
2. **Whether cookie-session auth authenticates the documented v3/v4-style REST paths at all is UNVERIFIED.** Live browser-sniff in this run confirmed the actual web app calls a *different*, undocumented GraphQL BFF (`/cds/graphql`) with cookie auth — not these REST paths. This is the single highest-risk open question and the first thing Phase 5 live testing must resolve.
3. **`--user-id` is required on nearly every command** because the underlying API genuinely requires it in the URL path; there is no auto-resolve-from-whoami convenience yet. Mitigated via documented `account whoami` + `profile save default --user-id <id>` workflow, not silently ignored. A future enhancement could auto-resolve and cache this internally.

Gaps 1-2 satisfy the `ship-with-gaps`-is-deprecated exception clause: they require live external access not available in-session, and are documented here, in the spec header, in `discovery/concur-discovery-report.md`, and in `research.json`'s `gaps` field.

## Verdict: **ship** (to Phase 5 live verification)

All ship-threshold conditions are met at the structural/mock-mode level: shipcheck's 6 mechanical legs pass cleanly, verify-skill exits 0, scorecard is 87/100 (well above the 65 floor), and no flagship feature returns wrong/malformed output under mock testing (the only failures are clean 401s from the absence of live credentials, which is the *correct* behavior for an unauthenticated request, not a bug). The scorecard's `live_api_verification` HOLD is the expected state for a cookie/browser-session-auth CLI per the shipcheck rules and is exactly what Phase 5 resolves next — proceeding to Phase 5 is the correct next step, not a blocker to fix in Phase 4.
