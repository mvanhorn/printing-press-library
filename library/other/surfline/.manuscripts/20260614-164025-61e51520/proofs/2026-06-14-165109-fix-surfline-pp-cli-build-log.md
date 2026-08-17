# Surfline CLI Build Log

Manifest transcendence rows: 7 planned (hand-code), 7 built. Phase 3 passed (8/8 novel features incl. offline search).

## Built — Priority 0/1 (data layer + absorbed, generator-emitted)
- 15 typed endpoint commands across spots/regions/taxonomy/buoys: spots find (online search), wave, wind, tides, weather, conditions, rating, sunlight, forecast (combined), details, report, batch; regions conditions; taxonomy get; buoys nearby.
- Surf (browser-chrome TLS) transport clears Surfline's Cloudflare gate (probe-reachability: browser_http, confirmed 200 JSON live).
- Optional api_key auth via `accesstoken` query param (SURFLINE_ACCESS_TOKEN); doctor reports it INFO/optional. Basic data works with no token.
- Clean unit flags (swell-units/wind-units/temp-units/tide-units) over bracketed wire keys (units[swellHeight] etc.).

## Built — Priority 2 (transcendence, hand-code)
1. `now <spotId>` — joins wave+swells+wind+rating+tides per hour into a paddle/no-paddle readout. Parallel fan-out with partial-failure accounting. Validated live (Pleasure Point).
2. `rank <spotId...>` — parallel per-spot scoring on rating+optimalScore, sorted best-first. Validated live.
3. `windows <spotId>` — contiguous daylight blocks where surf/swell/wind optimalScore clear the bar (intersected with sunlight). `--primo`. Validated live.
4. `raw <spotId>` — pipe-friendly numeric dump (min/max/optimalScore/humanRelation, swell components, wind directionType/gust). `--select` dotted paths. Validated live.
5. `buoy-check <spotId>` — observed nearby-buoy swell next to forecast swell; haversine distance, nearest-first. Validated live (coords from wave associated.location; buoys from /kbyg/buoys/nearby latestData).
6. `alert add|list|run` — local SQLite rule engine (swell/period/wind/offshore/rating thresholds); `alert run --fail-on-match` exits 8 for cron. Validated live.
7. `journal log|show <spotId>` — snapshots forecasts to SQLite (the API has no personal history); offline `search` indexes journaled spots. Validated live.

## Design decisions / deviations
- **search**: framework search was replaced by a novel "search" command (generator quirk — see retro). Reimplemented as offline lookup over journaled spots (surfline_journal), since Surfline has no bulk-listable resource to sync.
- **sync**: no-op with honest messaging. Surfline has no parameterless list endpoint (every endpoint needs spotId/subregionId/query), so a blanket sync has nothing to pull. Offline data comes from `journal log`.
- **regions subregionId**: the forecast subregionId (spot.subregion._id from `spots report`) is a different namespace than taxonomy/search ids. Flag help + example corrected to a real working id (58581a836630e24c44879011).

## Durable extensions (survive regen)
- internal/cli/surfline_forecast.go, surfline_store.go (hand-authored, no generated header)
- internal/store/surfline_migrations.go (EnsureSurflineTables; journal + alert tables)
- Novel command files rewritten as hand-authored units.

## Hand-patched generated files (noted for retro)
- internal/cli/doctor.go — auth.Optional not propagated to env-var requiredness; patched env-var branch ERROR→INFO.
- internal/cli/sync.go — no-op guard for no-bulk-listable-resources.
- Example spotId/subregionId fixtures (fake UUID → real ids) across spots_*/regions for dogfood + UX.
