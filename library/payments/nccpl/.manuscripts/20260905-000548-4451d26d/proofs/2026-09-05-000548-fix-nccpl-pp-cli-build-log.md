# NCCPL CLI build log

Manifest transcendence rows: 7 planned, 7 built. All approved rows ship; no stubs.

## Pre-build state
- Generated tree builds clean after one hand-authored workaround file.
- Generator bug worked around: `internal/cli/nccpl_truncate.go` supplies `truncateJSONArray`,
  which the endpoint template calls but never defines (retro candidate 1).
- Verified after generation: Surf transport present in `internal/client/client.go`, composed
  cookie auth over `cf_clearance` / `nccpl-session` / `XSRF-TOKEN` present in
  `internal/cli/auth.go:89`, `X-XSRF-TOKEN` header wired in the client.
- 42 generated endpoint commands across 21 resources.

## Data layer: hand-written (generator emits none)
The generator classified zero resources as syncable because every NCCPL data endpoint is
`POST` with a required date; the syncable profile requires GET list endpoints with no required
params (confirmed by comparing against the published `psx` spec, whose `symbols list` is
`GET /symbols` with `params: []`). Renaming endpoint keys `data` -> `list` did not change the
classification. Store schema and sync path are therefore hand-authored. Approved as revised
scope before build.

## Built

### Data layer (hand-written)
- `internal/store/nccpl_schema.go` - `nccpl_obs` (resource, date, row_key, payload, observed_at)
  and `nccpl_coverage` (resource, date, row_count, fetched_at). One generic row store rather
  than 20 typed tables: every NCCPL payload is a flat object belonging to exactly one
  (resource, date), so one shape serves panel export, cross-resource joins, universe
  reconstruction and per-field change detection without 20 near-identical schemas drifting.
  `SaveNCCPLDate` writes observations and the coverage entry in ONE transaction so neither half
  can exist without the other. `observed_at` is deliberately not updated on conflict: it records
  first observation, which is what establishes ex-ante availability.
- `internal/cli/nccpl_resources.go` - registry of all 20 data resources carrying the three
  date encodings and five response envelopes read out of NCCPL's own page JS.
- `internal/cli/nccpl_sync.go` - resumable per-session backfill. Always requests one session
  per call even on the range endpoints, because the flow rows carry no date of their own and a
  wider window returns a single aggregate rather than per-session rows.

### Transcendence commands (7/7)
| # | Command | File |
|---|---------|------|
| 1 | `verify` | internal/cli/nccpl_verify.go |
| 2 | `coverage` | internal/cli/nccpl_coverage.go |
| 3 | `panel` | internal/cli/nccpl_panel.go |
| 4 | `universe` | internal/cli/nccpl_universe.go |
| 5 | `leverage` | internal/cli/nccpl_leverage.go |
| 6 | `risk-changes` | internal/cli/nccpl_riskchanges.go |
| 7 | `contract-check` | internal/cli/nccpl_contractcheck.go |

Registered via `internal/cli/nccpl_register.go` (a preserved novel hook, not a root.go edit).

### Bug workarounds applied
- `internal/cli/nccpl_truncate.go` - supplies `truncateJSONArray`, emitted-but-undefined by the
  generator (retro candidate 1). Without it the module does not compile.
- `internal/cli/nccpl_auth_cookies.go` + a one-line edit at `internal/cli/auth.go:293` -
  URL-decodes the Laravel `XSRF-TOKEN` cookie before it is composed into the `X-XSRF-TOKEN`
  header (retro candidate 2). The generated `composeAuthFromCookies` substitutes raw, which
  would 419 every POST. The `Cookie:` header still receives the encoded form, which is correct.

### Tests
- `internal/cli/nccpl_logic_test.go` - 11 table-driven tests covering the three date encodings,
  the fipi-normal/lipi-normal envelope asymmetry, weekend skipping, row-key collision safety,
  numeric parsing, net-summing with pre-summed FN/LN rows excluded, cookie decoding, resource
  selection, and that the dogfood-curtailed contract-check set still covers every encoding and
  envelope.
- `internal/store/nccpl_schema_test.go` - 4 tests covering round-trip, that an empty fetch is
  still recorded as coverage, that re-sync refreshes payload without moving `observed_at`, and
  date-range filtering.

### Intentionally deferred
- Wide-window aggregate sync for the range endpoints. Their rows carry no date, so a wide window
  collapses to one aggregate; the generated `fipi data` / `lipi data` commands already expose
  arbitrary ranges for that use. Sync deliberately stays per-session.
- Pakistani market holiday calendar. A holiday returns zero rows and is recorded as
  fetched-and-empty, which is the honest representation and needs no hardcoded calendar.
