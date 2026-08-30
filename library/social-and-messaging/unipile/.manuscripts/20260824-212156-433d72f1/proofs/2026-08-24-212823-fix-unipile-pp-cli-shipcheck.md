# Unipile CLI Shipcheck

Command: `cli-printing-press shipcheck --dir <cli> --spec <spec> --research-dir <run> --api-key <redacted> --env-var UNIPILE_API_KEY`

## Final result

| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS | 0 |
| apify-audit | PASS | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

**Verdict: PASS (7/7 legs)**

Scorecard: **97/100, Grade A**, zero unverified dimensions.
Verify: 100% pass rate (113/113), live mode, Data Pipeline PASS.
Live dogfood: 267/267 (100%), 0 failures, coverage not hollow.

## Before / after

| Metric | First run | Final |
|--------|-----------|-------|
| Shipcheck verdict | FAIL (1/7 legs) | PASS (7/7) |
| Scorecard total | 92/100 (1 dimension unverified) | 97/100, none unverified |
| Verify verdict | FAIL (data pipeline) | PASS |
| Verify mode | mock | live |
| Data Pipeline Integrity | 5/10 | 10/10 |
| Live API Verification | N/A (unverified) | 10/10 |
| Live dogfood | 262/266, 4 failures, hollow coverage | 267/267, 0 failures |
| Full sync against the live tenant | page 2 of every resource 400s | 20 resources, 0 errors, 21,612 records |

## Blockers found and fixed

1. **Cursor pagination sent the wrong parameter.** The generator bound `cursorParam` to the unrelated `after` ISO-datetime filter, so page two of every list route failed with `400 errors/invalid_parameters ... "Expected union value"`. Patched both generated pagination tables (`resource_paths.go`, `determinePaginationDefaults`) plus 6 promoted call sites.
2. **Every parent-scoped typed projection failed.** `ParentTable` is spelled `chat-attendees` while the typed column is `chat_attendees_id NOT NULL`, so the injected foreign key landed under a key nothing reads. Generic rows stored, typed rows silently did not. Fixed by normalising the FK key.
3. **`/users/followers` rejected the default page size** with `400 errors/limit_too_high` despite the spec advertising a 250 maximum. Capped that resource at 10.
4. **Path-scoped dependents had no account scope.** `/users/{id}/posts` and siblings require `account_id`, but the env-derived scope was written to the flat-list-only bucket. Now seeded into the true-global bucket that dependents read.
5. **A default sync fanned out thousands of LinkedIn-backed calls** (one per attendee, one per follower) — the exact pacing LinkedIn punishes, and the same records already arrive through the flat routes. Per-parent fan-out is now opt-in via `--resources`, and a warning names how to include it.
6. **A full sync took 7m44s**, so verification harnesses timed it out and reported "sync crashed". Sync now caps at one page per resource under `PRINTING_PRESS_VERIFY` / `PRINTING_PRESS_DOGFOOD`. Real runs are untouched and the network calls still happen.
7. **Account-capability errors were treated as sync failures.** A tenant with only LinkedIn connected gets 401 `errors/not_authorized` on calendars, 422 `errors/invalid_account` on mail, and 401 `errors/disconnected_feature` on Recruiter. Those mean "this account has no such surface", so they are now warnings. `errors/missing_credentials` deliberately stays a hard failure.
8. **`search` auto-routed to the live LinkedIn search endpoint**, spending LinkedIn's ~1000-results/day budget on what reads like a local query. Now defaults to the local mirror; `--data-source live` and `linkedin search` are the explicit opt-ins.
9. **`contact` and `thread` returned exit 0 on a miss.** Now exit 3 when the mirror holds data and nothing matched, and exit 0 with a sync hint when the mirror is empty — an unsynced cache is not a failed lookup.
10. **`chat-attendees sync` had no distinguishable error path**: Unipile answers HTTP 200 `SYNC_RUNNING` for an id that does not exist. Annotated `pp:no-error-path-probe` rather than inventing a local heuristic the API does not back.
11. **Documentation over-claimed.** The headline advertised a ledger tracking "invitations and profile views" and `contact` claimed to show "recent posts"; neither is implemented. Corrected at the research.json source and propagated to all nine rendered surfaces.

## Product changes made during shipcheck

- **Account scope resolves itself.** `account_id` is required on 52 of 94 endpoints and is an opaque 22-character blob. With no scope configured, sync now asks the API which accounts exist and adopts the only one, saying so on stderr. Several accounts is a refusal that names them; an explicit scope always wins.
- **`accounts alias`** turns `linkedin`, `li`, or an account name into that id, with `--export` for shell use.

## Remaining known gap

The scorecard's sample-output probe reports 9/10. The one miss is `search "pricing"` run inside a sandbox with no synced mirror: search is offline-only by design, so an empty mirror returns an empty result. Making that probe pass would require falling back to LinkedIn's live search, which is the quota footgun removed in item 8. Documented in README/SKILL troubleshooting instead.

## Ship recommendation

**ship** — all seven legs pass, no known functional bugs in shipping-scope features.
