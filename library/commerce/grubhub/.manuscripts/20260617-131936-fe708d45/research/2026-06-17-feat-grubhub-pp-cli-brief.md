# Grubhub CLI Brief

## API Identity
- Domain: Food-delivery marketplace (Grubhub/Seamless). Private web API at `api-gtm.grubhub.com`.
- Users: Frequent orderers, deal/coupon hunters, office/admin managers ordering for teams, developers building integrations. No official consumer API — everyone reverse-engineers the GTM API.
- Data profile: Restaurants (fees, minimums, ETAs, ratings, distance, cuisines, offers/coupons), full menus (categories → items → modifiers), geocoded addresses, order history (credentialed).

## Reachability Risk
- **None / Low (confirmed live on 2026-06-17)**. Every endpoint below returned clean JSON 200s over plain `curl` from this host — no PerimeterX/HUMAN challenge, no 403, no JS interstitial.
- Broader scraping community reports PerimeterX on the web surface (datacenter IPs may get 403 under load), so the printed CLI should ship a realistic desktop `User-Agent` + `Origin`/`Referer`, mint its own client_id dynamically, and surface a clear typed error on 403 (never silent nulls).
- Probe-safe endpoints used: `POST /auth` (anonymous), `GET /geocode`, `GET /restaurants/search`, `GET /restaurants/{id}`, `GET /restaurants/{id}/menu_items/{id}` — all 200.

## Auth (confirmed)
- **Anonymous bearer, fully self-service — no user key required.**
- Flow: `GET https://www.grubhub.com/eat/static-content-unauth?contentOnly=1` → regex `beta_[A-Za-z0-9]+` to scrape a live `client_id` (currently `beta_UmWlpstzQSFmocLy3h1UieYcVST`; rotates — always scrape fresh).
- `POST https://api-gtm.grubhub.com/auth` with header `Authorization: Bearer` (empty seed) and body `{"brand":"GRUBHUB","client_id":"<scraped>","device_id":<random 10-digit>,"scope":"anonymous"}` → `{"session_handle":{"access_token","refresh_token"}}`.
- All subsequent calls: `Authorization: Bearer <access_token>`. Token refresh via `refresh_token` available.
- Credentialed (logged-in) features — order history via `GET /diners/{ud_id}/search_listing` — require a real account login; **out of v1 scope** (deferred, no resident-session needed for the public surface).

## Top Workflows
1. **Search restaurants near an address** — geocode address → `POINT(lng lat)` → search with cuisine/sort/delivery-pickup filters. Sort by fee/ETA/rating/distance.
2. **Browse a restaurant's full menu** — categories, items, prices, popular flags, item coupons.
3. **Compare delivery fee / minimum / ETA across restaurants** — the app buries this per-restaurant; a sortable table is the headline win.
4. **Find deals/promos** — surface `available_offers`, `coupons_available`, `available_promo_codes` across nearby restaurants in one view.
5. **Inspect a menu item's modifiers/choices** — `choice_category_list` (sizes, add-ons) for a specific item.

## Table Stakes (from competing tools: n0shake/dash, Cash App MCP, Grubhub MCP, Apify scrapers)
- `search` by location (address or lat/lng), cuisine facet, delivery vs pickup, sort, paginate.
- `menu` / restaurant details by id.
- `item` modifiers.
- `deals` / offers listing.
- `geocode` address → coordinates.
- Cart/order flow (credentialed) — competitors expose it; v1 ships read-only marketplace browsing, cart deferred (needs login).

## Data Layer
- Primary entities: `restaurants` (search cards + details), `menu_items`, `geocodes` (address cache).
- Sync cursor: search is location-scoped, not time-cursored; local store caches restaurant cards + menus per location for offline compare/search.
- FTS/search: full-text over cached restaurant names, cuisines, and menu item names/descriptions — powers offline "which nearby place has X dish" queries no single API call answers.

## Codebase Intelligence
- Source: `jlumbroso/grubhub` (Python, ~6★, best auth reference — dynamic client_id scrape, refresh_token), `patilanup246/grubhubScraper` (client_ids, restaurant/menu endpoints).
- Auth: anonymous bearer via `/auth`, dynamic client_id. Confirmed working 2026-06-17.
- Data model: `search_result.results[]` cards carry `delivery_fee.price`, `delivery_minimum.price`, `delivery_time_estimate`, `ratings`, `distance_from_location`, `cuisines`, `coupons_available`, `available_offers`. Details add `menu_category_list`, `latitude/longitude`, `address`.
- Location format: WKT `POINT(longitude latitude)`, URL-encoded (lng first).
- Rate limiting: not observed in probing; keep request rates modest, handle 403 as typed PerimeterX error.

## User Vision
- (none provided — user chose "Let's go")

## Product Thesis
- Name: **grubhub-pp-cli** ("gh" food CLI)
- Why it should exist: Grubhub's app forces tap-into-each-restaurant to see fees/minimums/ETAs and buries deals per-restaurant. A CLI gives instant sortable fee/ETA/deal comparison across every nearby restaurant, full offline menu search ("who near me has a poke bowl under $15"), and agent-native JSON output — none of which any official surface or existing tool offers together.

## Build Priorities
1. P0 data layer: restaurants, menu_items, geocode cache + FTS.
2. Auth flow: dynamic client_id scrape + anonymous bearer mint + refresh (generated auth, no user key).
3. P1 absorbed commands: `geocode`, `search`, `menu`, `item`, `deals`.
4. P2 transcendence: cross-restaurant fee/ETA/deal comparison, offline dish search, deal radar, menu-item price index.

## Source Priority
- Single source (grubhub.com / api-gtm.grubhub.com). No combo ordering.

## Reachability Gate
- Decision: PASS
- Reason: direct-http-confirmed
- Evidence: POST /auth (200), GET /geocode (200), GET /restaurants/search (200, 85KB), GET /restaurants/{id} (200, 121KB), GET /restaurants/{id}/menu_items/{id} (200) — all clean JSON over plain curl on 2026-06-17, no PerimeterX challenge.
