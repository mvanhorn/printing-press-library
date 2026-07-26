# Flipkart CLI Brief

## API Identity
- Domain: Flipkart (flipkart.com) — India's largest e-commerce marketplace. No general-purpose public API for arbitrary product lookup exists. Two real surfaces exist:
  1. **Flipkart Affiliate API** (official, https://affiliate.flipkart.com/api-docs/) — requires an approved affiliate account. Auth via `Fk-Affiliate-Id` + `Fk-Affiliate-Token` headers. Endpoints: Product Feed (by category), Delta Feed, Search by keyword, Search by product ID, Order Report (affiliate commission tracking, NOT personal purchase history).
  2. **The live website** (flipkart.com) — no official API, but a large community of scrapers exists (Python/Node/Rust) extracting product search results and product-detail pages. A direct fetch (WebFetch, no browser) returned HTTP 403, consistent with basic bot-detection; community scrapers report inconsistent blocking but multiple actively-referenced projects still work via plain HTTP + realistic headers, no full browser required.
- Users: shoppers comparing prices/specs/reviews before buying; deal-hunters tracking price drops and bank/card offers; affiliate marketers building product feeds.
- Data profile: product catalog (title, price, MRP, discount %, images, specs, brand, category), ratings/review counts, seller info, stock/COD availability, bank/card offer strings on product pages. Explicitly OUT of scope: personal order history / account data — Flipkart has no API for this and it requires an authenticated personal account session, which the user confirmed they do not want built.

## Reachability Risk
- Low-Medium. A raw unauthenticated `WebFetch` GET to `flipkart.com/` returned 403 (basic bot signal, not necessarily Cloudflare/DataDome-grade — no challenge page evidence yet). Multiple community scrapers (`mdvenukumar/flipkart-scraper`, `atharao/flipkart-scraper`, `dvishal485/flipkart-scraper` — Rust rewrite) use plain `requests`/HTTP + BeautifulSoup HTML parsing with realistic headers and report working results; `mdvenukumar/flipkart-scraper` has 0 open issues. One prior hosted scraper API (`dvishal485/flipkart-scraper-api`) was taken down not due to blocking but "over-exploitation... lack of funds & free-tier limitations" — a hosting/cost decision, not evidence Flipkart hard-blocks the underlying technique. `probe-reachability` will be run in Phase 1.9 to confirm the actual transport mode (standard HTTP vs browser-required) before generation.

## Top Workflows
1. Search products by keyword and get a ranked, filterable results list (price, rating, discount).
2. Fetch full product details from a URL or product ID (spec sheet, price history context, images, brand, stock).
3. Compare 2+ products side by side (price, rating, key specs).
4. Track a product's price over time and alert on drops (requires local persistence — a scraper alone can't do this; a CLI with SQLite can).
5. Surface bank/card offers and discount codes attached to a product page.
6. (Affiliate-key users only) Pull category-wide product feeds and delta feeds for bulk cataloging.

## Table Stakes
- Keyword search with pagination (every scraper has this).
- Single product detail fetch by URL or ID.
- Price, MRP, discount %, rating, review count, image URLs, brand, category path.
- Stock/COD availability.
- CSV/JSON export (several scrapers already do CSV).
- Affiliate-API mode: category feeds, delta feeds, search-by-ID, search-by-keyword (official, documented, needs key).

## Data Layer
- Primary entities: `products` (by product_id/url), `search_results` (query -> ranked product_id list, timestamped), `price_history` (product_id, price, observed_at), `offers` (product_id, offer_text, bank/card, observed_at).
- Sync cursor: no server-side "since" cursor exists for scraped data — sync means "re-fetch and snapshot," driving price_history accumulation locally. Affiliate Delta Feed API does have a real `fromVersion` cursor for category feeds.
- FTS/search: FTS5 over product title/description/brand for offline search across everything ever fetched.

## Reachability / Auth Intelligence
- No login required for product search/details — public browsing surface.
- User confirmed: logged into Flipkart in browser (useful for browser-sniff capture quality/session longevity), but order/account data is explicitly out of scope, so no authenticated endpoints will be targeted.
- Affiliate API requires a separate approval process outside this session; user has no key today. CLI will support it as an optional, superior auth mode (`FLIPKART_AFFILIATE_ID` / `FLIPKART_AFFILIATE_TOKEN`) for users who later get approved, with browser-sniffed direct-HTTP as the default no-key path.

## Product Thesis
- Name: Flipkart CLI (flipkart-pp-cli)
- Why it should exist: every existing tool is a single-purpose scraper script (Python or Node, one file, no persistence, no comparison, no price tracking, no offer parsing). None combine search + detail + price-history + offer-parsing + local SQLite + offline search + optional official Affiliate-API mode in one agent-native tool. This CLI absorbs every scraper feature found in research and adds price-history tracking and card-offer arbitrage, which no scraper (all stateless single-shot scripts) can do.

## Build Priorities
1. Data layer (products, search_results, price_history, offers) + sync + FTS5 search — foundation for every downstream feature.
2. Absorb every scraper feature: search, product detail, CSV/JSON export, brand/category filtering, stock/COD flags.
3. Transcendence: price-drop tracking/alerts, multi-product compare, bank-offer/card arbitrage finder, category deal scanner — all only possible because of local persistence across runs.
