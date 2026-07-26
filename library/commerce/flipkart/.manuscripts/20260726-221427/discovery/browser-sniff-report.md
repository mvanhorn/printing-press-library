# Flipkart Browser-Sniff Discovery Report

## Goal
Primary browser-sniff goal: search for a product and view its details (read-only, anonymous — no login required).

## Reachability
`printing-press probe-reachability https://www.flipkart.com` returned:
- stdlib HTTP: 403, evidence "CAPTCHA widget"
- surf-chrome (Chrome TLS fingerprint): 200
- **mode: browser_http** — runtime settled: ship Surf/Chrome-impersonated transport (`--transport browser-chrome`). No clearance cookie or live browser needed at runtime.

## Architecture Findings
Flipkart's web app is server-rendered via an internal "widget/screen" system (`multiWidgetPage`, screen names like `OV_HOMEPAGE`), NOT a client-side SPA with a discoverable JSON XHR API. Fetch/XHR/Performance-API interception during interactive browsing (homepage load, search submission, product page load) surfaced only telemetry endpoints (New Relic `bam.nr-data.net`, Flipkart's own analytics collectors `sonic.fdp.api.flipkart.com`, `rome.api.flipkart.com`) — no product-data JSON endpoint exists to replay directly.

**The reliable extraction surface is schema.org JSON-LD embedded in a `<script id="jsonLD" type="application/ld+json">` tag on both search and product pages:**

- **Search results page** (`GET /search?q=<query>&page=<n>`): JSON-LD is a schema.org `ItemList` — ordered `{name, url, position}` per result. Verified live: search for "wireless earbuds" returned properly ordered, titled, linked results.
- **Product detail page** (`GET /<slug>/p/<itemId>?pid=<pid>`): JSON-LD is a schema.org `Product` with `name`, `description`, `image[]`, `sku`, `color`, `category`, `brand.name`, `aggregateRating.{ratingValue,reviewCount,ratingCount}`, `offers.{price,priceCurrency,availability,itemCondition,shippingDetails,hasMerchantReturnPolicy}`, and `review[]` (author, date, body, rating). Verified live against a real product page (OPPO Enco Buds3 Pro).

**Not in JSON-LD (DOM-only, requires hand-coded text extraction, not the generator's built-in html_extract engine):**
- MRP (strikethrough original price) and discount percentage — rendered as plain page text (`document.body.innerText` regex `/[0-9]+% ?OFF/i` matched reliably).
- Bank/card offers — a distinctly-labeled "Bank offers" DOM section with a repeating `<amount> off / Apply / <bank+card+type>` text pattern. Verified live.

**Product URL constraint:** Flipkart product URLs require the full descriptive slug — `GET /p/<itemId>?pid=<pid>` (slug omitted) returns 404. There is no bare-ID canonical URL. This means "product get" must accept the full product URL as input, not a short ID — consistent with how every community scraper CLI already works.

## Generator Fit
- `response_format: html` + `html_extract: {mode: embedded-json, script_selector: "script#jsonLD"}` cleanly spec-emits the **search** endpoint (query-string params only, no path-escaping issue).
- The generator's per-request path-param substitution (`replacePathParam`) applies `url.PathEscape` to positional path values, which would corrupt a full product-URL-as-path-segment (slashes become `%2F`). **Product detail fetch and bank-offer extraction are therefore hand-coded commands** (Phase 3) that call the same generated `extractHTMLResponse`/`extractEmbeddedJSON` helpers directly against a raw HTTP GET of the user-supplied URL, rather than being spec-emitted endpoints. The `search` endpoint's spec declaration still triggers generation of those shared helper functions (`HasHTMLExtractMode("embedded-json")`).

## Official Affiliate API (separate, optional tier)
No browser-sniffing needed — fully documented at https://affiliate.flipkart.com/api-docs/. Modeled as a `tier_routing` optional tier (`auth.type: api_key`, header `Fk-Affiliate-Id`, plus `additional_headers` for `Fk-Affiliate-Token`), gated behind `FLIPKART_AFFILIATE_ID` / `FLIPKART_AFFILIATE_TOKEN`, default tier remains `none` (free/public browsing surface).

## Rate/Pacing
Only 2 page loads + 1 search performed during discovery (homepage, search results, one product page) — well under any rate-limiting concern. No 429s observed.
