Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

# iClassPro CLI — Phase 3 Build Log

## What was built

### Priority 0 — data layer

The generator emitted **no** `sync`, `search`, `sql`, `stale`, or `tail` command and no `internal/syncer` package. Every endpoint in the spec requires positional `{account}` and/or `{locationId}` path parameters, so no resource was classified as syncable and the whole local-store stack was skipped. Five of the eight approved novel features depend on that stack.

`internal/store` *was* emitted, so the data layer was hand-authored against it using the separate-file extension pattern (regen-safe by construction):

- `internal/store/iclasspro_migrations.go` — three tables plus their accessors:
  - `icp_sync_runs` — one row per sync
  - `icp_snapshot` — full entity payload per run, powering `drift`
  - `icp_openings_history` — append-on-sync availability observations, powering `fill-rate` and `watch`
  - `PruneICPRuns` keeps the last N snapshots per account; openings history is retained because its value grows with depth.
- `internal/cli/iclasspro_sync.go` — the `sync` command. Walks every active location, resolves camp typeIds from the **booking menu** (never `camp-programs` — that returns programIds, which silently match nothing), paginates on `totalRecords`, and records observations.
- `internal/cli/iclasspro_common.go` — envelope decoding, gate classification, catalog collection, store plumbing, stale hints.

`RecordICPObservations` writes snapshot and history rows in one transaction and deliberately does **not** call `Upsert` inside it: `Upsert` opens its own write transaction and SQLite permits a single writer. `resources` rows are written after that transaction commits.

### Priority 1 — absorbed

31 absorbed manifest rows ship. Generated endpoint commands cover locations, booking menu, all three program families, levels, instructors, sessions, classes (list + get), camps (list + get), party availability, products, and news. Hand-authored behavior covers pagination, the typeId trap, gate classification, media-URL absolutization, portal deep links, and the read-only auth path.

`internal/cli/iclasspro_auth.go` implements manifest row 31: `auth login|status|logout`. Credentials are read from `ICLASSPRO_EMAIL` / `ICLASSPRO_PASSWORD` and never accepted as flags; only the returned token is persisted, at `~/.config/iclasspro-pp-cli/session.json` with mode 0600. The token is replayed as a `token` query parameter on catalog reads via `icpGet`. No cart, enrollment, promo-code, or checkout command exists anywhere in the CLI.

### Priority 2 — transcendence (8 of 8)

| # | Command | Data source | Verified live |
|---|---|---|---|
| 1 | `watch` | live + local baseline | 27 entities watched on `scaq`, observations recorded |
| 2 | `drift` | local | 2 runs, 124 entities compared |
| 3 | `opens-soon` | local | 12 findings on `scottsdalegymnastics` at `--days 60` |
| 4 | `calendar` | local | 316 VEVENTs from 124 entities, 0 skipped |
| 5 | `tenant` | live | correctly classifies `nadoclub` as sign-in-gated on 3 surfaces |
| 6 | `fill-rate` | local | 27 trends from 81 observations across 3 syncs |
| 7 | `compare` | local | 3 buckets across 2 accounts |
| 8 | `lint` | local | 124 entities checked, 34 info findings |

All eight are hand-written Cobra commands, as declared at the Phase 1.5 gate.

### Pure-logic package

`internal/icp` holds all domain logic — normalization, drift diffing, fill-rate math, registration windows, lint rules, RFC 5545 rendering, cross-tenant aggregation — with **24 table-driven tests** and no I/O. Fixtures are real payloads captured live on 2026-08-05.

## Bugs found and fixed during the build

Three real defects, all caught by exercising the commands against live data rather than by the build passing.

### 1. `lint` flagged every camp as missing a description

The `/camps` list endpoint does not return a `description` field at all; only `/camps/{id}` does. The rule fired on all 28 camps at `scottsdalegymnastics`, reporting a catalog as broken for data the response never carried.

**Fix:** added `Entity.Detailed`, set only when the payload actually contains `description` or `blocks`. Detail-only rules now consult it. Regression test: `TestLintDoesNotFlagListSourcedCampsForDetailOnlyFields`. Result: 28 phantom warnings → 0.

### 2. `calendar` silently dropped every camp

Camps from the list endpoint carry no `blocks` and no `availableDates`, so no slot ever received a date and all 28 were skipped — while classes exported fine, which made the output look plausible.

**Fix:** `spanDates()` expands `startDate`..`endDate` when no explicit dates exist, honoring the weekday of any schedule slot (a Saturday camp across a week yields Saturdays), capped at 366 days. Explicit `blocks` still win. Regression tests: `TestSpanDatesFallbackKeepsListCampsInTheCalendar`, `TestSpanDatesIgnoredWhenExplicitDatesExist`. Result: 96 events / 28 skipped → **316 events / 0 skipped**.

### 3. Openings history silently lost half its rows

`icp_openings_history` is keyed by `(account, kind, entity_id, observed_at)` and stamps were RFC3339 second-precision. Two syncs landing in the same wall-clock second collapsed into one row via `INSERT OR REPLACE` — with no error anywhere. Verified: two consecutive syncs of 124 entities produced 124 history rows and `fill-rate` reported zero trends.

**Fix:** fixed-width nanosecond stamps (`icpStampFormat`), with `icpParseStamp` still accepting the older RFC3339 form. Fixed width also keeps range-query string comparisons correct against a formatted cutoff. Result: three back-to-back syncs → 81 rows, 3 distinct stamps, 27 trends.

### 4. Spec examples used the wrong command shape (caught pre-build)

Single-endpoint resources are **promoted**, so the real invocation is `locations <account>`, not `locations list <account>`. My spec examples used the un-promoted form and the generator rendered them verbatim into help text and the README. Corrected in the spec and in `research.json`, then regenerated.

### 5. A scoped `watch` poisoned the catalog for five other commands

`watch` recorded its observations through the same path as `sync`, which writes both openings history **and** a catalog snapshot under a new run id. But `watch --class 8357` sees exactly one entity. That one-entity snapshot became "the newest run", and every command that reads the latest snapshot — `drift`, `lint`, `calendar`, `opens-soon`, `compare` — then treated a single-class poll as the whole catalog.

Caught by checking a documented example against upstream truth: `fill-rate --programs 589` reported 1 entity where the API returns 13. `drift` was the worst case — comparing a full sync against a one-entity watch run reports every other class as removed.

**Fix:** added `RecordICPOpenings`, which appends history only and never opens a run or writes a snapshot. `watch` uses it; only `sync` writes snapshots.

**Verified:** after a full sync followed by a scoped watch — `drift` compares 124 entities with **0 phantom removals**, `fill-rate --programs 589` sees all 13 classes, `lint` still checks 124.

### 6. Three documented examples were wrong

Found by executing every command string in README.md and SKILL.md rather than trusting `validate-narrative`, which passed all three because `--dry-run` short-circuits before validation:

- `sync --resources classes,camps,locations` → `locations` is not a syncable resource; the command errors.
- The quickstart's `sync` step omitted the required positional account entirely.
- `watch scottsdalegymnastics --class 16010` → class 16010 belongs to `scaq`; `scottsdalegymnastics` returns "Class not found".
- `fill-rate scottsdalegymnastics --programs 246` → 246 is not a program at that account (real ids are 550, 567, 585–593).

All corrected at the source in `research.json` and regenerated, then re-executed against the live API to confirm.

## Deferred / not built

- **`--detail` sync mode.** Fetching `camps/{id}` per camp would populate `description`, `roomName`, `instructors`, and real `blocks`, enabling the detail-only lint rules on a full catalog. It is an N+1 walk (~124 requests for one mid-size gym) and was left out of the default sync rather than made implicit. The `Detailed` flag already gates behavior correctly for both cases.
- **Generator-side dead code.** `dogfood` reports 3 dead helpers (`collectionItemsForOutput`, `hasChangedLocalFlags`, `isDryRunResponseForClient`) in generated files. Left untouched: hand-editing generated code to satisfy a lint would create regen churn for no runtime benefit. Retro candidate.

## Generator limitations found

1. **No syncable resources when every endpoint has a positional path parameter.** A multi-tenant API keyed by a path slug gets no `sync`/`search`/`sql`/`stale`/`tail` and no `internal/syncer`, even though `internal/store` is emitted and the resources are perfectly syncable per tenant. This is the single biggest gap encountered; the entire local-store stack had to be hand-authored. Strong retro candidate.
2. **`pipeline_check` assumes a generated syncer.** It reports `sync uses generic Upsert only` and `sync_file_emitted: false` for a hand-authored sync that does call `Upsert`, because it looks for the generated file shape.
3. **`browser-sniff` path templating.** Static path segments were inferred as identifiers (`bookings/{booking_id}` for what is `{locationId}`, `levels/active/{active_id}`, `parties/create/{create_id}`) and the tenant slug was baked into every path instead of becoming a parameter, making the sniffed spec unusable as a generation input for a multi-tenant API.
4. **`browser-sniff` CAPTCHA false positive.** The org-settings field `recaptchaPublic` was classified as an observed CAPTCHA challenge, producing `reachability.mode: browser_required` at 0.9 confidence — which under Phase 1.9's matrix means HOLD. `probe-reachability` disagreed at 0.95 (`standard_http`), and all 13 captured entries were HTTP 200 with no challenge sentinel.
