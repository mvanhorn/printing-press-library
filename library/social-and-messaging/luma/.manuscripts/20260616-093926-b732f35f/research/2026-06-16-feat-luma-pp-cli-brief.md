# Luma CLI Brief

## API Identity
- Domain: Luma (luma.com / lu.ma) — events platform. Discover, browse, host events; sell tickets; community calendars.
- Route: **public website internal API** (`https://api.luma.com`). NOT the documented Luma Plus API (paywalled, requires Luma Plus subscription + per-calendar `x-luma-api-key`).
- Users: People finding events near them, by city or topic; agents pulling event/community data programmatically.
- Data profile: Events (time, place, host, guest/ticket counts, categories), Places (cities), Categories (AI/Crypto/Tech/...), Calendars (communities). All public, read-only, JSON.

## Reachability Risk
- **None.** All 8 captured endpoints return 200 via plain `curl` — no auth header, no cookie, no Cloudflare/bot challenge. `http_transport: standard` (stdlib HTTP). Printed CLI needs no browser at runtime.
- Verified live 2026-06-16: discover/bootstrap-page (202KB), get-paginated-events, get-place, get-calendars, event/get, calendar/get all 200.

## How the surface was discovered
- User pivoted from the documented Luma Plus API (paywall) to the public website.
- Playwright CLI drove anonymous browsing of luma.com/discover and luma.com/sf; captured `api.luma.com` XHR; each endpoint then re-verified replayable via plain curl.

## Verified endpoints (all GET, no auth, standard_http)
| Endpoint | Purpose | Key params |
|---|---|---|
| `/discover/bootstrap-page` | discover home: featured place, places, categories, calendars | — |
| `/discover/category/list-categories` | category list w/ event counts | pagination_limit, pagination_cursor |
| `/discover/get-place` | place (city) details + api_id | **slug** OR discover_place_api_id |
| `/discover/get-paginated-events` | events list (the core) | **slug** OR discover_place_api_id OR **category_api_id**, pagination_limit, pagination_cursor |
| `/discover/get-calendars` | calendars in a place | discover_place_api_id |
| `/discover/place/get-points-for-mini-map` | geo points for events in a place | discover_place_api_id |
| `/event/get` | full event detail | event_api_id (evt-...) |
| `/calendar/get` | calendar (community) detail | api_id (cal-...) |

Notes:
- `get-place?slug=sf` resolves city slug → `discplace-...` api_id; `get-paginated-events?slug=sf` accepts the slug directly too.
- `get-paginated-events` is the same endpoint for place- OR category-filtered events.
- Pagination is cursor-based: response `{entries, has_more, next_cursor}`; pass `pagination_cursor=<next_cursor>`.
- **No public free-text search endpoint exists** — discovery is browse-by-place/category only. (We add search via local FTS — see transcendence.)
- Quirk: event uses `event_api_id`, calendar uses `api_id` (not `calendar_api_id`).

## Top Workflows
1. "What events are happening in {city} this week?" → `events list --city sf`
2. "Show me {category} events" (AI, Crypto) → `discover categories` then `events list --category cat-ai`
3. "Full details for this event" → `events get <evt-id>`
4. "Export a city/category's events to my calendar" → ICS export of a filtered set
5. "Search across everything I've pulled" → offline FTS (API has none)

## Table Stakes (from competitor tools)
- Discover events from public feed (alx1p luma-cal-mcp, Apify scraper)
- Distance / geo filtering (alx1p) — we have coordinates + mini-map points + place distance_km
- ICS / calendar export (alx1p)
- Browse calendars/communities
- Browse by category
- Fetch single event detail

## Data Layer
- Primary entities: events, places (cities), categories, calendars.
- Sync cursor: `next_cursor` per place/category.
- FTS: events (name, host, location, category) — **the public API has no search, so local FTS is pure value-add.**
- Geo: events carry `coordinate` (lat/lng) → haversine radius queries over the local store.

## Why install this instead of incumbents
- Incumbents are either Plus-API wrappers (paywalled) or Node/Python MCP servers. This is a single Go binary, no account, no key.
- Adds what the platform lacks: full-text search, cross-city aggregation in one query, change/drift tracking over time, ICS export of any filtered set, agent-native `--json`/`--select`.

## Product Thesis
- Name: **luma-pp-cli** ("luma")
- Thesis: The events-discovery CLI Luma never shipped publicly — browse every city and category, then search/aggregate/export offline, no Luma account required.

## Build Priorities
1. P0 data layer: events/places/categories/calendars in SQLite; sync; FTS.
2. P1 absorbed: discover home, places (get/calendars/map), events (list/get), categories, calendar get, ICS export, geo/distance filter.
3. P2 transcendence: offline FTS search, cross-city aggregation, watch/drift, geo-radius, category×city digest.
