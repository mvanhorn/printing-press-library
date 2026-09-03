# Flipp CLI Brief

## API Identity
- Domain: Flipp flyer, deal, coupon, and merchant discovery for local shopping.
- Users: shoppers comparing weekly grocery and household prices by ZIP or postal code, and agents building shopping lists from local flyers.
- Data profile: unauthenticated JSON over HTTPS, keyed mainly by `postal_code`, `locale`, item query, flyer ID, and merchant.

## Reachability Risk
- Low. Direct probes on 2026-06-27 returned HTTP 200 JSON for item search, flyers, merchants, combined flyer/coupon data, flyer item clippings, and IP location.
- Probe-safe endpoint used: `GET /items/search?q=milk&postal_code=85001&locale=en-us`.
- Official support risk: Flipp does not publish these as a stable public API. Existing wrappers describe them as reverse-engineered web endpoints, so endpoint drift remains possible.

## Top Workflows
1. Search for grocery staples near a ZIP code and sort by price, discount, or merchant.
2. Browse active weekly/monthly flyers for local grocery and household stores.
3. Pull the item clippings from a specific flyer for meal planning or pantry restocking.
4. Aggregate coupon and flyer data into a local SQLite mirror, then search offline.
5. Compare a shopping list across local merchants to find the cheapest practical basket.

## Table Stakes
- `thomas-chong/flipp-cli` covers search, flyers, merchants, locate, deal aggregation, unit-price heuristics, images, JSON/NDJSON, field projection, and postal-code config.
- `Kiizon/flippscrape` demonstrates `flyers-ng.flippback.com/api/flipp/data` and per-flyer item extraction for grocery flyers.
- A Printing Press CLI must at least match search, flyers, merchants, locate, coupon/data fetch, and flyer items, then add SQLite-backed shopping workflows and MCP.

## Data Layer
- Primary entities: items, flyers, flyer_items, merchants, coupons, searches, and shopping-list rows.
- Sync cursor: timestamped snapshots by ZIP, locale, endpoint, and query pack. Flipp has no explicit cursor, so snapshot freshness is local.
- FTS/search: item name, merchant, brand, category, sale story, flyer name, coupon description.

## User Vision
- The user wants easy access to weekly/monthly food flyer deals by ZIP code, plus coupons, savings, and other deals.

## Product Thesis
- Name: Flipp
- Why it should exist: Flipp's web API already exposes useful unauthenticated local shopping data, but existing wrappers are stateless. A Printing Press CLI can turn those endpoints into an agent-native local savings workspace with sync, SQL, MCP, and shopping-list optimization.

## Build Priorities
1. Generate endpoint commands for search, flyers, flyer data/coupons, flyer items, merchants, and IP location.
2. Add shopping-list/basket commands that fan out item searches and compare local prices.
3. Add expiring-soon and merchant coverage views over the local mirror.
4. Preserve raw JSON, `--json`, `--select`, `--csv`, and MCP surfaces for agent workflows.

## Reachability Gate
- Decision: PASS
- Evidence: GET https://backflipp.wishabi.com/flipp/items/search?locale=en-us&postal_code=85001&q=milk returned HTTP 200 JSON on 2026-06-27.
