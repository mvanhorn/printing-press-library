# Luma CLI — Phase 5 Acceptance (Full Dogfood, live)

- Level: Full Dogfood (live, no-auth API)
- Matrix: 76 tests
- Result: **76/76 passed, 0 failed** → Gate: **PASS**

## Fixes applied inline (first run failed 7/77)
All 7 were fixture/placeholder mismatches against the live API, not behavioral bugs (every command works with real inputs, verified separately):

| Failure | Fix | Type |
|---|---|---|
| `events get evt-XXXX` → 404 (×2) | `pp:happy-args=--event-id=evt-...` annotation | CLI fix (fixture) |
| `places calendars --place-id <UUID>` → 404 (×2) | real example + `pp:happy-args=--place-id=discplace-...` | CLI fix (fixture) |
| `places get` (no args) → API 400 (×2) | local guard: require `--city`/`--place-id` → usageErr instead of opaque 400 | **CLI fix (real UX)** |
| `tail __invalid__` error-path | `pp:no-error-path-probe` (poll command tolerates initial 404 by design) | CLI fix (annotation) |

Root cause of the re-run miss: `dogfood --live` ran the **stale staged binary** (`build/stage/bin/`), which it does not auto-rebuild (scorecard does). Rebuilding the staged binary made all fixes take → 76/76.

## Behavioral verification (novel features, live)
- `agenda --city sf --category cat-ai` → merged 2 sources, 0 failures ✓
- `near --lat 37.77 --lng -122.42 --radius-km 3` → 25 events ≤3km, nearest 0.3km ✓
- `ics --city sf` → valid RFC5545 VCALENDAR ✓
- `watch --city sf --category cat-ai` → baseline → "no changes" diff ✓
- `calendars compare cal-A cal-B cal-C` → ranked by upcoming events ✓

No PII in this report (no auth, public discovery data only).

## Printing Press issues for retro
1. ID-fallback list omits `api_id` (silent empty store for api_id-keyed APIs).
2. `dogfood --live` does not auto-rebuild the staged binary; fixes to sources are invisible until the staged binary is rebuilt (scorecard auto-rebuilds, dogfood does not).
