# Wolt CLI Brief

## API Identity
- Domain: Food delivery / venue discovery — Wolt operates in 25+ countries (mostly EU + Israel + Japan)
- Users: Anyone deciding where/what to order; analysts who want venue/menu data; AI agents that want "what's open near me with X cuisine?"
- Data profile: Cities (~hundreds), venues per city (~thousands), menus per venue (~50-300 items), categories/cuisines, delivery fees + ETAs, opening hours, ratings, share-link order tracking

## Reachability Risk
- **Low for browse endpoints** (confirmed via live curl):
  - `GET /v1/cities` (restaurant-api): 200, 265 KB
  - `GET /v1/pages/restaurants?lat=&lon=` (consumer-api): 200, 2.2 MB, two sections of restaurant items
  - `POST /v1/pages/search` (restaurant-api): 200, returns venues or items by `target`
- **Medium for menu endpoint**: `GET /v4/venues/slug/{slug}/menu/data` returns HTTP 200 with `content-length: 0` via CloudFront. Likely needs browser-sniffed headers (Origin/Referer/X-Wolt-Session-Id or similar fingerprint). The OzTamir gist documents the path but the live response shape varies.
- **Unknown for order tracking by share link**: Tracking URL is `wolt.com/en/track/<id>`. The JSON endpoint behind it isn't documented in the gist (404 on guesses). Browser-sniff required.
- No 403/429/CF challenge observed on the working endpoints — no clearance cookie or browser fingerprint needed for search/list/cities.

## Top Workflows (browse-only scope)
1. "What's open near me right now for cuisine X with delivery under N minutes?"
2. "Find a specific venue and see its menu (categories + items + prices + dietary tags)"
3. "Compare delivery fees and ETAs across nearby venues for the same cuisine"
4. "Track an order via the share link my friend sent me"
5. "Browse what cities Wolt operates in (planning a trip)"

## Table Stakes (from competitors)
- **what-to-eat** (Python CLI): `ls` (filtered list), `random` (weighted pick), profile/location config, sort by rating, filter by tag/cuisine, price-tier display
- **wolt-cli** (setevik): order history — **out of scope** (requires login)
- **Apify wolt-restaurants-scraper MCP**: bulk scrape city → venues with name/address/zip/phone/rating
- Common features to absorb: lat/lon profile, cuisine/tag filter, sort by rating/ETA/delivery-fee, price-tier display, JSON output, random/weighted pick, multi-city browsing, search items (not just venues)

## Data Layer (SQLite)
- Primary entities: `cities`, `venues`, `menu_categories`, `menu_items`, `cuisine_tags`, `tracked_orders` (share-link cache)
- Sync cursor: per-city last-fetched timestamp; per-venue menu last-fetched
- FTS5 search: venues (name + description + cuisines), items (name + description)
- Snapshots: delivery_fee / ETA / price-range over time for price/fee drift analysis

## Codebase Intelligence
- Source: OzTamir gist + curl probes
- Auth: **none** for browse endpoints. `Authorization: Bearer` only for user-specific endpoints (orders, profile) — explicitly out of scope.
- Data model: venues have `slug` (stable id), `id` (mongoid), `city.slug`, `online`, `estimate` (ETA in min), `delivery_price`, `rating`, `tags`, `location`
- Rate limiting: not observed; CloudFront-fronted; respect normal rate limits
- Architecture: CloudFront → backend; menu endpoint cache-fingerprints on Origin/Referer headers

## Product Thesis
- **Name**: `wolt-pp-cli` — "Wolt, but for terminals and agents"
- **Why it should exist**:
  1. Existing CLIs are city-specific (what-to-eat) or auth-gated (wolt-cli). Nothing offers offline-cached menu/venue data with FTS across multiple cities for AI agents.
  2. Wolt's website is heavy SPA; a fast CLI that returns `--json` of "10 open noodle places under 30 min in Helsinki" beats clicking through cards.
  3. Trip planners want to scout food options before they arrive. No tool does multi-city venue snapshots.
  4. Agents currently can't reason about delivery fees/ETAs in a structured way.

## Build Priorities
1. **Foundation**: SQLite store (cities, venues, items, snapshots, tracking). City + venue sync from `pages/restaurants`. FTS5 over venues+items.
2. **Absorb table-stakes**: `venues list/get`, `cities list`, `search`, `items search`, `random`, profile/location config, filter/sort, JSON+CSV+select.
3. **Transcendence**: cross-city compare; delivery-fee/ETA drift over time; "what's-open-now" filter; offline menu browse; order-tracking poll by share link; cuisine bottleneck analysis (where are wait times longest right now?).
4. **Polish**: README cookbook, agent-context, MCP exposure.
