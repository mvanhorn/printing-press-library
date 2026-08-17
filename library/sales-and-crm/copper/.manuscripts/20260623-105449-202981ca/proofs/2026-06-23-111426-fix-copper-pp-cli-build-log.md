Manifest transcendence rows: 7 planned, 7 built.

# Phase 3 — Transcendence commands build log

All 7 novel commands implemented in `internal/cli/` with real, table-driven
acceptance tests in the `_test.go` siblings. Each file carries a
`// pp:data-source local|live` annotation and the correct `mcp:read-only`
disposition. Core logic is extracted into pure functions (e.g. `runForecast`,
`runStale`, `runDedupe`, `runWho`, `runBulk`, `runUpsert`, `buildActivityBody`)
so behavior is asserted without the network; RunE is a thin wrapper.

## Per-feature

1. **forecast** (store reader, `pp:data-source local`, read-only) — `forecast.go`
   Weighted pipeline roll-up. `runForecast(ctx, db, opts) []forecastRow`:
   weighted = SUM(monetary_value * win_probability/100), open = SUM(monetary_value),
   count; GROUP BY stage | assignee (json_extract assignee_id) | close-month
   (substr of close_date). Flags `--pipeline --by --status(Open) --db`. JSON +
   table. Missing-mirror guard.
   Test asserts: stage 9 with (1000@50%, 2000@25%) → weighted 1000, open 3000,
   count 2; Won opp excluded; assignee grouping → 2 buckets; pipeline filter miss → 0.

2. **stale** (store reader, local, read-only) — `stale.go`
   Cold-deal sweep. `runStale` / `runStaleByAssignee`. Staleness from
   json_extract date_last_contacted; NULL treated as MAXIMALLY stale (never
   contacted, days_stale=-1) and sorted first, then oldest contact, then value.
   Flags `--days(30) --by(assignee) --pipeline --status(Open) --limit(50) --db`.
   Test asserts: 60d-old appears, contacted-today excluded, never-contacted
   appears and sorts first, Won excluded; by-assignee aggregation counts/values.

3. **bulk** (live API, `pp:data-source live`, mutating, NOT read-only) — `bulk.go`
   `bulk move|reassign --query file.json | --ids csv --set k=v… --concurrency(3)
   --apply`. Bounded worker pool; per-id PUT /opportunities/{id}; exponential
   backoff on 429 (`putWithRetry`, cap 5) surfacing `*cliutil.RateLimitError` —
   never empty success. Preview by default; mutates only with `--apply` AND
   `!cliutil.IsVerifyEnv()`. Non-zero exit (apiErr) when failures after apply.
   Test asserts: `--set` parse (int/float/string), persistent 429 →
   RateLimitError, happy path 3 applied, verify-env preview exits 0 no network.

4. **upsert** (live API, live, mutating) — `upsert.go`
   `upsert person|lead --match email --file rows.json --apply`. person email
   scalar → `emails:[{email,category:"work"}]` (normalizePerson); lead keeps
   scalar. **person**: lookup via POST /people/fetch_by_email → PUT /people/{id}
   (update) or POST /people (create). **lead**: single PUT /leads/upsert (Copper's
   server-side idempotent endpoint matches by key — no client probe, no
   duplicate-on-rerun). Preview default; verify-env short-circuit.
   Test asserts: normalize shape, plan build (+ missing-match error), person
   create-vs-update routing via fake client (1 created/1 updated, PUT to
   /people/55), lead routes to a single PUT /leads/upsert with no POST,
   verify-env preview exits 0.
   (Existing generated `newLeadsUpsertCmd` in leads_upsert.go left untouched.)

5. **dedupe** (store reader, local, read-only) — `dedupe.go`
   `dedupe people|leads --on email|name|company --db`. Self-join via IN-subquery
   on a case-insensitive lower(trim(key)); groups with >1 member.
   Test asserts: 2 people a-at-x.example/A-at-X.example + 1 b-at-x.example → one group size 2 on
   email (ids 1,2); unique name → 0 groups.

6. **log** (live API, live, mutating) — `log.go`
   `log call|note|meeting|fix --on entity:id --note … [--activity id] --apply`.
   buildActivityBody → POST /activities {parent, type{category:user,id}, details,
   activity_date}. Note=0 hardcoded; call/meeting resolved by name from GET
   /activity_types (matchActivityType handles keyed + flat shapes), fallback to
   Note(0) with warning. `log fix` deletes then recreates (immutable activities).
   Preview default; verify-env short-circuit.
   Test asserts: body shape (parent/type/details/date), activity-type matching
   both response shapes, verify-env preview exits 0, bad --on errors.

7. **who** (store reader, local, read-only) — `who.go`
   `who opportunity:id` or `--id --type`. parseEntityRef("opportunity:88"). Loads
   opp → company (company_id) → primary contact (primary_contact_id) → recent
   activities (parent.id/parent.type, limit 10) into one object. Missing-mirror
   guard; missing opp → notFoundErr.
   Test asserts: parseEntityRef table (valid + malformed), opp 88 assembles
   company 5, person 7, activity 900 (unrelated activity on opp 99 excluded),
   missing opp → nil/no error.

## Verification
- `go build ./...` → exit 0.
- `go vet ./internal/cli/` → exit 0.
- `go test ./internal/cli/` → ok (all 24 Novel* tests pass).
- Binary (`/tmp/copper-bin`) live-exercised against a seeded SQLite mirror:
  forecast/stale/dedupe/who produce correct JSON + tables; bulk/log/upsert
  print PREVIEW plans and exit 0 under PRINTING_PRESS_VERIFY=1 with no network;
  missing-mirror guard prints the sync hint + `[]` and exits 0.

## Known pre-existing (out of scope, NOT caused by Phase 3)
`go test ./...` shows 4 failures in `internal/cliutil/credentials_test.go`
(TestCorruptCredentialsFallsBackToLegacyConfig,
TestEmptyCredentialsFileDoesNotClearLegacyConfig,
TestAuthWriteMigratesLegacyConfigToCredentialsOnly,
TestAuthWriteScrubsLegacyConfigWhenRelocated) — legacy-config→credentials
migration logic. These files were never touched by Phase 3 and none of the 7
novel commands reference credential code. `internal/cli` (all novel work)
passes fully.

Phase 4.95: 2 security warnings autofixed (numeric-id validation in bulk + log), regression test added. 7 planned, 7 built.
