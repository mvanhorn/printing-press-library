# Daraz CLI Brief

## API Identity
- Domain: **Daraz.pk** — Pakistan's largest online marketplace (Alibaba / Lazada group). Shopper-side product discovery.
- Users: Pakistani online shoppers, deal hunters, resellers, price-comparison users, agents doing product research.
- Data profile: product listings (price, discount, rating, sold count, seller, brand, location, stock), product detail (specs, SKUs/variations, description, images), customer reviews + rating distribution. Search returns 40 items/page with `totalResults` reported (e.g. "laptop" → 4080).

## Reachability Risk
- **None.** `cli-printing-press probe-reachability` → `mode: standard_http`. Bare `curl` with **no User-Agent and no cookies** returns HTTP 200 JSON. Homepage returns 200 SSR HTML, no captcha/Cloudflare/DataDome wall.
- Probe-safe endpoint used: `GET /catalog/?ajax=true&q=laptop&page=1` → 200, JSON.

## Top Workflows
1. Search products by keyword; sort + filter; compare prices across listings.
2. Inspect a product: price, variations/SKUs, specs, seller, stock, images.
3. Read reviews + rating distribution before buying.
4. Track a product's price over time; catch genuine price drops and real deals.
5. Vet a seller (rating, catalog breadth, price/discount patterns).

## Table Stakes (from competitors)
- Keyword search → structured fields (name, price, original price, discount %, rating, review count, sold count, seller, brand, location, URL, image, itemId)
- Sort (price asc/desc, popularity, newest, top-sales, ratings)
- Filter (category, brand, price range, rating, shipped-from, free/express delivery, condition)
- Pagination with total count
- Product detail extraction
- Reviews extraction
- CSV / JSON output

Competing tools surveyed: oxylabs/lazada-scraper (search + detail + reviews + seller, paid SaaS), zohaibbashir/Daraz.pk-Web-Scraper (search → CSV), kazalnsl/daraz-scraper (search → CSV: name/price/discount/rating/seller), Yeasir-Hossain/daraz-scrapper (search → JSON), abdulalikhan/DarazPK-API (Flask product info). All are one-shot scrapers; none keep local state.

## Data Layer
- Primary entities: `products` (itemId, skuId), `sellers` (sellerId), `reviews` (reviewRateId), `price_snapshots` (itemId, ts, price), `saved_searches`.
- Sync cursor: per-query page sweep; per-item price snapshot timestamp.
- FTS/search: product name / brand / seller full-text over the local mirror.

## Source Priority
- Single source: the **daraz.pk website** (replayable JSON + SSR HTML). No official shopper API; the Daraz Open Platform is seller-side and out of scope.

## Reachability / endpoint evidence (replayable, standard_http)
- **Search JSON:** `GET https://www.daraz.pk/catalog/?ajax=true&q=<kw>&page=<n>&sort=<sort>` → `{mods.listItems[40], mods.filter.filterItems[15 facets], mods.sortBar.sortItems, mainInfo{totalResults,pageSize,page,currency,noMorePages}}`
- **Reviews JSON:** `GET https://my.daraz.pk/pdp/review/getReviewList?itemId=<id>&pageSize=<n>&filter=0&sort=0&page=<n>` → `{success, model:{items[], ratings, paging, item}}`
- **Product detail:** `GET <itemUrl = .../products/...-i<itemId>.html>` → SSR HTML, `window.__moduleData__` JSON island (skuBase, skuInfos, specifications, description, seller). Needs a balanced-brace extractor (Phase 3 hand-code; not a clean typed endpoint).
- **Autocomplete:** behind the signed mtop gateway (`acs.daraz.pk`, requires request signing) — out of scope for v1.

## Product Thesis
- Name: `daraz-pp-cli` (display: **Daraz.pk**)
- Why it should exist: Every existing Daraz tool is a one-shot scraper that dumps a CSV. None keep a local SQLite history, so none can answer "did this price *actually* drop or was the discount faked?", "which seller is genuinely cheapest **and** best-rated?", or "what's new since I last checked this search?". A local-store, agent-native CLI turns Daraz from a browse-only site into a queryable, watchable price database — search, detail, and reviews are table stakes; the local price history + deal/value scoring + drop alerts are what no competitor has.

## Build Priorities
1. `products search` (+ sort/filter/paginate) — foundation
2. `products get` (detail) + `reviews`
3. local store + `sync` + price snapshots
4. transcendence: price-history, deals ranking, drop alerts, seller scorecard, value/"real-discount" detector
