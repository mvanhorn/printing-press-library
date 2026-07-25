# irail-pp-cli — Phase 3 Build Log

Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

## Priority 0 — foundation (generated)
- SQLite store, sync, search (FTS), sql, doctor, learn loop: PASS
- Verified live: 714 stations + 31 disruptions synced; local FTS returns hits.

## Spec-level fixes applied before build
1. `cache.enabled` removed — pre-read sync fired against board/route/train, which
   require params the syncer does not supply (HTTP 500 / 400).
2. `format=json` pinned into every endpoint path — iRail ignores the Accept
   header and defaults to XML; the generated syncer builds its query map from
   scratch and does not apply spec param defaults.
3. `--format` flag dropped (foot-gun: the parser expects JSON).

## Priority 1 — absorbed (24 rows)
- 7 generated endpoint commands: `board`, `route`, `stations`, `disruptions`, `logs`,
  `train get`, `train composition`. Five single-endpoint resources were promoted by
  the generator to clean top-level paths (`board`, not `board list`); spec examples,
  research.json narrative and the manifest were corrected to the promoted form.
- 3 hand-authored: `stations search`, `saved add|list|remove`, `occupancy report`.
- Behaviours: typed coercion, human dates, station resolver, enum validation, `--lang`.

## Priority 2 — transcendence (7 rows, all hand-coded)
All 7 shipped: `transfer-risk`, `punctuality`, `disruptions route`, `observe`,
`leave-by`, `stations facilities`, `changes`.

Manifest transcendence rows: 7 planned, 7 built.

## Supporting packages built
- `internal/irailref/` — embedded stations.csv + facilities.csv (166 KB) with a
  folding resolver over 4-language names, 566 telegraph codes and TAF/TAP codes.
  Marked `pp:novel-static-reference`. 6 test functions.
- `internal/store/irail_migrations.go` — `irail_observations` + `irail_saved_routes`,
  batch insert with INSERT OR IGNORE idempotency, NULL-safe reads.
- `internal/cli/irail_helpers.go` — Belgian-timezone clock (embedded tzdata),
  human date/time parsing, iRail string→typed coercion, nested-map access that
  tolerates iRail collapsing single-element arrays to bare objects.
- `internal/cli/irail_wiring.go` — regen-safe `registerNovelCommand` hook.

## Bugs found and fixed during the build
1. `cache.enabled: true` triggered a pre-read sync on `board`/`route`/`train`,
   which require params the syncer does not supply -> HTTP 500/400. Cache removed.
2. iRail ignores `Accept: application/json` and defaults to XML; the syncer does
   not apply spec param defaults. Fixed by pinning `format=json` into each path.
3. `stations facilities` / `disruptions route` were registered twice (the generator
   already attaches them to promoted parents). Removed from the local hook.
4. `matched_stations` listed one physical station twice under its Dutch and English
   names. Fixed by keying route stations on resolved station URI.
5. Spec/narrative examples used non-promoted forms (`board list`) that silently
   ignored the extra argument. Corrected everywhere.
6. Help text referenced `route plan` / `disruptions list`, which do not exist.

## Deferred / not built
- Nothing from the approved manifest. No stubs.
- `/v1/logs` is covered but returns `[]` upstream; documented rather than hidden.

## Generator limitations observed (retro candidates)
- The syncer builds its query map from scratch and ignores spec param defaults,
  so any API needing a constant query param (`format=json`) breaks under `sync`.
- Cache freshness pre-read assumes every resource is syncable; resources with
  required params produce upstream errors.
- `root.go` renders the CLI title from the slug (`Irail`) rather than the
  research-authored `narrative.display_name` (`iRail`).
