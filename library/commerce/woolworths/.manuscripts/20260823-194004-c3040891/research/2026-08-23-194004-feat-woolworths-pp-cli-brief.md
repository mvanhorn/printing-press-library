# Woolworths (AU) CLI Brief

## API Identity
- **Domain:** Australian grocery retail. woolworths.com.au — catalogue, pricing, specials, trolley, stores.
- **Users:** AU household shoppers optimising a weekly grocery spend; bargain hunters tracking
  half-price cycles (OzBargain / GroceryWise culture); agents building shopping lists.
- **Data profile:** large catalogue, 25 top-level departments. Every product tile carries shelf
  price, was-price, savings, unit ("cup") price, barcode, availability, and special flags.
  Prices move on weekly promo cycles — the data is inherently a *timeseries*, but the API only
  ever exposes "now".

## Official API
**None exists for consumer data.**
- `developer.woolworths.com.au` (the old Developer Network) is DNS-dead — no A record. Blog posts
  claiming "Woolworths has a public API" all trace back to this defunct portal.
- `apiportal.woolworths.com.au` is live but internal/B2B (Supply Chain, SAP, IAM, People &
  Culture). No product/price/specials/store API; access routed through internal ITSM.
- Partner Hub is trade-supplier onboarding. Everyday Rewards has no developer program.
- Context: the ACCC has *recommended* compelling the major supermarkets to expose live prices via
  API. Recommended, not legislated.

Therefore this CLI is built on the site's own internal `/apis/ui/*` web surface, discovered and
verified live.

## Reachability Risk
**LOW — but the entire community has misdiagnosed it, which is itself the opportunity.**

Consensus is "Woolworths blocks scrapers." The cited evidence is real: `nwbort/stores-woolworths`
runs a daily cron and has committed Akamai `Access Denied` HTML for **269 consecutive days**
(since 2025-11-27). OzBargain threads document PriceHipster and DiscountKit being blocked.

**But that cron sends no `User-Agent` at all.** Verified behaviour, this host, 2026-08-23:

| Request | Result |
|---|---|
| `/apis/ui/*` bare curl, no UA | **403** Akamai HTML (`errors.edgesuite.net`) |
| same + browser `User-Agent` | **200 JSON** |
| POST endpoints, UA but no cookie jar | **hang -> timeout / connection reset** |
| POST endpoints, UA + jar warmed by one `GET /shop` | **200**, full payloads |

- Cookies from a single plain GET: `bm_sz`, `bm_so`, `_abck`, `ak_bmsc`, `bm_mi`,
  `akaalb_woolworths.com.au`, `bff_region`. **No JS sensor execution required.**
- Verified independently across curl and Node (two TLS stacks) — no JA3/fingerprint gating, so Go
  `net/http` + `cookiejar` is sufficient. No utls, no Playwright, no proxies.
- Protected endpoints **do not 403 — they silently hang**. Naive clients fail by timeout and
  conclude "network down". The CLI must treat a hang as "re-prime cookies", not as an outage.
- The real constraint is **crawl breadth, not entry**: a Whirlpool user scraped fine until they
  hit every category, then got IP-banned. 12 rapid sequential requests all returned 200, no
  throttling. Ship a rate limiter and cache; do not walk the whole catalogue.
- `robots.txt` does not disallow `/apis/`; it does disallow `/shop/search`, `/shop/mylists`,
  `/shop/myaccount`, `/checkout`. Site Terms of Use are a separate document.

## Dead ends (do not build on these)
- **Mobile API `prod.mobile-api.woolworths.com.au`** — `drkno/au-supermarket-apis` publishes an app
  key and ~35 decompiled Retrofit signatures (shopping-list CRUD, barcode/EAN lookup, stores,
  fulfilment, past-shop, shopper preferences). Independently probed twice: every endpoint returns
  **401 `invalid_client`**. The published key is revoked. This is the surface ecosystem research
  called "the biggest greenfield" — it is closed.
- **Everyday Rewards** (`api.woolworthsrewards.com.au`, GraphQL `apigee-prod.api-wr.com`) —
  programmatic login was removed in favour of mandatory MFA; tokens last ~30 minutes and must be
  harvested by hand from DevTools. Probed -> 401 `Access token is empty`.
  **Not automatable. Do not promise points, boosters, or e-receipts.**

## Verified endpoint surface (all live 200s, 2026-08-23)

| Endpoint | Method | Auth | Notes |
|---|---|---|---|
| `/apis/ui/Search/products` | POST & GET | none | PascalCase body; `Products[].Products[]` double-nested |
| `/apis/ui/v2/Search/count` | GET | none | **Nobody implements this.** ProductCount / SpecialProductCount / RecipeCount / SuggestedTerm, ~514 B |
| `/apis/ui/search-suggestions/suggestionsb2c` | GET | none | **Nobody implements this.** Ranked suggestions + autocorrect |
| `/apis/ui/product/detail/{stockcode}` | GET | none | 115 fields + Nutrition, Variants, CountryOfOrigin |
| `/apis/ui/products/{sc1,sc2,...}` | GET | none | Batch fetch, comma-separated |
| `/api/v3/ui/schemaorg/product/{id}` | GET | none | Independent JSON-LD path; survives blocks differently |
| `/apis/ui/browse/category` | POST | none | camelCase body (inconsistent with Search); `TotalRecordCount` |
| `/apis/ui/PiesCategoriesWithSpecials` | GET | none | 25 departments + live specials taxonomy |
| `/apis/ui/StoreLocator/Stores` | GET | none | StoreNo, address, lat/long, TradingHours, Facilities |
| `/apis/ui/Trolley` | GET | none | Works anonymously (guest cart in cookie jar) |
| `/apis/ui/Trolley/Items` | POST | none | Add item; verified 200 + echo |
| `/api/ui/v2/bootstrap` | GET | none | Config, CurrentVersion, ContextExpiryPeriod |
| `/apis/ui/settings` | GET | none | 323 KB feature flags |

**Confirmed 404 — do not implement:** `Search/AutoComplete`, `Fulfilment/GetTimeSlots`,
`Shopper/Lists`, `Trolley/GetTrolleyItems`, `PiesProductDepartmentsJson`, `SeoMetatags`.

### Conventions
- `pageSize` **hard cap 36** — server returns 400 `"Page size should not be greater than the limit: 36"`.
- `PageNumber` 1-based; totals via `SearchResultsCount` (search) / `TotalRecordCount` (browse).
- `SortType`: `TraderRelevance | PriceAsc | PriceDesc | Name | CUPAsc | CUPDesc`.
  **`CUPAsc` / `CUPDesc` sort by unit price server-side** — cheapest-per-100g is one flag.
- Request casing is inconsistent (Search PascalCase, browse camelCase); responses always PascalCase.
- Errors: `{"ResponseStatus":{"ErrorCode":...,"Message":...}}`. Unknown path -> 404 plain `404`.
- `bm_sz` short-lived (hours); `_abck` ~1 year. Re-prime on hang or 403.

### Live specials taxonomy (from PiesCategoriesWithSpecials)
| NodeId | Description | ProductCount |
|---|---|---|
| `specialsgroup.3676` | Half Price | 1,655 |
| `specialsgroup.3673` | Everyday Low Price | 2,895 |
| `specialsgroup.3668` | Buy More Save More | 1,052 |
| `specialsgroup.3694` | Lower Shelf Price | 820 |
| `specialsgroup.3721` | Seasonal Price | 343 |

## Top Workflows
1. **Half-price cycle tracking** — highest-value pattern. 1,655 half-price items live right now,
   flagged `IsHalfPrice` + `WasPrice` + `SavingsAmount`. Woolworths rotates predictably.
2. **Unit-price comparison** — `CupPrice` / `CupMeasure` / `CupString` are first-class and
   server-sortable. "Cheapest per 100 g in this category" is a one-command answer.
3. **Genuine-vs-fake special detection** — snapshot price over time to catch was-price gaming.
   This is the most-documented user grievance and no agent-facing tool does it.
4. **Build a trolley from a shopping list** — search each line, add via `Trolley/Items`. Works
   anonymously.
5. **Barcode -> product** — every product carries `Barcode` + `GtinFormat` for pantry tooling.
6. **Store-aware availability** — `StoreLocator/Stores` + per-product fulfilment fields.

## Table Stakes (from the incumbent, elijah-g/Woolworths-mcp, 12 MCP tools)
Product search, product detail, specials listing, category tree, trolley read, trolley
add / remove / update-quantity, and session establishment.

## Data Layer
- **Primary entities:** products (PK `Stockcode`, alt key `Barcode`), categories/departments
  (`NodeId`), specials groups, stores (`StoreNo`), trolley items, price observations.
- **Sync cursor:** per-category `PageNumber` walk with `TotalRecordCount`; snapshot timestamp per
  price observation.
- **FTS/search:** product name + brand + description -> FTS5 for offline search.
- **The compounding asset is `price_observation(stockcode, ts, price, was_price, is_half_price,
  cup_price)`.** Everything differentiating derives from it: real-special detection, price
  history, cycle prediction. The API cannot answer any historical question; a local store can.

## Product Thesis
- **Name:** `woolworths-pp-cli`
- **Why it should exist:** Woolworths tells you today's price and a "SAVE $X" badge, and the badge
  is exactly what shoppers say they cannot trust. Every existing tool either dumps raw
  today-prices into an LLM (elijah-g, 12 tools, needs Puppeteer) or keeps real price history with
  no agent interface (auscost / Grocermatic). Nothing does both. A pure-Go binary that needs no
  browser, keeps its own price history, and can tell a genuine half-price from a was-price
  inflation occupies a slot that is verifiably empty — there is **no Woolworths Claude Code skill
  and no Home Assistant integration anywhere**.

## Build Priorities
1. Cookie-warming HTTP transport + rate limiter (the thing everyone else needs Puppeteer for).
2. Data layer: products, categories, stores, price observations + FTS5.
3. Absorbed surface: search, detail, batch, category tree/browse, specials, stores, trolley R/W.
4. The two unimplemented cheap endpoints: search-count and autocomplete.
5. Transcendence: price history, genuine-special detection, unit-price ranking, half-price cycle
   tracking — all requiring the local store.

## Open decision
Authenticated surfaces (real shopping lists, order history) are approved for browser-sniff. The
user has a Chrome tab ready; capture proceeds against the running browser. Catalogue + guest
trolley stand on their own if the sniff yields nothing replayable.
