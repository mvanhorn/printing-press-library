# Grubhub CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search restaurants near a location | n0shake/dash, Cash App MCP, Grubhub MCP | grubhub-pp-cli near | Address-based (auto-geocodes), anonymous-token auto-mint, --json/--select, offline cache. Named `near` to avoid the framework `search` (offline FTS) command. |
| 2 | Browse restaurant full menu | Cash App MCP get-single-menu, Apify scrapers | grubhub-pp-cli menu | Full categories + items + prices, --json/--select, offline |
| 3 | Menu item modifiers/choices | Apify scrapers | grubhub-pp-cli item | choice categories, item coupons, --json |
| 4 | Geocode address to coordinates | (internal need; geocoders) | grubhub-pp-cli geocode | Address -> POINT(lng lat), --json |
| 5 | Filter by cuisine | n0shake/dash, Grubhub MCP | (behavior in grubhub-pp-cli near) --cuisine | server facet + client filter |
| 6 | Delivery vs pickup | n0shake/dash, Cash App MCP (pickup) | (behavior in grubhub-pp-cli near) --pickup | order-method facet |
| 7 | Sort results | Grubhub MCP, app | (behavior in grubhub-pp-cli near) --sort | fee/eta/rating/distance client sort |
| 8 | Pagination | all scrapers | (behavior in grubhub-pp-cli near) --page/--page-size | bounded pages |
| 9 | Raw restaurant search endpoint | Apify scrapers, jlumbroso | (generated endpoint) restaurants search | typed POINT-based raw surface for power users |
| 10 | Raw restaurant details endpoint | Apify scrapers | (generated endpoint) restaurants get | typed details + menu |
| 11 | Raw menu-item endpoint | Apify scrapers | (generated endpoint) restaurants menu-item | typed item detail |
| 12 | Anonymous auth handshake | jlumbroso/grubhub | (behavior in grubhub-pp-cli auth login) | credential-free token mint + dynamic client_id scrape + cache |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Fee/ETA/minimum comparison board | compare | hand-code | Requires a local join across cached restaurant cards + multi-key client-side sort over delivery_fee/minimum/ETA/rating/distance; the app buries each per-restaurant | none |
| 2 | Offline dish search | dish | hand-code | Requires SQLite FTS over cached menu_item_list across all nearby restaurants; no single API call searches menu item names neighborhood-wide | Use this to find a specific menu item across all nearby restaurants. Do NOT use it to browse one known restaurant's full menu; use 'menu <restaurant-id>' for that. --diet is a mechanical keyword match, not a verified dietary certification. |
| 3 | Deal radar | deals | hand-code | Requires cross-restaurant aggregation + value ranking of offers/coupons/promo codes from the local cache; the app shows offers only per-restaurant | Use this for a ranked cross-restaurant view of who is running a deal right now. Do NOT use it to read offers on a single restaurant; 'search' surfaces per-restaurant offers inline. |
| 4 | Best-value picker | pick | hand-code | Requires a transparent normalized score across cached fee/rating/active-offer/ETA — deterministic, no LLM | Use this for one recommended restaurant from a transparent fee/rating/deal/ETA score. Do NOT use it for the full ranked table; use 'compare'. |

**Hand-code transcendence rows: 4** (compare, dish, deals, pick). Plus framework `sync` populates the offline store.

## Deferred / stubs
- **Order history / re-order** (`/diners/{ud_id}/search_listing`): requires credentialed login (real account). Out of v1 scope — documented as a known gap, not stubbed in the command surface.
- **Cart / place order**: requires credentialed session + payment; out of v1 scope (read-only marketplace browsing).
