# MotoHunt CLI Brief

## API Identity
- Domain: motohunt.com (motorcycles) + atvhunt.com (ATV/UTV/SxS) — used-vehicle marketplace aggregator pulling dealer inventory nationwide.
- Users: motorcycle/ATV buyers hunting deals by make/model/style/location; the differentiator is MotoHunt's per-listing **price research** (Base MSRP, Average Listing Price, deal rating).
- Data profile: medium volume, mutable (inventory turns over daily), realtime=false, search need=high.
- **No public API.** No OpenAPI, no GraphQL, no JSON endpoints for listings, no `__NEXT_DATA__`, no Product/Vehicle JSON-LD (only a generic `WebSite` schema). Pure server-rendered HTML (jQuery + Bootstrap 4.3). `/api/*` endpoints are utility/auth/tracking only (`savebike`, `signin`, `c`, `t`) — none serve listing data.

## Reachability Risk
- **None.** `probe-reachability https://motohunt.com/motorcycles-for-sale` → `standard_http`, confidence 0.95, "no protection signals". Plain `curl` returns HTTP 200 on every content page. No Cloudflare/WAF/bot wall on listing/search/detail pages (only the `/api/t` tracking beacon rejects headless with `{"ok":false,"msg":"Bot"}` — irrelevant to scraping).
- Transport: `standard` HTTP + a desktop `User-Agent`. No browser needed.

## Source Priority
- Primary: motohunt.com (motorcycles) — headline commands. No official spec; hand-authored internal YAML from completed recon.
- Secondary: atvhunt.com (ATV/UTV/SxS) — **byte-identical platform** (same `/static/mh.js`, same `sc-title` cards, same `/l/{id}` detail, same sort/start params). Exposed via `--site moto|atv`; only the domain + base search path differ (`/motorcycles-for-sale` vs `/atv-utv-for-sale`).
- Economics: both free, no auth. No paid tier.

## The "API" = URL grammar (reverse-engineered)
Search = `GET {base}{searchPath}[/{Facet}]?{query}` → HTML.
- Free text: `?q=harley+davidson`
- Location: `?location=33705` (US zip; sorts by distance)
- Path-segment facet (single value): Style / Make / Make-Model / State, e.g. `/motorcycles-for-sale/Cruiser`, `/Harley-Davidson`, `/BMW-S-1000-RR`, `/Florida`. q/location/sort/start ride as query params alongside.
- Sort: `?sort=t` recent · `?sort=p` high$ · `?sort=a` low$ · `?sort=c` best-deal.
- **Pagination: `?start=N` offset, 24/page** (`page`/`p`/`offset`/`skip` IGNORED — verified `start=24` returns a distinct page, others return page 1).
- Detail: `/l/{id}/{slug}` (slug optional).
- Enumeration: `/model-selector?...` returns an HTML fragment of the make→model→trim cascade; the homepage/SRP "Browse by Make/Style/State" blocks are the canonical fallback enumerations.

## Field extraction (goquery selectors — verified against live HTML)
- **Search card** (`#srp-results-container .card`): id (`span.save-bike[postId]` `p:13194784` → strip `p:`), title (`.sc-title`), price (when listed), mileage (`.sc-line2` Mileage row → `3,969m`), badges (`.sc-badges-container .badge` → `Great Price`/`Good Price`/`Low Mileage`), location (`.sc-loc-icon-div`), dealer, image (`img.srp-listing-img`), deal-rating class (`sc-pr-true|false`).
- **Detail** (`/l/{id}`): title, subtitle, certifiedPreOwned, location, condition, dealer, price (or "visit dealer"), mileage, color, age, stock#, VIN, description, + **MotoHunt Price Research** (baseMSRP, alp, dealRating), images. Best matched by visible label text (`VIN:`, `Mileage:`, `Stock #:`, etc.) → sibling value.

## Top Workflows
1. Search deals by make/model/style + location, sorted best-deal, paged past 24.
2. Pull a single listing's full detail + price research (is it actually a good price?).
3. Enumerate valid makes/models to drive precise searches without guessing slugs.
4. Watch a saved search over time → new listings + price drops.
5. Same four, for ATVs via `--site atv`.

## Table Stakes (what a buyer expects)
- Filter by make/model/style/state/zip; sort by price + best-deal; paginate full result set.
- Per-listing specs (mileage, VIN, color, condition, dealer, location).
- Price context (MSRP vs ALP vs ask) — MotoHunt's actual differentiator.

## Data Layer
- Primary entities: Listing (card), ListingDetail (full + priceResearch), SavedSearch (watch query), Snapshot (watch history).
- Search/FTS: local SQLite mirror of synced listings for offline search + watch diff.
- Sync cursor: `start` offset paging until a page returns 0 cards.

## Product Thesis
- Name: MotoHunt CLI (`motohunt-pp-cli`).
- Why it should exist: token-cheap, agent-native access to a marketplace with no API. One HTTP GET per page, goquery parse, structured JSON out. The price-research data (MSRP/ALP/deal-rating) makes "is this a good deal?" answerable from the terminal/an agent — no other tool exposes it. Covers two sister marketplaces (moto + ATV) with one `--site` flag.

## Build Priorities
1. P0 data layer: Listing + ListingDetail types, SQLite mirror, sync via `start` paging.
2. P1 absorb: `search` (q/location/make/style/model/state/sort/start/limit), `get <id>`, `makes`, `models`. Generated html-extract endpoints give the thin link/page surface; hand-written goquery gives the rich card/detail structs.
3. P2 transcend: `deal` (rank synced listings by MSRP-vs-ALP gap), `watch` (saved-search snapshot diff → new listings + price drops), `--site atv` parity.
