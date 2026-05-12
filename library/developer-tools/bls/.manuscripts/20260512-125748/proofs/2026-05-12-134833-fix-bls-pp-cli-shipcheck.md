# bls-pp-cli Shipcheck Report

## Verdict: **SHIP** (all 6 legs pass)

```
  LEG               RESULT  EXIT      ELAPSED
  dogfood           PASS    0         2.116s
  verify            PASS    0         3.66s
  workflow-verify   PASS    0         17ms
  verify-skill      PASS    0         185ms
  validate-narrative PASS   0         397ms
  scorecard         PASS    0         77ms
```

## Scorecard: **76/100 — Grade B** (≥65 ship threshold)

Strong dimensions (10/10): Output Modes, Auth, Error Handling, Doctor, Agent Native, MCP Quality, Local Cache, Sync Correctness, Path Validity.

Weak dimensions (call-outs for future polish):
- `mcp_token_efficiency`: 4/10
- `mcp_remote_transport`: 5/10
- `workflows`: 4/10
- `insight`: 2/10
- `auth_protocol`: 4/10
- `cache_freshness`: 5/10

Notes:
- `MCP: 5 tools (0 public, 5 auth-required) — readiness: full`. All endpoints expose the API as authenticated; we list auth as optional but it's recommended for >25 queries/day.
- Domain Correctness scored 29/40 (path validity 10, sync correctness 10, type fidelity 4/5, dead code 4/5, auth protocol 4/10, data pipeline 7/10, live API N/A).

## Top issues found and fixed

1. **`series batch --ids` failed to parse comma-separated input.** The generated body field used `type: array` and emitted a JSON unmarshaler. Patched both the spec (type → string) and the generated `series_batch.go` to accept both forms (JSON array OR CSV). Also added BLS-specific body-key injection because BLS only honors `registrationkey` in the JSON body for POST.

2. **`sync --dry-run` exited 1 with "missing id for series" / "missing id for surveys".** The generator's typed `UpsertSeries` / `UpsertSurveys` functions used the generic `extractObjectID` lookup table (`id`, `Id`, `uuid`, ...) which doesn't match BLS records' `seriesID` / `survey_abbreviation` keys. Added `extractBLSResourceID` in store.go that consults the spec-declared primary key first, with a special-case for the client's `{"dry_run": true}` sentinel so dry-run sync preview is non-fatal.

3. **Narrative recipes used `&&` chaining.** `validate-narrative --full-examples` passes the command line directly to the binary; `&&` is a shell construct and parsed `--since` as belonging to the first command. Replaced the chained recipe with a single-command alternative.

4. **`snapshot macro --calc` was invalid.** Snapshot already sets `calculations: true` automatically (it's the whole point of the curated batch). Removed `--calc` from the recipe.

5. **Live POST commands were silently demoted to the unauthenticated 10-year tier even with `BLS_API_KEY` set.** BLS only honors the registration key when it's in the JSON body for POSTs, not the query string. Added `injectRegistrationKey` helper used by `snapshot`, `series compare-sa`, `inflation adjust`, and patched `series_batch.go` to inject the body key too. The generator emits the key in the query because the spec declares `in: query`; the body-injection is a BLS-specific override.

6. **`footnotes decode` Use spec said `<code...>` (cobra treats as one positional).** Changed to `<code>...` so cobra recognizes it as variadic.

## Before/After

- verify pass rate: PASS (auto-fix loop converged on first pass after fixes)
- scorecard total: 76/100 (Grade B)
- dogfood novel_features check: planned 6 / found 6 / missing []

## Known limitations (deferred; not ship-blockers)

- `auto-sync` mirrors only what `/timeseries/popular` and `/surveys/` return (small lists). The richer ~120-entry curated series catalog and the 2026 release calendar are seeded from embedded static data via `openBLSStore`, not via sync.
- `series extremum --since 1990` with a window >20 years will scan only the most recent 20-year window BLS will return per call; the command surfaces this honestly in the `window_start`/`window_end` fields rather than silently fetching a smaller range.
- `--calc`/`--annual-avg` on the auto-generated `series get` work via the query-key (which BLS honors for GET). `series batch` body-key injection was added explicitly.
