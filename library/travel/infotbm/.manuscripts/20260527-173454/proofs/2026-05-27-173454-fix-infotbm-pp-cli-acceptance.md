# Acceptance Report: infotbm

## Summary
- **Level:** Full Dogfood
- **Tests:** 104/104 passed (49 skipped — no positional arg or no-error-path-probe)
- **Gate:** PASS

## Auth Context
- Type: API key (`INFOTBM_API_KEY=opendata-bordeaux-metropole-flux-gtfs-rt`)
- SIRI endpoints use `AccountKey` query parameter (fixed in client.go)
- GTFS/REST endpoints use `apiKey` query parameter

## Fixes Applied During Phase 5 (3 fixes)

### Fix 1: SIRI LineRef resolution — LineCode + array LineName parsing
- **File:** `internal/cli/lines_frequency.go` (resolveSIRILineRef + extractSIRITextValues helper)
- **Root cause:** SIRI `LineName` is an array of `{"value":"...", "lang":"fr"}` objects, not a plain string. The short code (e.g., "A") lives in `LineCode`, not `LineName`.
- **Fix:** Updated `resolveSIRILineRef` to check `LineCode` first (string or object form), then use shared `extractSIRITextValues` helper for `LineName` (handles string, object, and array forms).
- **Type:** CLI fix

### Fix 2: Lines stops — complete rewrite to use estimated-timetable
- **File:** `internal/cli/lines_stops.go` (fully rewritten)
- **Root cause:** The `lines-discovery.json` endpoint has NO stop point data — `Destinations` only contains `PlaceName` and `DirectionRef`. The `stop-points-discovery.json` endpoint returns 404 on this Bordeaux SIRI API.
- **Fix:** Rewrote to use `estimated-timetable.json`, extracting ordered stop sequences from vehicle journey EstimatedCalls. Finds the journey with the most stops (complete run), deduplicates while preserving order.
- **Type:** CLI fix

### Fix 3: Realtime stop Example string
- **File:** `internal/cli/realtime_stop.go`
- **Root cause:** Example used synthetic UUID `550e8400-e29b-41d4-a716-446655440000` which doesn't exist on the API. Dogfood extracts args from Example for happy_path tests.
- **Fix:** Updated Example to use `BPGALL` (a real stop code).
- **Type:** CLI fix

## Printing Press Issues (for retro)
1. Dogfood runner ignores `pp:happy-args` annotation when Example string is present — parses Example for test args instead of using the annotation override. This forced changing the Example text to fix the test. The annotation should take priority.

## Test Matrix Highlights
- All 8 transcendence commands pass (schedule diff, trips last-departure, trips reroute, alerts impact, lines stops, schedule changes, trips plan, lines frequency)
- All generated endpoint commands pass (agencies, alerts, fares, feed-info, realtime stop, routes, stops)
- Workflow commands pass (archive, status)
- Framework commands pass (doctor, sync, search, sql, context, reconcile, stale, export, import, deliver, feedback)
