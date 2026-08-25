# foodpanda CLI Brief

Target chosen in Phase 0: **the foodpanda consumer website**, not the vendor-side
Partner API at developer.foodpanda.com (which requires partner onboarding credentials).

## API Identity
- **Domain:** Food + q-commerce delivery marketplace (Delivery Hero group), APAC-wide.
- **Users:** Consumers ordering food/groceries; price-sensitive comparison shoppers;
  analysts tracking restaurant pricing/availability; anyone who wants menu + fee data
  the web UI shows only one restaurant at a time.
- **Data profile:** Vendor catalog (80+ fields/vendor), full nested menus with prices,
  deals/discounts, cuisines, ratings + reviews, delivery fees/times, opening hours,
  delivery radius. Heavily geo-scoped — every query is anchored to a lat/lng.

## Reachability Risk
- **Low.** Programmatic access works today over plain HTTP with no credentials.
- PerimeterX (`px-captcha`) fronts the site and returns a deterministic **403** to
  bare `curl` (3/3 identical 4599-byte challenge pages). A realistic browser header
  set clears it completely: **200 with 995 KB of real SSR HTML**, zero PX markers.
- `cli-printing-press probe-reachability` → `mode: standard_http`, confidence 0.95
  (stdlib 200, surf-chrome 200). Runtime is plain HTTP; no clearance cookie, no
  resident browser, no Surf requirement.
- **Mitigation the CLI must ship:** a fixed browser-like header set on every request
  (UA, Accept, Accept-Language, sec-ch-ua, sec-fetch-*). Omitting it is the single
  failure mode that turns this CLI into 403s.
- Tier/permission hints from 4xx body: none — the 403 body is a PX captcha shell,
  not a tier/quota message.
- Probe-safe endpoint used: `GET /listing/api/v1/pandora/vendors` (read-only listing).

## Verified Contract (all endpoints probed live, this run)

Required headers on JSON endpoints:
`X-FP-API-KEY: volo`, `x-disco-client-id: web`, `perseus-client-id`,
`perseus-session-id`, browser UA. **perseus IDs are client-generated, not
credentials** — synthesized values (`<unix_ms>.<18 digits>.<8 alnum>`) returned 200.

| # | Endpoint | Verified result |
|---|----------|-----------------|
| 1 | `GET disco.deliveryhero.io/listing/api/v1/pandora/vendors` | 200. 80+ fields/vendor; `aggregations` facet block (22 cuisines, payment_types, quickFilters) |
| 2 | `GET disco.deliveryhero.io/listing/api/v1/pandora/search?query=` | 200, **discriminates**: biryani 63, pizza 19, sushi 5 |
| 3 | `GET {cc}.fd-api.com/api/v5/vendors/{code}?include=menus` | 200. Full menus → categories → products → variations w/ prices; deals, discounts, delivery_conditions |
| 4 | `GET /city/{city}/area/{area}` (SSR HTML) | 200. Carries `latitude`/`longitude` + ~48 vendor links → **native geocoding** |
| 5 | `GET /restaurant/{code}/{slug}` (JSON-LD) | 200. aggregateRating (4.6 / **1104 count**), geo, openingHours, `areaServed.geoRadius: 4000`, priceRange, **`reviews[]` × 30** |

### Landmines found (must not be built naively)
- **`q=` on the vendors endpoint is silently ignored.** `biryani`, `pizza`, and
  `zzzqqqnonsense` all returned the identical 102 vendors. Text search MUST use
  endpoint #2 (`/search?query=`), not `vendors?q=`.
- **`/search` rejects `q`/`search`/`keyword`/`term`** with 400 `"query parameter not
  found"`. The only accepted name is **`query`**.
- **Search is fuzzy and never truly empty** — `zzzqqqnonsense` still returned 18
  results. "No matches" is not expressible; the CLI must not imply exact matching.
- **JSON-LD reviews use the non-standard key `reviews`** (plural), not schema.org's
  `review`. A standards-correct extractor silently reads 0.
- **Vendor codes are market-scoped.** `pk2v` 404s on `sg.fd-api.com`; the SG code
  `fqow` returns 200 there. Never cross market and code.
- **`vertical=darkstores` returns a store but `menus: 0`** — the pandamart grocery
  catalog is a separate q-commerce surface not reachable via `/api/v5/vendors`.

## Top Workflows
1. **Find food near me, ranked by something the UI won't rank by** — filter/sort a
   whole area's vendors by rating, delivery fee, min order, delivery time at once.
2. **Pull a full menu with prices** for one or many restaurants, as JSON/CSV.
3. **Compare the same dish across restaurants** — price/rating/fee spread for
   "biryani" across an area.
4. **Track price + availability over time** — snapshot menus and diff them.
5. **Cross-market comparison** — the same query across pk/bd/sg/my/hk.

## Table Stakes (from competing tools)
- Restaurant search by location; menu + price extraction; ratings/cuisines/delivery
  details — Apify actors (`scrapesage/foodpanda-scraper`, `fatihtahta/food-panda-scraper`,
  deprecated `nowi5/apify-foodpanda-restaurants`) all cover exactly this, paid and
  hosted. `iCHAIT/oloviz` scrapes personal order history.
- Every competitor is a **hosted paid scraper or a one-off Python script**. None is a
  local, agent-native CLI; none has an offline store; none does cross-market.

## Data Layer
- **Primary entities:** vendor, menu_category, product, product_variation, deal,
  review, cuisine, area/city, price_snapshot.
- **Sync cursor:** per (country, lat, lng, vertical) listing sweep + per-vendor menu
  fetch; snapshot timestamp per sync.
- **FTS/search:** products (dish name + description) and vendors (name, cuisines,
  address) — this is what enables cross-restaurant dish search offline, which the
  live API cannot do at all.

## Why this CLI should exist
Every existing option is a paid hosted scraper or a dead Python script. Nothing lets
you keep a **local, queryable mirror** of an area's catalog. The live API cannot
answer "which restaurant near me sells the cheapest biryani", "what changed in this
menu since last week", or "how do delivery fees compare across 100 vendors" — those
require joins across data the API only serves one vendor at a time. That gap is the
product.

## Product Thesis
- **Name:** `foodpanda-pp-cli`
- **Why it should exist:** The only local-first, agent-native foodpanda client —
  turns a geo-scoped, one-vendor-at-a-time web API into a queryable local catalog
  with cross-vendor dish search, price history, and fee comparison no foodpanda
  surface offers, across every market foodpanda runs.

## Build Priorities
1. Browser-header HTTP transport + perseus ID synthesis (everything depends on it).
2. Vendor listing/search/detail + full menu extraction → SQLite store + FTS.
3. Area→coordinate resolution from `/city/{c}/area/{a}` so users never type lat/lng.
4. Cross-vendor dish price comparison; menu diffing over snapshots.
5. Reviews + opening-hours + delivery-radius enrichment from the restaurant JSON-LD.
