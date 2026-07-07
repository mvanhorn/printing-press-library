Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

# Sutra Fitness CLI — Phase 3 Build Log

## Priority 0 — Foundation (data layer + sync)
The generator emitted typed SQLite tables (classes, reservations, clients, purchases,
referrals, locations, rooms) + FTS, but **sync was broken**: every Sutra path is scoped
under `/{partnerId}/`, so the generator left the sync registry empty
(`syncResourcePath`/`defaultSyncResources`/`knownSyncResourceNames` all empty), defaulted
the wrong cursor param (`after` vs `start_after`), and generated no dependent-resource
iteration. Fixed:
- Populated the registry (flat: locations, classes, clients, purchases, referrals) with
  `partnerId` resolved from `SUTRA_PARTNER_ID`.
- Fixed `determinePaginationDefaults` cursor param to `start_after` and added
  `nextStartAfterId` to the envelope cursor keys (Sutra's `pagination.nextStartAfterId`/
  `pagination.hasMore`).
- Added `updated_at_min` incremental since-param for the flat resources that support it.
- Hand-wrote dependent sync (`internal/cli/sutra_sync_deps.go`): reservations per class,
  rooms per location, injecting the parent foreign key (`classes_id`/`locations_id`) the
  typed upserts require. Partitioned flat vs dependent; folded counts into the summary.
- Added a `--dry-run`/verify short-circuit and a clear `SUTRA_PARTNER_ID required` error.
- Verified: paths resolve (`/test-partner/classes`), the client reaches the live API
  (real 401 with the documented envelope), dry-run exits clean. (Generator defect logged
  for retro.)

## Priority 1 — Absorbed (generator-emitted)
All 12 endpoints (locations, rooms, classes list/get, reservations list/create/cancel/
check-in, clients list/get, purchases, referrals) + framework sync/search/sql/analytics +
CSV/JSON/--select export. partnerId is a positional arg on endpoint commands;
SUTRA_PARTNER_ID powers sync + analytics.

## Priority 2 — Transcendence (8 hand-coded, all built)
Each reads the local typed tables and joins offline (the Sutra API has no reporting
endpoints). Shared helpers in `internal/cli/sutra_analytics.go`; each command in its own
hand-authored file with `// pp:data-source local`, dry-run guard, missing-mirror guard,
and sync-hint helpers.

| # | Command | Verified output (against seeded data) |
|---|---------|----------------------------------------|
| 1 | `scorecard` | Bob fill90/ns0/ci100, Alice fill70/ns40/ci60 ✓ |
| 2 | `no-shows --group-by instructor\|class\|client` | Alice 40%, Bob 0%; client Di 100%, Bob Ray 50% ✓ |
| 3 | `utilization --group-by class\|instructor\|timeslot\|location` | Alice 70%, Bob 90% (asc) ✓ |
| 4 | `expiring --within 7d --low-credits` | Bob Ray p2, reason expiring+low_credits ✓ |
| 5 | `churn --inactive-days 30` | Cy Tan (164d, no plan), Ed Fox (no check-in); new/removed excluded ✓ |
| 6 | `revenue --group-by type --compare-prior` | subscription 199.98, class_pack 350, prior delta correct ✓ |
| 7 | `referral-funnel` | 3 referrals, 2 signed_up, 2 purchased, 1 attended; top c1=2,c2=1 ✓ |
| 8 | `ltv` | Ann 299.99, Bob Ray 150, Di 99.99, Cy 25 ✓ |

## Bug found & fixed during behavioral testing
- `round2` truncated toward zero for negatives (`-25.0` → `-24.99`, `-100%` → `-99.99%`).
  Fixed with `math.Round`. Caught by the revenue prior-period comparison test.

## Scope deviations
- `--group-by location` dropped from `revenue` and `ltv`: purchases and clients carry no
  location field in the schema, so it is not derivable. `research.json` updated to match.
- `mark-no-shows` was killed at the brainstorm (API has no NO_SHOW write transition); not
  a stub.

## No stubs shipped.
