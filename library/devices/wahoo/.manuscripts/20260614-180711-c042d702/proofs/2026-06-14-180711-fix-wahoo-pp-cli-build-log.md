# Wahoo CLI — Phase 3 Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Built
### Priority 0/1 (generated)
- All 28 endpoints as typed commands (user, workouts, workout_summary, workout_file_uploads, plans, routes, power-zones, permissions).
- OAuth2 Authorization Code + PKCE auth (`auth login/logout/setup/status`), config, doctor.
- Framework: sync (rate-limit-aware), search, sql/analytics, agent-context, MCP server (cobratree mirror), import, workflow, which, profile, feedback.

### Priority 2 (hand-coded transcendence — all 6)
| # | Command | File | Behaviorally verified (demo DB) |
|---|---------|------|-------------------------------|
| 1 | `backup` | backup.go | 3 records written; failed FIT download surfaced in failures[] + stderr (partial-failure accounting). Added `--db` flag (was hardcoded). |
| 2 | `load` | load.go | CTL/ATL/TSB series; TSB -21.7 "building", FTP 265 from power-zones, 14-day truncation. |
| 3 | `digest` | digest.go | --days 7 windowed: 125km/4.25h/1500m/3180kJ; avg power 217.5 over power rides only. |
| 4 | `bests` | bests.go | Correct record per metric (power 240W, distance 90km, ascent 1200m, work 2100kJ, duration 3h); --metric filter. |
| 5 | `ftp-history` | ftp_history.go | 250→265 (+15W), watts/kg 3.45→3.66 @72.5kg, sorted ascending. |
| 6 | `routes find` | routes_find.go | distance band, max-ascent, and haversine proximity filters all correct (Mountain at 60km excluded at r=30, included at r=80). |

Shared: `wahoo_analysis.go` (NULL-safe `parseLooseFloat` for string-typed summary metrics, `parsedWorkout` model, `estimateRideLoad`, `computeLoadSeries`, haversine, power-zone/user readers, `mirrorMissing` guard).

## Conventions applied
- All 6 are hand-authored files (header stripped of "DO NOT EDIT") so regen-merge treats them as NOVEL.
- Verify-friendly RunE: dryRunOK short-circuit, missing-mirror guard (empty JSON + sync hint), `pp:data-source local`.
- backup adds `cliutil.IsVerifyEnv` (no file writes under verify) + `cliutil.IsDogfoodEnv` (curtail to 1 item).
- `--db` flag on every command; `--json/--select/--agent` route through `printJSONFiltered`.
- `mcp:read-only` on the 5 pure-read commands; omitted on `backup` (writes user-visible files).
- Real table-driven tests per feature (parse, load math, bests, digest, ftp progression, haversine + temp-store findRoutes, httptest downloadTo). `go vet` + `go test ./internal/cli/...` green.

## resource_type naming (confirmed from store.go)
workouts → `workouts`, routes → `routes`, power zones → `power-zones` (hyphen), user → `user`. workout_summary embedded in each workout's data.

## Intentionally deferred / honest limits
- `bests`/`load` use per-ride summary AVERAGES (the API exposes no per-second streams) — no mean-maximal power curve. Documented in command Long help.
- Live OAuth + live smoke testing deferred: no approved Wahoo app/client_id available.
