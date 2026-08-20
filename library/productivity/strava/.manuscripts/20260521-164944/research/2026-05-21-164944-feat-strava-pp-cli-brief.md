# Strava CLI Brief

## API Identity
- Domain: Sports/fitness tracking — running, cycling, swimming, hiking activity tracking
- Users: Serious athletes, coaches, data-driven fitness enthusiasts, sports scientists
- Data profile: Rich temporal data — GPS tracks, heart rate, power, cadence, elevation; heavy write (activity upload) and heavy read (analytics)

## Reachability Risk
- **Low**
- Evidence: Official REST API at https://www.strava.com/api/v3 with stable Swagger 2.0 spec (34 endpoints). OAuth2 auth. Rate limits are 200 requests/15min, 2,000/day — benign for CLI use. stravalib has some endpoint deprecation issues (friends/followers removed, kudos→summit rename) but these are in the library wrapper, not the API itself. The raw API is stable.

## Top Workflows
1. **Activity analytics** — Sync all historical activities to local SQLite, then query: filter by date/type/effort, aggregate weekly/monthly mileage, spot trends over months/years. The #1 pain the website can't satisfy.
2. **Segment performance tracking** — Query personal segment efforts, compare PRs across seasons, track progression on key segments. Competitive cyclists/runners obsess over specific segments.
3. **Training load analysis** — Compute training stress scores, identify overtraining vs. under-training periods, correlate HR/power zone time with performance. Requires raw streams from activities.
4. **Bulk activity management** — Update activity names, gear assignments, descriptions in bulk. Upload GPX/FIT files programmatically. Power users maintain activity metadata at scale.
5. **Export & integration** — Export activities to GPX/TCX for third-party tools, pipe data to Sheets/Notion/custom dashboards, feed AI agents for personalized coaching insights.

## Table Stakes
- `activities list` — list recent/all activities with filtering
- `activities get <id>` — get detailed activity
- `activities create` / `activities update`
- `activities streams <id>` — raw HR/power/GPS data
- `athlete get` — profile + stats
- `athlete zones` — power/HR zones
- `segments get/search/explore` — star/unstar
- `segment-efforts list` — personal efforts
- `routes list/get/export` — GPX/TCX export
- `clubs list/get/activities`
- `gear get`
- `uploads create` — upload FIT/GPX/TCX
- Auth: `auth login` (OAuth2 flow with local callback server)
- `doctor` — validate credentials, test connectivity
- `sync` — local SQLite mirror with incremental updates
- `search` — full-text search over local store

## Data Layer
- Primary entities: activities, segment_efforts, routes, segments, gear, clubs
- Sync cursor: activity `start_date` for incremental sync; `before`/`after` params for pagination
- FTS/search: activity names, descriptions, segment names
- Streams: store as JSON blobs keyed by activity_id (too large for relational)

## Codebase Intelligence
- Source: Official Swagger spec — https://developers.strava.com/swagger/swagger.json
- Auth: OAuth2 authorization-code flow; `strava_oauth` security scheme; env vars `STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET`, `STRAVA_ACCESS_TOKEN`, `STRAVA_REFRESH_TOKEN`
- Data model: Activities are the central entity; Segments and SegmentEfforts link to activities; Streams hang off activities; Gear references athletes
- Rate limiting: 200 req/15min, 2000/day; HTTP 429 on exceed
- Architecture: REST-only; 34 endpoints; OAuth2 tokens expire and need refresh

## Product Thesis
- Name: strava-pp-cli
- Why it should exist: The Strava website is beautiful but analytically limited. Every serious athlete wants to query "show me all rides where I beat my FTP threshold more than 3 times" or "which segments did I PR last month?" — questions the website can't answer. A CLI that mirrors your Strava data to local SQLite and exposes it through composable commands + AI-native JSON output makes you the owner of your own fitness data.

## Build Priorities
1. OAuth2 auth flow with token refresh (`auth login`, `auth refresh`, token storage)
2. Full data layer: activities, segments, efforts, gear, streams in SQLite
3. Incremental sync with smart cursor (avoid re-fetching the full history each run)
4. Analytics commands: weekly/monthly rollups, zone analysis, FTP trends
5. Segment PR tracking and progression
6. Raw stream data access for third-party analysis
7. GPX/TCX export for activities and routes
8. Bulk operations: update activity metadata in batch
9. Training load score computation (local — no API call needed once streams are synced)
10. MCP server with read-only tools for AI coaching queries
