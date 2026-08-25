# RideWithGPS CLI — Ecosystem Absorb Manifest

Sources scanned: official `ride-cli` (LLM-agent shell), `boezzz/ridewithgps-mcp` (8 read-only tools), `ckdake/pyrwgps` (fullest wrapper), `SteveWinward/RideWithGPS` (.NET, partial), `jmoseley/ridewithgps-client` (TS, stale), v1 OpenAPI (29 ops). No Claude skill/plugin exists (open lane).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List routes (filter distance/elevation/visibility/name/archived) | v1 listRoutes; boezzz get_routes | (generated endpoint) routes list | Offline after sync, `--json`/`--select`/`--csv`, typed exit codes |
| 2 | Route detail (track_points, course_points, POIs, photos) | v1 getRoute; boezzz get_route_details | (generated endpoint) routes get | Cached in SQLite, `--compact` high-gravity fields |
| 3 | Delete route | v1 deleteRoute | (generated endpoint) routes delete | `--dry-run`, idempotent |
| 4 | Route polyline | v1 getRoutePolyline | (generated endpoint) routes polyline | Composable with map tooling |
| 5 | List trips (filters incl. archived/stationary) | v1 listTrips; boezzz get_trips | (generated endpoint) trips list | Offline, filterable, agent-native |
| 6 | Trip detail (track_points + speed/HR/power/cadence + gear) | v1 getTrip; boezzz get_trip_details | (generated endpoint) trips get | Cached; powers stats/records/gear |
| 7 | Delete trip | v1 deleteTrip | (generated endpoint) trips delete | `--dry-run` |
| 8 | Trip polyline | v1 getTripPolyline | (generated endpoint) trips polyline | Composable |
| 9 | Events list/get/create/update/delete | v1 events CRUD; pyrwgps | (generated endpoint) events {list,get,create,update,delete} | Full CRUD with `--dry-run`; club-event bulk ops scriptable |
| 10 | Collections list/get + pinned | v1 collections; pyrwgps | (generated endpoint) collections {list,get,pinned} | Offline, searchable |
| 11 | POIs CRUD + route associate/disassociate | v1 POIs; pyrwgps | (generated endpoint) points-of-interest {list,get,create,update,delete,associate,disassociate} | Full write surface no MCP exposes |
| 12 | Club members list/get/update | v1 members; pyrwgps | (generated endpoint) members {list,get,update} | Roster scripting |
| 13 | Current user | v1 getCurrentUser; boezzz get_current_user | (generated endpoint) users current | `doctor` uses for auth check |
| 14 | Auth token harvest (email/password → auth_token) | v1 createAuthToken; pyrwgps authenticate | (generated endpoint) auth-tokens (POST /api/v1/auth_tokens.json) | `auth-tokens --user-email --user-password` returns the token; api key + token set via env or `auth set-token` |
| 15 | Incremental sync (changed-since) | v1 getSyncInfo; boezzz sync_user_data | (behavior in ridewithgps-pp-cli sync) framework sync via /sync.json cursor | Populates local SQLite mirror; cursor persisted |
| 16 | Offline FTS search | (our addition) | (behavior in ridewithgps-pp-cli search) FTS5 over routes/trips/POIs/collections | No competitor has offline search |
| 17 | Raw authenticated API passthrough | ride-cli `api` | ridewithgps-pp-cli api | Scriptable, no LLM dependency (ride-cli requires Claude Code) |
| 18 | SQL passthrough over local store | (our addition) | (behavior in ridewithgps-pp-cli sql) | Power-user querying no tool offers |

Every read-only generated command sets `mcp:read-only`; mutating ones (delete/create/update) do not. The whole Cobra tree is mirrored as MCP tools automatically.

## Transcendence (only possible with our approach)

Survivors from the novel-features subagent (adversarial cut: 6 kept of 14). All `hand-code`. See `2026-06-17-novel-features-brainstorm.md` for customer model + killed candidates.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Bulk export | export | hand-code | Synthesizes GPX/TCX/CSV/KML from local track_points/course_points; `--native` streams legacy `/{routes,trips}/{id}.{gpx,tcx,fit,kml}`. The #1 unmet need — no competitor ships bulk export. | Use for exporting many routes or trips to disk. `--native` fetches RideWithGPS's own file render (incl. .fit) instead of locally-built files. For one route's cue sheet, the cue data is in `routes get`. |
| 2 | Gear mileage + maintenance | gear | hand-code | Local join of synced gear ↔ trips to sum per-bike distance and flag components past a wear threshold. Dashboard-only data, never queryable until now. | Use for per-bike distance and maintenance-due flags from logged trips (not planned routes). `gear due` derives wear from cumulative trip mileage, not a server-side maintenance log. |
| 3 | Route dedup | dedup | hand-code | Buckets routes by rounded distance + start/end coords + bbox in SQLite to cluster near-duplicates; `--apply` deletes extras. Requires local cross-row comparison no API call provides. | Use to find/remove duplicate routes. For quality issues other than duplication (stale, no cue sheet, visibility) use 'audit'. |
| 4 | Training stats | stats | hand-code | Time-windowed distance/elevation/moving-time aggregates with activity-type grouping over local trips. No API aggregation endpoint exists. | Use for time-windowed training totals/breakdowns. For all-time best single efforts use 'records'. |
| 5 | Personal records | records | hand-code | Sorts local trips on track-point-derived metrics (distance, elevation, max speed, max power) to surface all-time bests. | Use for all-time best efforts (longest, most climbing, fastest, biggest power). For period totals/averages use 'stats'. |
| 6 | Library audit | audit | hand-code | Predicate checks over synced routes/trips (age, empty course_points, empty tags, visibility) in SQLite. | Use for catalog-hygiene flags across the library. For duplicate detection use 'dedup'. |
| 7 | Routes from an event | events routes | hand-code | Legacy `GET /events/{id}.json` embeds a routes array (confirmed in .NET wrapper EventDetailsResponse); v1 event response does not surface routes. Lists an event's routes and reuses the export path to write device-ready files. | Use to list/export the routes attached to an organized ride. For your own routes use 'routes list'/'export'; this command resolves an event's official routes. |

Hand-code count: **7** (all transcendence rows). Auto-emitted: all absorbed rows (#1-13 generated endpoints; #14-18 framework/promoted). Added at Phase Gate 1.5 from user input (events→routes, bike-computer mapping emphasis).

### User-added context (Phase Gate 1.5)
User: *"what about mapping? most people I know use ridewithgps for better routes and mapping to their bike computer. Also for events and routes from events. Along with Analyzing data."* → `export` reframed for bike-computer/device output (FIT + cue sheets); new `events routes` feature added; `stats`/`records` confirmed as the data-analysis surface. CLI cannot render maps or discover *other people's* public routes (v1 returns only owned assets) — the device-export workflow is how this CLI serves "mapping."

## Stubs
None. Every feature ships fully implemented.
