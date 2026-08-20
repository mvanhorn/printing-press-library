# RideWithGPS CLI Brief

## API Identity
- **Domain:** Cycling route planning, ride logging, and navigation (ridewithgps.com). Routes (planned, with turn-by-turn cue sheets), Trips (logged rides with GPS tracks + sensor data), Gear (bikes), Events (organized rides), Collections, POIs, Clubs.
- **Users:** Cyclists (road/gravel/MTB/touring), randonneurs, club organizers, bikepackers. Power users plan routes, push to head units (Garmin/Wahoo/Hammerhead), log rides, track gear mileage.
- **Spec:** Official OpenAPI 3.0.1 — `https://ridewithgps.com/api/v1/openapi.yaml` (2941 lines, 29 operations, 8 resource groups). This is the primary spec source.
- **Data profile:** Deeply nested, geo-heavy. Routes/Trips carry `track_points[]` (lat/lng/elevation/distance, + speed/HR/power/cadence on trips) and `course_points[]` (cue sheet). Distances/elevations in **meters**, durations in **seconds**. Pagination `page` + `page_size` (20–200), response root key named after resource, `meta.pagination` block.

## Reachability Risk
- **Low.** API is current, actively documented, stable-versioning promise (new endpoints added without version bump). Base `https://ridewithgps.com`, `/api/v1` namespace. No deprecation, no WAF/bot-protection observed (browser-sniff served everything cleanly).
- **Credentials are self-service** (account → developers tab → API client → `api_key`). Low onboarding friction.
- **Rate limits: UNDOCUMENTED.** No published RPM/RPS. → ship adaptive backoff, respect `Retry-After`/429 defensively.
- **Shared-route blindspot:** v1 list endpoints return only user-owned assets; shared routes need copying first. Note in export-all promises.
- Probe-safe endpoint: `GET /api/v1/users/current` (read-only, auth-gated).

## Auth Model (critical for generation)
- Two required headers: `x-rwgps-api-key` (client API key) + `x-rwgps-auth-token` (user token). OAuth2 also supported (preferred per docs) but the api_key+token path is what the spec + every wrapper use.
- **Login flow:** `POST /api/v1/auth_tokens` with `{user:{email,password}}` + `x-rwgps-api-key` header → returns `{auth_token}`. This is `auth login` (harvest token).
- **Spec gap:** the spec references a `basic_auth` security scheme but defines NO `components.securitySchemes`. Auth is modeled as required header *parameters* (`api_key`, `auth_token` $refs). **Phase 2 must enrich** the spec with proper apiKey securitySchemes + env vars (`RIDEWITHGPS_API_KEY`, `RIDEWITHGPS_AUTH_TOKEN`) so the generated config/doctor/client/README handle auth correctly instead of emitting ugly `--x-rwgps-api-key` flags.
- v1 API does NOT accept browser cookies (confirmed: cookie request → 401).

## Top Workflows
1. **Browse/list my routes & trips** with filters (distance, elevation, visibility, name) — the bread and butter.
2. **Bulk export** routes/trips to GPX/TCX/CSV (the #1 unmet need — see pain points).
3. **Sync my whole library offline** (incremental via `/sync.json?since=`) into a local store, then query/search it without hitting the API.
4. **Track gear mileage & maintenance** (derive per-bike distance from trips).
5. **Analyze training** — weekly/monthly distance/elevation/time, activity-type breakdown, personal records.

## Table Stakes (match every existing tool)
From official `ride-cli`, `boezzz/ridewithgps-mcp`, `pyrwgps`, `SteveWinward/.NET`, `jmoseley/ts`:
- List + get: routes, trips, events, collections, POIs, club members; current user.
- Route detail incl. track_points, course_points, POIs; trip detail incl. track_points + sensor metrics + gear.
- Sync (changed-since).
- Pagination, `--json`, auth setup.
- Write where the API allows: events (CRUD), POIs (CRUD + route association), route/trip delete, club-member update.
- Raw authenticated API passthrough (ride-cli's `api` command).

## Data Layer
- **Primary entities (SQLite tables):** routes, trips, gear, events, collections, points_of_interest, club_members, user.
- **Sync cursor:** `GET /api/v1/sync.json?since=<datetime>&assets=...` returns items changed/created/deleted since a datetime — ideal incremental sync. Store a `last_sync` cursor.
- **FTS5 search:** route/trip names, descriptions, localities; POI names; collection names.
- **Derived data (the moat):** gear→trip mileage joins, time-windowed ride aggregates, route dedup clusters (distance + start/end coords + bbox), personal records (max distance/elevation/speed from trip metrics).

## Codebase Intelligence
- **Auth:** `x-rwgps-api-key` + `x-rwgps-auth-token` headers (pyrwgps, boezzz MCP both use these). Login → `POST /api/v1/auth_tokens`. Env-var convention across wrappers: `RWGPS_API_KEY` / `RWGPS_AUTH_TOKEN`; canonical for this CLI: `RIDEWITHGPS_API_KEY` / `RIDEWITHGPS_AUTH_TOKEN` (keep `RWGPS_*` as a documented alias).
- **Data model:** Summary vs full schemas (RouteSummary→Route adds track_points/course_points/POIs/photos/collections; TripSummary→Trip adds track_points/gear/photos/collections). Gear embedded in Trip.
- **Legacy/web endpoints (browser-sniff confirmed):** `GET /{routes,trips}/{id}.{gpx,tcx,fit,kml}` (+`?sub_format=track`) for native file export; `GET /gear.json` legacy gear list. Not in v1 spec → implemented as hand-coded transcendence commands, not generated typed endpoints.

## Reachability Risk — competitor tooling
- `ride-cli` (OFFICIAL) is an **LLM-agent shell** hard-dependent on Claude Code — no structured/scriptable route/trip/gear/export subcommands, no offline mode. The bar to beat is low.
- Wrappers are stale (jmoseley ~5yr) or partial (SteveWinward read-only). `pyrwgps` is the only full-coverage wrapper.

## Product Thesis
- **Name:** `ridewithgps-pp-cli` (binary), display **"Ride with GPS"**. Tagline candidate: *"Your whole Ride with GPS library, offline — bulk export, gear mileage, and ride analytics no other tool ships."*
- **Why it should exist:** Every existing tool is read-only, online-only, or an AI shell. Cyclists can't bulk-export their routes, can't see gear mileage outside the dashboard, can't query their library offline. A fast, scriptable, agent-native CLI with a local SQLite mirror turns the API's rich track data into the things power users actually want: export everything, dedup the library, track chain wear, see this month's climbing — none of which any competitor does.

## Build Priorities
1. **Foundation:** SQLite data layer for all 8 entities + `sync` (incremental via `/sync.json`) + FTS5 search + SQL passthrough.
2. **Absorb:** every list/get for routes/trips/events/collections/POIs/club-members/user; writes for events/POIs (CRUD), route/trip delete, club-member update; polylines; auth login; raw `api` passthrough.
3. **Transcend:** bulk `export` (GPX/TCX/CSV from track_points + `--native` passthrough); `gear` mileage + maintenance-due; route `dedup`; `stats` (training aggregates); `records` (PRs); library `audit`.
