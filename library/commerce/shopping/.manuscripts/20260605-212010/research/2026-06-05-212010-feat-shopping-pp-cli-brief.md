# Shopping CLI Brief (LemmeBuyIt API)

## API Identity
- Domain: Aggregated retail product data ("LemmeBuyIt" / Linkscopic). One key unlocks search across 115M+ products from 70+ retailers (Walmart, Home Depot, Target, Nike, Kohl's, Macy's, Sephora, Chewy, ...).
- Users: Resellers / arbitrage sellers, deal hunters, price-tracking agents, e-commerce analysts.
- Data profile: Products (price, discount, brand, identifiers UPC/EAN/GTIN/ASIN/MPN, ratings, stock), weekly price history (retailer + Amazon retail/3P/Buy-Box + monthly_sold), Amazon profitability/FBA fee breakdowns, categories, retailers.
- Base URL: https://api.lemmebuyit.com/v1 (the /v1 is part of the base; do not double it).

## Auth
- Single `X-API-Key` header for EVERY endpoint (apiKey-in-header, no "Bearer" prefix). Env var: `SHOPPING_API_KEY`.
- The OpenAPI spec marks `/status` as unauthenticated, but the live gateway returns 401 "API key required" for it too — ALL paths need the key.
- Free vs paid is PLAN-gated on the same key: free keys reach the `/shopping/` paths; paid keys reach full `/products`, `/price-history`, `/amazon/.../price-history`, and `/categories`. This is NOT a separate-credential split, so no tier-routing machinery is used.

## Reachability Risk
- None. Official vendor spec + live API. Phase 1.9 probe: GET /v1/status -> 401 "API key required" (expected without a key) = PASS.

## Top Workflows
1. Search a retailer's catalog with rich filters (text/identifier/price/discount/rating/brand/category) and paginate by cursor.
2. Look up one product by SKU; pull its weekly price history (retailer + Amazon merged).
3. Pull Amazon weekly price history by ASIN for trend/Buy-Box analysis.
4. Find on-sale / deeply-discounted items, in stock, in a price band.
5. (Reseller) Evaluate Amazon FBA profitability for a product.

## Table Stakes (absorbed)
- List retailers; list categories for a retailer.
- Search/filter products per retailer (free shopping surface + paid full surface).
- Get product detail by SKU (free shopping + paid full).
- Weekly price history by product and by Amazon ASIN.
- API health check.

## Data Layer
- Primary entities: retailers, products (+shopping_products), price_history_points, categories.
- Sync cursor: cursor pagination via `after`/`next_cursor`; `updated_since`/`created_since` for incremental.
- FTS/search: local product name/brand/category search over synced rows; SQL composable.

## Product Thesis
- Name: shopping (shopping-pp-cli)
- Why it should exist: The live API filters one retailer at a time and offers no persistence. A local SQLite store turns it into a cross-retailer deal engine: compare the same UPC across stores, track price-history over time, rank biggest weekly drops, and compute Amazon FBA arbitrage margin — compound queries no single API call can answer. Plus agent-native --json/--select, offline search, typed exit codes.

## Build Priorities
1. Data layer + sync for retailers/products/price-history/categories.
2. Absorb every endpoint (free shopping + paid surfaces) as typed commands.
3. Transcendence: cross-retailer compare, compound deals query, price-drop ranking, price tracking, FBA arbitrage margin, local analytics.
