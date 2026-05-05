# Redfin CLI Brief

## API Identity
- Domain: US residential real estate — homes for sale (primary), homes sold, rentals, agent search, neighborhood market trends.
- Owner: Redfin Corporation (publicly traded; acquired by Rocket Companies in 2025).
- Users: home buyers comparison-shopping a 3–6 month window; investors tracking market trends; relocators across cities; agents researching comparables; data-driven hunters wanting $/sqft and historical price trajectories.
- Data profile: server-rendered HTML pages with rich `__INITIAL_DATA__` JSON blobs, plus a documented internal **Stingray** JSON/CSV API used by Redfin's own web app (no public dev portal, no key, no versioning).

## Reachability Risk
- **Medium.** `printing-press probe-reachability https://www.redfin.com` returned `mode: browser_clearance_http, confidence: 0.6`. Both stdlib and Surf-Chrome got 200 OK on the homepage with an AWS WAF marker (the marker is ambient, not a block). Several community scrapers (reteps/redfin, dreed47/redfin) have GitHub issues reporting 403s when scraping at scale, which suggests aggressive per-IP rate limiting more than a hard bot wall.
- Mitigation: ship Surf with Chrome TLS fingerprint at runtime + `cliutil.AdaptiveLimiter` for back-off. The Stingray endpoints are geo-restricted to US IPs; non-US users will get blocks regardless of fingerprint.

## Top Workflows
1. **Search homes for sale by city + filters** — beds, baths, price, sqft, lot size, year built, property type, schools.
2. **Inspect one listing** — full details: price, beds/baths, sqft, lot, year built, MLS#, history, AVM (Redfin Estimate), tax, schools, photos, virtual tour.
3. **Track sold-prices in a market** — Redfin's "sold" filter shows actual transaction prices; the only way most consumers can see this for free.
4. **Watch a saved search over time** — diff against last sync; surface NEW listings, REMOVED, PRICE-CHANGED, STATUS-CHANGED (active → pending → sold).
5. **Pull market trends for a neighborhood** — median sale price, days on market, list-to-sale ratio, supply, by month/quarter (Redfin's `aggregate-trends` endpoint).
6. **Discover agents in a market** — agent profiles, recent transactions, reviews.

## Table Stakes (from competing tools)
- Search by city / zip / neighborhood with all standard filters
- Filter by status: for-sale, sold, pending, coming-soon
- Filter by property type: single-family, condo, townhouse, multi-family, land
- Min/max price, min beds/baths, min sqft, year-built range
- Pagination
- Listing detail extraction (price history, tax history, school assignments, photos)
- Bulk CSV export (Redfin's UI offers this; gis-csv endpoint backs it; capped at 350 rows)
- Sold homes (last 1/3/6 months / 1/2/3 years)
- Open houses, virtual tours, video tours
- Walk Score / Transit Score
- Polygon-bounded map searches

## Data Layer
- Primary entities: `listing` (id, url, address, price, beds, baths, sqft, lot, year_built, mls#, status, type, listing_data JSON), `region` (id, type, name, state), `region_market_snapshot` (region+month, median_sale, dom, supply), `agent` (id, name, brokerage, sales_count), `listing_history_event` (listing_url, observed_at, event: list/price-change/sold/status-change, value).
- Sync cursor: keyed on `region_id` × `status` for searches; `listing_url` for individual listings.
- FTS5: across address, city, neighborhood, agent name, MLS#.

## User Vision
> "Use internal endpoints. Mostly explore homes for sale; open to whatever else is useful."

This drives the headline command toward `homes` (search), `listing` (detail), and `sold` (transaction history). Beyond that, secondary surfaces worth shipping in v1: market trends, agent search, saved-search watch/diff. Out of scope for v1: account-only features (saved homes, agent dashboard, mortgage calculator), tour booking, offer submission.

## Codebase Intelligence
Synthesized from the public Redfin scraper landscape (no MCP/SDK exists):

- **Endpoints (`/stingray/...`):**
  - `GET /stingray/api/gis?al=1&...` — JSON search; primary entry; supports polygon, region_id, status, sf, uipt, num_homes, page_number, min_price, max_price, num_beds, num_baths, sort.
  - `GET /stingray/api/gis-csv?...` — CSV bulk download; max 350 rows.
  - `GET /stingray/do/gis-search?...` — legacy JSON search.
  - `GET /stingray/api/home/details/initialInfo?path=<listing-url-path>` — gives canonical `listingId` and `propertyId`.
  - `GET /stingray/api/home/details/aboveTheFold?listingId=&propertyId=` — photos, headline price/beds/baths/sqft.
  - `GET /stingray/api/home/details/belowTheFold?propertyId=&listingId=&accessLevel=` — amenities, price/tax/school history, AVM.
  - `GET /stingray/api/region/<type>/<id>/<period>/aggregate-trends` — neighborhood/city market stats.
  - `GET /do/region-search-autocomplete?location=<query>` — region autocomplete.
  - `GET /stingray/api/v1/rentals/<id>/floorPlans` — rental floor plans (rentals out of v1 scope).
  - `GET /newest_listings.xml`, `/sitemap_com_latest_updates.xml` — RSS-style feeds.
- **Response quirk:** Stingray JSON responses are wrapped in `{}&&{...}` — a CSRF-prevention prefix. The first 4 bytes (`{}&&`) must be stripped before JSON-decode.
- **Auth:** None for the public stingray endpoints v1 uses. Account-only features (alerts, saved homes) require cookie session — out of scope.
- **Rate limiting:** community scrapers report blocks "at scale" (no hard number). Use adaptive limiter, conservative default 1–2 req/s, exponential backoff on 429.
- **Geo:** US-only. Non-US callers get 403 regardless of headers.

## Source Priority
- Single-source CLI; no priority ordering needed.

## Product Thesis
- Name: `redfin-pp-cli` (binary), `redfin` (slug).
- Tagline: *"Stingray-backed Redfin CLI with the workflows the website can't do — saved-search diff, sold-price trends, $/sqft ranking, and offline SQL."*
- Why it should exist:
  1. Every public Redfin scraper is bot-blocked or paid SaaS. Surf with Chrome TLS fingerprint + adaptive limiter clears the protection that broke reteps/redfin and dreed47/redfin.
  2. The Stingray endpoints expose real for-sale, sold, and market-trend data — but Redfin's UI surfaces almost no analytical views (no saved-search diff, no per-listing history view, no $/sqft sort, no aggregate sold-price trends across multiple cities).
  3. Local SQLite + FTS5 unlocks the workflows: watch a saved search, rank by $/sqft net of HOA, diff sold prices, compare a shortlist, market summary across 5 neighborhoods, agent leaderboards.
  4. Agent-native by default (`--json`, `--select`, `--csv`, MCP-ready).

## Build Priorities
1. **Foundation (P0)** — Surf-backed client + Stingray prefix-stripping decoder, SQLite store keyed on listing URL/ID, autocomplete (region resolver), `/stingray/api/gis` search wired to filter flags.
2. **Absorb (P1)** — full filter set (beds, baths, price, sqft, lot, year-built, status, property type, schools, polygon), pagination, listing detail (initialInfo + aboveTheFold + belowTheFold combined), CSV export, sold-homes search, region autocomplete, market trends.
3. **Transcend (P2)** — saved-search watch with diff, sold-price trends across regions, $/sqft and $/bed ranking, side-by-side compare, listing history (from belowTheFold history), price-drop alerts, stale-listing flag, neighborhood market summary, agent search/leaderboard, multi-region union ranking, weekly digest.
