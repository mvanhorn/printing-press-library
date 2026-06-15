# Tripadvisor CLI Brief

## API Identity
- Domain: Travel content — hotels, restaurants, attractions, geos. Official Tripadvisor Content API (`api.content.tripadvisor.com/api/v1`).
- Users: Travel agents/planners, trip-research agents, anyone comparing places by rating/ranking.
- Data profile: Per-location records (rating, num_reviews, ranking, address, hours, price level, awards), up to 5 reviews (UGC) and 5 photos per location on the free tier.
- Auth: API key as `?key=` query param (env `TRIPADVISOR_API_KEY`). Key requires IP (/32) or domain restriction set at creation. Domain-restricted keys also need a `Referer` header.

## Build Path Decision (logged)
- User asked for a no-key path. Firsthand browser capture confirmed direct tripadvisor.com scraping is NOT shippable: DataDome blocks replay — in-browser 200 w/ JSON-LD, but curl replay of detail pages 403 even with a freshly page-warmed `datadome` cookie (homepage replays only; data pages bound to the browser TLS/HTTP2 fingerprint). Resident-browser runtime is forbidden by the Press. User then chose the **official Content API**.

## Reachability Risk
- None. `standard_http`. Base returns clean JSON `401 Unauthorized` without a key (confirmed). No bot protection on the API host.

## Top Workflows
1. "Find the best-rated hotels in <city>" — search, then compare rating + review count + ranking.
2. "Should I pick place A or place B?" — fetch details for both, compare side by side.
3. "Show me what travelers say" — recent reviews (UGC) for a location.
4. "Find places near these coordinates" — nearby search by lat/long.
5. "Get photos / the detail page link for a place."

## Table Stakes (from the one competing tool — pab1it0/tripadvisor-mcp)
- search_locations, search_nearby_locations, get_location_details, get_location_reviews, get_location_photos. We match all five 1:1 (`find`, `near`, `show`, `reviews`, `photos`) AND add offline store, --json/--select, typed exit codes, SQLite persistence.

## Data Layer
- Primary entities: locations (details), reviews, photos. Local SQLite caches fetched locations so `search` (framework FTS), `compare`, `shortlist`, and `drift` work offline and compound.
- Metered API (5k calls/mo free) — no auto-refresh-before-read cache; rely on manual sync + the client's read cache.

## Naming note (surface at gate)
- `search` is a reserved framework command (offline FTS over the local store). The live API location search is therefore exposed as **`find <query>`**; framework `search` stays as offline FTS over cached locations. All other verbs (`near`, `show`, `reviews`, `photos`) match the brief verbatim.

## Product Thesis
- Name: tripadvisor (display: Tripadvisor)
- Why it should exist: The only TripAdvisor tool that turns the search→details→compare loop into one agent-native command surface with a local store. Surfaces rating + review count + ranking up front so an agent can rank and choose without parsing verbose payloads.

## Build Priorities
1. P0: store for locations/reviews/photos; sync/search/SQL path.
2. P1: the 5 API verbs (find/near/show/reviews/photos), each agent-native with --json/--select/--dry-run.
3. P2 (transcend): compare, best, digest, shortlist, drift — the local-aggregate commands that serve "compare options."
