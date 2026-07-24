# Peekaboo Guru CLI Brief

## API Identity
- Domain: Deals & discounts aggregator for Pakistan (Fetch Sky). Restaurants, retail, banks/card-offers across 982 cities. Karachi (17,676 entities) & Lahore lead.
- Users: Deal-hunters ("where can I eat cheap tonight"), cardholders ("which of my bank cards gives a discount here"), and people who need a restaurant's nearest branch + directions.
- Data profile: cities(locations), categories, merchants(entities), brands/card-sources(sourceEntities), branches(with lat/long), deals(with % + validity), amenities. All keyed by city + coordinates.

## Reachability Risk
- None. All 10 endpoints verified live (HTTP 200). Cloudflare present but does not challenge /api/* for a Chrome User-Agent.
- Rate-limits bursts (~30 rapid requests drop). CLI paces requests.
- Probe-safe endpoint used: POST /api/v5/locations (public, no auth).

## Auth
- Public guest JWT bearer token, embedded in every page as `window.__guest__.token` (role=guest, no expiry, static since 2019).
- `locations` is public; every other endpoint 401s without the token.
- Zero-config design: CLI scrapes the token from a page once, caches it, sends `Authorization: Bearer`. Token value never written to any artifact.

## Top Workflows
1. Browse deals in a city+category: pick city -> pick category -> list merchants with "up to X% off".
2. Find a restaurant's branches with directions: merchant -> branches (name, address, lat/long -> Google Maps directions URL).  ← user's headline requirement
3. Card-deal lookup: which bank cards give discounts at merchant X (or across the city).
4. Deal detail: the actual offers on a merchant (title, %, validity window, terms).
5. Discover top-deal merchants / biggest discounts in a city.

## Table Stakes
- List merchants by city+category, list deals, list branches, list categories, list cities, list brands/card-sources, merchant detail, amenities.
- Filtering by category, discount, open-now; pagination (limit/offset).

## Data Layer
- Primary entities: locations(cities), categories, entities(merchants), branches, deals, sourceEntities(brands/banks).
- Sync cursor: offset-based; entities have nextPage flag.
- FTS/search: merchant names, deal titles/descriptions, branch addresses, brand names.
- Local store unlocks: city->coordinates resolution (so users pass --city not raw lat/long), offline branch/coordinate export, cross-merchant deal ranking, card-to-merchant reverse index.

## User Vision
- Full coverage: places, deals, brands, cities, categories, search.
- PLUS: list all branches of each restaurant WITH the coordinates of each branch — because the site's per-branch "Direction" button opens Google Maps with the coordinates in the URL (maps.google.com/maps?daddr={lat},{long}). Verified: GET /api/v5/entity/{id}/branch/_all returns branches[].latitude/.longitude. Example given by user: /lahore/detail/609/arcadian-cafe/branches.

## Product Thesis
- Name: peekaboo (peekaboo-pp-cli)
- Why it should exist: Peekaboo has no CLI and no public API docs. This is the only way to script deal discovery, bulk-export a restaurant's branch coordinates for mapping/routing, and answer "which card, which branch, which deal" from the terminal or an agent — with a local SQLite mirror that makes city->coords resolution and cross-merchant deal ranking instant and offline.

## Build Priorities
1. Data layer + sync for locations, categories, entities, branches, deals, brands.
2. Absorbed read commands for all 10 endpoints (places/deals/branches/cards/brands/categories/cities/detail/amenities).
3. Zero-config guest-token bootstrap.
4. City->coordinate auto-resolution so users pass --city instead of lat/long.
5. Transcendence: branch coordinate/Maps export, card-to-merchant reverse index, cross-merchant deal ranking, offline deal search.
