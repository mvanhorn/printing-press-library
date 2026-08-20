# Discovery Report — Daraz.pk

**Method:** Direct HTTP probing (the browser-sniff gate was pre-approved via the Phase 0 "website itself" choice). A resident browser was not required: every needed surface is reachable with plain HTTP. `cli-printing-press probe-reachability` classified the search endpoint as `standard_http`, and a bare `curl` with no User-Agent and no cookies returned HTTP 200 JSON. browser-use / agent-browser were installed for the gate but not needed for this run.

**Primary user goal (Phase 1.7 Step 0):** "Search Daraz for a product and compare price/rating across listings." Read-only, search/filter/paginate shape.

## Replayable surfaces discovered

### 1. Search (JSON) — PRIMARY
- `GET https://www.daraz.pk/catalog/?ajax=true&q=<keyword>&page=<n>&sort=<sort>` (+ optional filter params)
- Response: `application/json`, keys `templates, mods, mainInfo, seoInfo`.
  - `mods.listItems[]` — up to 40 products/page. Per item: `itemId, skuId, name, price, originalPrice, originalPriceShow, priceShow, discount, ratingScore, review, itemSoldCntShow, location, sellerName, sellerId, brandName, brandId, inStock, image, itemUrl, nid, skus`.
  - `mods.filter.filterItems[]` — 15 facets (Category, Brand, Price, Rating, Shipped From, Color, Condition, Express delivery, Warranty, ...).
  - `mods.sortBar.sortItems[]` — sort options.
  - `mainInfo` — `totalResults, pageSize(=40), page, currency, noMorePages, allProductURL, cate_id`.
- Sort values confirmed: default (best match), `priceasc`, `pricedesc` (others: popularity, newest, top-sales, ratings via `sortBar.sortItems`).
- Pagination: `page` param; total pages = ceil(totalResults / pageSize).
- Auth: none.

### 2. Reviews (JSON)
- `GET https://my.daraz.pk/pdp/review/getReviewList?itemId=<itemId>&pageSize=<n>&filter=<0|...>&sort=<0|...>&page=<n>`
- Host is `my.daraz.pk` (www.daraz.pk 301-redirects this path).
- Response: `application/json`, `{success, model:{items[], ratings, paging, item, mediaList}}`.
  - `model.items[]` per review: `rating, reviewContent, reviewTitle, buyerName, reviewTime, boughtDate, upVotes, likeCount, images[], skuInfo, helpful, relevanceScore`.
  - `model.ratings` — rating distribution (1–5 star counts + average).
  - `model.paging` — total + page size.
- Auth: none.

### 3. Product detail (SSR HTML + JSON island)
- `GET https://www.daraz.pk/products/<slug>-i<itemId>.html`
- Returns 200 HTML containing `window.__moduleData__ = {...}` (also `app.run(...)`) with `skuBase`, `skuInfos`, `data`, specifications, description, seller, breadcrumbs, images.
- Extraction: balanced-brace JSON-island parse (the island is large, ~28KB+; a naive regex over-captures). Implemented as a Phase 3 hand-coded command (`response_format: html` with custom extraction), not a typed JSON endpoint.
- Auth: none.

## Not pursued
- **Autocomplete / suggestions** — served by the signed mtop gateway (`acs.daraz.pk`, requires `x-mini-wua`/sign params). Out of scope v1; search covers the discovery need.
- **Cart / checkout / order / wishlist** — require a logged-in session. Out of scope for a read-only shopper-research CLI v1.

## Transport decision
- `mode: standard_http`. Generated CLI ships plain Go `net/http` transport. A Chrome-like `User-Agent` header is set defensively but not required. No Surf, no browser-clearance cookies, no resident browser.
