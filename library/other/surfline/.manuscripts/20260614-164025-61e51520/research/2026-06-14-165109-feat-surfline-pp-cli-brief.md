# Surfline CLI Brief

## API Identity
- Domain: Surf forecasting. Reverse-engineered JSON API at `https://services.surfline.com` (the backend powering surfline.com + apps). No official/public OpenAPI spec exists.
- Users: Surfers doing the dawn-patrol "is it worth paddling out" decision; multi-spot scanners; data-distrusters who want raw swell/wind/buoy numbers over the star rating.
- Data profile: Spots (24-char IDs), subregions/regions/geonames (taxonomy tree), per-spot forecasts (wave+swells, wind, tides, weather, conditions, rating, sunlight), cameras (live + rewind clips), buoys.

## Reachability Risk
- Decision: PASS (browser_http row). `probe-reachability` on `/kbyg/spots/forecasts/wave` returned stdlib=403 (Cloudflare) but **surf-chrome=200 application/json** (confidence 0.85). Runtime is settled: ship **Surf transport** (Chrome TLS fingerprint). No clearance cookie, no browser capture required.
- Tier/permission hints: None from 4xx body (the 403 is a generic Cloudflare bot-block, not a tier message). Premium tier (LOTUS) gates forecast horizon >6 days and premium cams.
- Probe-safe endpoint used: `GET /kbyg/spots/forecasts/wave?spotId=...&days=1` (read-only).
- Note for generation: must pass `--traffic-analysis` hint or rely on probe; set `http_transport: browser-chrome` in spec so Surf transport is emitted.

## Top Workflows
1. **Dawn-patrol check** — for one spot, see swell (height/period/direction) + wind (speed/direction/offshore) + tide stage + rating together. Period and wind are the real "tells," not the star count.
2. **Multi-spot scan & rank** — compare a handful of favorite spots side-by-side, pick today's best. Surfline's real comparison tool is web-only (not in the apps); the favorites list drops swell/wind/tide.
3. **Tide window planning** — when is the next high/low, does it line up with the spot's working window.
4. **Cam ground-truth** — pull the live cam still + rewind clip URL as the final gut-check.
5. **History/journal + scriptable alerts** — log daily forecast (+ later actuals), run arbitrary `swell>X AND wind<Y AND tide∈window` rules. Official alerts are Premium-AND-iOS-only; demand proven by DIY tools (SurfScrape).

## Table Stakes (must match every competitor)
- Spot search by name → spotId (`/search/site`)
- wave/swells, wind, tides, weather, conditions, rating forecasts (`/kbyg/spots/forecasts/*`)
- combined forecast (`/kbyg/spots/forecasts`)
- multi-spot batch (`POST /kbyg/spots/batch`)
- region conditions (`/kbyg/regions/forecasts/conditions`)
- taxonomy browse (`/taxonomy`)
- nearby buoys (`/kbyg/buoys/nearby`)
- premium token support via `accesstoken` query param for 7–17 day horizon
- units overrides (`units[swellHeight]` etc.), days, intervalHours

## Data Layer
- Primary entities: spots, subregions/regions/geonames (taxonomy), forecast snapshots (wave/wind/tide/rating/conditions keyed by spotId+timestamp), cameras, buoys, favorites (local), forecast-history log (local), alert rules (local).
- Sync cursor: per-spot `runInitializationTimestamp` from `associated`; forecast points keyed by unix `timestamp`.
- FTS/search: spot name + breadcrumb/location FTS so `search` works offline after a taxonomy sync.

## Auth
- Type: optional bearer/api-key. Basic forecast/search/taxonomy/batch/buoys work with **NO auth** (up to 6-day horizon). Token unlocks 7–17 day forecasts + premium cams.
- Token delivery (source-verified): lowercase `accesstoken` query param on GETs. (Bearer header also works generally; wrappers use the query param.)
- Acquisition: `POST /trusted/token` password grant with static community client (`5c59e7c3f0b6cb1ad02baf66`). CLI will support `auth login` (email/pw → token) and `auth set-token`.
- Env var: no community standard. We define `SURFLINE_ACCESS_TOKEN`.

## Product Thesis
- Name: **surfline-pp-cli** ("Surfline, scriptable")
- Why it should exist: The one place to scan many spots at once from the terminal, with a local database that the official apps don't give you — offline spot search, a forecast-history journal, and cron-able multi-condition alerts. No ads, no 5-checks-a-week free-tier cap on the data you pull, raw numbers for people who distrust the rating.

## Build Priorities
1. Data layer: spots/taxonomy + forecast snapshot tables, sync, FTS search, SQL.
2. Absorb every wrapper feature: search, all forecast resources, combined, batch, region, taxonomy, buoys, units/days/interval flags, token auth.
3. Transcend: multi-spot rank, dawn-patrol one-shot, tide-window, forecast-history journal, scriptable alert rules, cam-rewind fetch.
