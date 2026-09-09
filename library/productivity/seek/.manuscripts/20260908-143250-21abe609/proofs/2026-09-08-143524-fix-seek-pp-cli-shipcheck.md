# SEEK CLI — Phase 4 Shipcheck

Run: 20260908-143250-21abe609 · binary v4.32.0 · 1 fix loop.

## Final result

```
LEG                 RESULT  EXIT
verify              PASS    0     (live mode, real SEEK API)
validate-narrative  PASS    0
dogfood             PASS    0
workflow-verify     PASS    0
apify-audit         PASS    0
verify-skill        PASS    0
scorecard           PASS    0

Verdict: PASS (7/7 legs passed)
```

Scorecard **84/100 — Grade A**. Live API Verification 10/10 (verify ran against
the real `au.seek.com`). Sample Output Probe 5/6 — the 6th (`me new-jobs`)
correctly exits 4 (auth-required) without a SEEK session, which is the intended
typed behaviour, not a failure.

## Blockers found and fixed (loop 1)

| # | Blocker | Fix |
|---|---|---|
| 1 | `me new-jobs` → GraphQL 400: `apacSavedSearches.query` is `SavedSearchApacQuery!` and `createdDate` is `SeekDateTime!` — both need sub-selections, not scalars | Probed the live schema (validation runs before auth): `query { searchQueryString }`, `createdDate { dateTimeUtc }`. Fixed the spec's `me/saved-searches` query default and the `fetchSavedSearches` helper; `seekSavedSearch.Query` is now a struct. |
| 2 | `verify-skill` FAIL: `company --active` referenced in README/SKILL, not declared | Added a real `--active` bool flag to `company` (drops openings listed >30 days ago). |
| 3 | `verify-skill` / scorecard: `classifications resolve "software"` — 2 positional args vs `[term]` | Fixed the example in `research.json` (`novel_features`, `novel_features_built`, recipes) to `classifications software`; also made the command strip an optional leading `resolve`/`list`/`search`/`find` verb. |
| 4 | Scorecard `me new-jobs` probe: HTTP 200 + `code: UNAUTHENTICATED` body was surfaced as a generic exit-5 error | `fetchSavedSearches` now detects `UNAUTHENTICATED` / null `viewer` in the 200 body and returns the typed `authErr` (exit 4); `me new-jobs` declares `pp:typed-exit-codes: "0,4"`. |
| 5 | Scorecard `live_api_verification` unverified (verify ran mock-only) | Re-ran shipcheck with `--env-var SEEK_COOKIE` (placeholder value) so verify exercises the real API's read-only surface. Now 10/10. |

Regen after the spec change was `--force` novel-only preservation: all 6
hand-authored novel command files + `internal/seekparse/` + `seek_novel.go` +
`seek_classifications_data.go` survived; `me_saved-searches.go` re-emitted with
the corrected query.

## Before / after

| Metric | Before | After |
|---|---|---|
| shipcheck verdict | FAIL (verify-skill FAIL, scorecard HOLD) | **PASS (7/7)** |
| Sample Output Probe | 3/6 | 5/6 (+1 correctly auth-gated) |
| verify pass-rate | PASS (mock) | PASS (live) |
| scorecard total | 82 (HOLD) | **84 (PASS), Grade A** |
| Live API Verification | N/A | 10/10 |

## Remaining gaps (not ship blockers)

- **`auth_protocol 2/10`** — cookie auth is emitted but the `press-auth`
  companion and `login_complete_selector` verification aren't fully wired.
  Only the 3 `me/*` commands need it; the whole public surface is keyless.
  Polish candidate (`/printing-press-polish`).
- **`Cache Freshness 3/10`, `Dead Code 3/5`** — no framework `sync` command
  (SEEK is search-only; the novel commands self-cache), plus 3 generator-emitted
  dead helpers (`handleBinaryResponseDelivery`, `hasChangedLocalFlags`,
  `isDryRunResponseForClient`) and the dead root `--max-age` flag. Generator-side.
- **`me new-jobs`** saved-search `query` parsing is best-effort; an unrecognised
  `searchQueryString` shape lists that search under `skipped_searches` rather
  than failing the run.

## Ship recommendation: **ship**

All ship-threshold conditions met. Every flagship/approved feature returns
correct output against the real SEEK API (`salary`, `trends`, `company`,
`listings facets`, `classifications` verified with live data; `me new-jobs`
correctly requires a session). No known functional bug in shipping scope.
