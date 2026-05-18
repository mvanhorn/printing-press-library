# Bandsintown CLI Brief

## API Identity
- **Domain:** Live music — concert/event data keyed by artist
- **Users:** Promoters, festival bookers, tour routers, fan apps, artist managers, music analytics
- **Data profile:** Read-only artist info + chronological events with venue (lat/lng), lineup, ticket offers, tracker counts
- **Spec source:** apis.guru — `https://api.apis.guru/v2/specs/bandsintown.com/3.0.0/swagger.yaml` (Swagger 2.0, 2 endpoints, 7KB)

## Reachability Risk
**HIGH.** Live API now returns HTTP 403 ("explicit deny" AWS IAM-style) for unauthenticated requests. The legacy "free app_id" model has been deprecated — access is partner-only. Confirmed via:
- Direct probe: `GET /artists/Radiohead?app_id=test` → 403
- Multiple GitHub issues on community wrappers (subdigital/intown, aroscoe/bandsintown) about 403 errors
- Bandsintown help center explicitly directs to partner application process

**Decision:** Generate against the spec for the scaffold. Live testing skipped. Partner key wires in post-gen via `BANDSINTOWN_APP_ID` env var. Matches Double Deer `docs/printing-press-queue.md` plan.

## Top Workflows (project-context driven)
1. **Tour-routing feasibility for Jakarta/SEA** — For a target city + date window, find which artists already have nearby shows (Singapore, KL, Bangkok, Manila, Tokyo) we could route into Indonesia. This is the entire reason Double Deer's `tour-router` agent exists.
2. **Gap detection** — For an artist of interest, find empty windows between confirmed tour dates that match Double Deer's event slots.
3. **Lineup intelligence** — Mine `lineup[]` arrays across many events to detect frequent collaborators, festival co-bills, and emerging artist patterns.
4. **Demand snapshot tracking** — Capture `tracker_count` + `upcoming_event_count` over time per artist; surface rising-demand artists.
5. **Regional event aggregation** — Sync many artists' events, then query "all SEA events between dates X and Y" locally.

## Table Stakes
From the existing wrapper ecosystem (subdigital/intown, aroscoe, chrisforrette, TappNetwork, datafire, bandsintown/api-gem):
- Get artist info by name
- Get artist events (upcoming / past / all / date-range)
- Print/format event details with venue + tickets
- Filter events by date or location
- (None of them have: sync, offline search, routing analysis, snapshot diffing, MCP exposure, agent-native output)

## Data Layer
- **Primary entities:** `artists`, `events`, `venues`, `offers`, `lineup_members` (junction)
- **Sync cursor:** `(artist_name, synced_at)` — Bandsintown has no cursor token, so we re-fetch by artist with a 12-24h staleness window
- **FTS/search:** artist name, venue name+city, event description, lineup members
- **Snapshots:** `artist_snapshots(artist_id, tracker_count, upcoming_event_count, snapped_at)` — enables `tracker-trend` transcendence command

## Codebase Intelligence
- **Source:** Community wrappers + spec analysis (no MCP server exists yet for Bandsintown)
- **Auth:** `app_id` query parameter on every request (Swagger 2.0 spec has it as required parameter but no `securityDefinitions` block — enrich before generation)
- **Token format:** Bandsintown-issued partner string; env var `BANDSINTOWN_APP_ID`
- **Data model:** Artist 1:N Events; Event 1:1 Venue; Event 1:N Offers; Event N:M Artists via lineup
- **Special chars in artist names:** Spec requires URL-escaping: `/` → `%252F`, `?` → `%253F`, `*` → `%252A`, `"` → `%27C` (double-encoded). Must handle this in client.
- **Rate limiting:** No documented limits in the spec; community wrappers report polite ~1 req/sec is fine.

## User Vision
The user's Double Deer project explicitly lists Bandsintown as data source #1 in the printing-press queue. Their `tour-router` agent is designed around Bandsintown as the primary source. The CLI being built is the foundation for `lib/sources/bandsintown.ts` in their Next.js app.

## Product Thesis
- **Name:** `bandsintown-pp-cli`
- **Why it should exist:** Every existing wrapper is a 10-year-old language binding for the 2 endpoints. None offer: a local SQLite cache, offline search, agent/MCP exposure, routing-feasibility analysis, gap detection, lineup mining, or snapshot-based demand tracking. Double Deer (and any booker, festival promoter, or music agency) needs all of these to do the actual work of booking artists for events.

## Build Priorities
1. **Spec generation + auth enrichment** (`app_id` as query-param API key, env var `BANDSINTOWN_APP_ID`)
2. **Absorbed endpoint commands**: `artists get`, `events list` (with `--date upcoming/past/all/range`)
3. **Sync + local store**: artists, events, venues, offers, lineup_members
4. **Search**: FTS5 across artist/venue/lineup
5. **Transcendence commands** (the differentiators):
   - `route` — routing feasibility from many tracked artists to a target city/date
   - `gaps` — empty windows in an artist's calendar
   - `lineup` — co-bill / festival lineup intelligence
   - `snapshot` + `trend` — tracker_count snapshot diffing
   - `nearby` — events within radius of lat/lng
   - `festivals` — auto-detect multi-artist events as festivals
