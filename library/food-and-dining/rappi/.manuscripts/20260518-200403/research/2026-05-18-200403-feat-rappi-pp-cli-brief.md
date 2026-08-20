# Rappi CLI Brief

## API Identity
- Domain: On-demand delivery super-app (restaurants, supermarkets, pharmacy, liquor, convenience, Turbo dark stores). LATAM-wide, MX is #1 market.
- Users: Urban Mexican consumers (CDMX/Guadalajara/Monterrey concentration), restaurant operators (merchant API, out of scope), and a small developer/analyst audience.
- Data profile: SSR-rendered store and restaurant catalogs, geo-gated by lat/lng, all Spanish content, numeric stable store_ids (e.g., `642`, `888002`), Spanish category slugs (`hamburguesas`, `pizza`, `sushi`, `tacos`, `pollo`, `mexicana`).

## Reachability Risk
- **Low for HTML SSR** — direct curl of `rappi.com.mx` returns 200 OK from a US residential connection with `web-version: 7.0.0` and SSR content embedded. probe-reachability returned `standard_http` mode (95% confidence). HTML path is the safe default.
- **High for JSON/XHR endpoints** — every commercial scraper (Apify parseforge, Apify yasmany.casanova, Bright Data) requires residential proxies. The browser-sniff will reveal whether JSON endpoints work from datacenter IPs; if not, fall back to HTML-only.
- **No GitHub issue trail for "rappi 403/blocked/cloudflare"** because no widely-used wrapper exists for issues to land on. Blocking signal is the proxy requirement on commercial offerings.

## Top Workflows
1. **What's near me right now?** — discover open restaurants/stores at a given lat/lng with delivery time and fee thresholds.
2. **Menu comparison across stores** — pull the same item across multiple supermarkets and rank by price/availability.
3. **Restaurant menu archival / price tracking** — snapshot a menu, diff over time to track price changes.
4. **Category sweeps** — list every burger / sushi / pharmacy / liquor option in a neighborhood, sorted by multi-axis (rating × delivery-time × fee).
5. **Coverage / availability checks** — given an address, what's reachable, what's Turbo-eligible, which promotions apply.

## Table Stakes
Cross-referenced from Bright Data, Apify, and competitor (DoorDash CLI, UberEats client) field sets:
- **Store/restaurant:** id, name, image, rating, review_count, delivery_fee, delivery_eta, prime_flag, free_delivery_flag, address, city, store_type, cuisine_category, url
- **Product/menu item:** id, name, brand, description, image, current_price, original_price, discount, currency, in_stock, rating, reviews_count, category
- **Filters:** city, lat/lng, store_type (market|pharmacy|liquor|express|turbo), restaurant category (hamburguesas, pizza, sushi, tacos, pollo, mexicana, saludables, postres), sort (rating|delivery-time|fee), prime, free_delivery
- **No login required** for any of the above; all reachable from public SSR pages.

## Data Layer
- Primary entities:
  - `stores` — supermarkets/pharmacy/liquor/convenience (`store_type` enum)
  - `restaurants` — delivery restaurants (`category` slug)
  - `menu_items` / `products` — items for sale (per-store catalog + global catalog at `/p/<slug>-<id>`)
  - `categories` — Spanish category enum, closed set
  - `cities` — slug enum (`ciudad-de-mexico`, `guadalajara`, `monterrey`, `naucalpan`, `coyoacan`, ...)
- Sync cursor: per-(city × store_type) and per-(city × category) timestamp; on re-sync, diff against last snapshot to detect new/removed/price-changed items.
- FTS/search: `restaurants`, `stores`, `menu_items` (name, description, category). Powers offline "find tacos near Coyoacán" and "compare Coca-Cola 600ml" queries.

## Codebase Intelligence
- **No first-party Rappi MCP server exists** (Apify hosts paid shim MCPs but no standalone). First-party Rappi MCP/CLI is an open niche.
- **No actively maintained unofficial wrapper.** Closest signal: `ingmpesca/rappi-webScraping` (Selenium, 4 commits, abandoned), `luminati-io/rappi-price-tracker` (Bright Data tutorial-grade).
- **Merchant API** at `dev-portal.rappi.com` uses Bearer via `x-authorization` header but is OAuth-gated to your own store only — does not expose public catalog. Out of scope for v1.
- **Auth pattern for public surface:** anonymous, geo headers (lat/lng) required for JSON endpoints, optional `Cookie` for SSR personalization. No tokens.

## User Vision
- Not captured (user used "no-stopping" directive; briefing question skipped).

## Source Priority
- Single-source (rappi.com.mx). No combo CLI, no inversion concerns.

## Product Thesis
- **Name:** `rappi-pp-cli` (machine slug `rappi`)
- **Why it should exist:** The Rappi mobile/web app hides cross-store comparison, menu archival, multi-axis sort, and bulk catalog access. A read-only CLI + MCP fills the gap with FTS search, SQLite snapshots, and agent-native output. The killer use case is "cheapest 600ml Coca-Cola within 5km of Polanco right now" — currently a 6-tab manual exercise.

## Build Priorities
1. `restaurants list` — restaurants near lat/lng with `--category`, `--city`, `--sort`, `--prime`, `--free-delivery`
2. `stores list` — supermarkets/pharmacy/liquor/express with `--type`, `--city`, `--lat/--lng`, `--sort`
3. `restaurant menu <id>` — full restaurant menu/catalog snapshot
4. `store catalog <id>` — full store catalog snapshot
5. `search <query>` — cross-store product search scoped to a city/lat/lng
6. `categories` — enumerate store types and restaurant categories (offline, closed enum)
7. `cities` — enumerate served cities with default coordinates (offline lookup)

## Auth / Geo Defaults
- `--lat` / `--lng` (or `RAPPI_LAT`/`RAPPI_LNG` env) is mandatory for any geo-scoped command; default to CDMX zócalo (19.4326, -99.1332) when neither is set.
- `--city` short-circuits to a baked-in city → coordinates lookup:
  - `ciudad-de-mexico` → (19.4326, -99.1332)
  - `guadalajara` → (20.6597, -103.3496)
  - `monterrey` → (25.6866, -100.3161)

## Implementation Notes
- **HTML-first parsing of SSR pages is the v1 transport.** All store and category pages render data server-side.
- **JSON-endpoint sniffing is a phase-2 optimization** behind `--mode json` with warnings about likely 403s from datacenter IPs.
- **No login, no cart, no order tracking in v1.** Mark as explicit non-goals.
- **Mobile API reverse-engineering is off-limits** (signature reversal is brittle).
- Spanish content stays Spanish (no auto-translation in v1); ratings/prices are normalized (MXN, decimal points).
