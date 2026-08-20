# intervals.icu CLI Brief

## API Identity
- Domain: Endurance-training analytics platform (cycling, running, swimming, triathlon). Free/donation-supported; the analytics layer competitors charge for (TrainingPeaks, WKO5).
- Users: Athletes and coaches who upload activities (from Garmin/Strava/Wahoo), track wellness (HRV, resting HR, sleep, weight, fatigue/soreness), plan structured workouts, and analyze power/pace/HR curves and fitness trends (CTL/ATL/TSB).
- Data profile: Per-athlete time-series. Activities (52 endpoints) with rich derived analytics (power/pace/HR curves, histograms, best efforts, streams, intervals), daily Wellness records, calendar Events (planned workouts/notes/plans), a Workout Library (with Zwift/erg/mrc/fit export), Gear, Sports settings/zones, Routes, Weather.

## Reachability Risk
- None. Public documented REST API at https://intervals.icu/api/v1. HTTP Basic auth (username `API_KEY`, password = personal key from /settings).
- Spec advertises `http://intervals.icu/` server (wrong); real origin is HTTPS. Base URL override to https://intervals.icu required at generation. (Systemic: generator should auto-upgrade http servers — retro candidate.)

## Top Workflows
1. Sync training history locally, then query/search/SQL it offline (no existing tool does this).
2. Daily wellness logging + trend review (HRV, resting HR, sleep, fatigue).
3. Fitness/form tracking: CTL (fitness), ATL (fatigue), TSB/form trend over time.
4. Power/pace/HR curve analysis: best efforts over arbitrary date ranges; season-over-season PR comparison.
5. Calendar: see upcoming planned workouts, what was missed, mark workouts done.
6. Workout library management + export to Zwift (.zwo) / trainer (.erg/.mrc/.fit).

## Table Stakes (from absorbed tools)
- Activity list/get/search, multi-activity fetch, CSV export.
- Wellness get/list/update (single + bulk).
- Events list/get/create/update/delete, bulk, apply-plan, mark-done.
- Power/pace/HR curves + histograms, best efforts, streams, intervals.
- Workout library + folders/plans + workout file export/import.
- Athlete profile, sport settings/zones, gear, routes, weather, chats.

## Data Layer
- Primary entities: activities, wellness, events, workouts (library), gear, sport_settings, routes, athlete (profile).
- Sync cursor: date-range (activities/wellness/events keyed by ISO date; `oldest`/`newest` query params).
- FTS/search: activity name + type + tags; workout name + tags; event name.

## Codebase Intelligence
- Auth: HTTP Basic, username literal `API_KEY`, password = personal key. Community tools also pass `ATHLETE_ID` (path param `{id}`, accepts `0` or `i<num>` for self). Bearer/OAuth2 also supported for third-party apps (out of scope for personal CLI).
- Rate limiting: generous for personal keys; no documented hard limit.

## User Vision
- Print a personal-use intervals.icu CLI from the official OpenAPI spec, API-key auth, read-first, with a local store for offline analysis.

## Product Thesis
- Name: intervals.icu CLI (binary: intervals-icu-pp-cli)
- Why it should exist: Every existing tool is an MCP server or thin wrapper that proxies live calls. None persists your training history locally, none gives you offline SQL/search across seasons, and none computes fitness/form trends from the CLI. This is the first agent-native intervals.icu CLI with a local SQLite training database.

## Build Priorities
1. Data layer + sync for activities, wellness, events, workouts, gear (Priority 0).
2. Full absorbed endpoint surface — all 115 endpoints as typed commands (Priority 1, generator-emitted).
3. Transcendence (Priority 2, hand-coded): fitness/form trend, season curve comparison, wellness↔load correlation, "what did I miss" calendar window, gear mileage rollup.
