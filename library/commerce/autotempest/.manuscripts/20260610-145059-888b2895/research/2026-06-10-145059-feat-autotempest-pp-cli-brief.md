# AutoTempest CLI Brief

## API Identity
- Domain: Used-car meta-search aggregator ("Kayak for cars"). One query fans out across the major used-car marketplaces.
- Users: Car shoppers comparing prices across many sites at once; deal hunters; resellers; agents automating car search.
- Data profile: Vehicle listings. Aggregated 5M+ from partnered sources (inline results) plus comparison links for non-partnered sources.
- Sources aggregated: eBay Motors, Craigslist, Cars.com, Autotrader, CarGurus, TrueCar, Carvana, CarsDirect, Kijiji (CA), CarMax, Facebook Marketplace (link-only). Partnered = inline; others = comparison links.
- No official public API. Mobile apps (iOS/Android) and the website are the only first-party surfaces.

## Reachability Risk
- [Low-Medium] Homepage returns HTTP 200 with full HTML (Next.js App Router), no bot challenge, no Cloudflare/DataDome interstitial. Static1 CDN for assets, blog subdomain.
- Search/results endpoint NOT yet identified — App Router fetches client-side. Must browser-sniff `/results?...` to find the internal data endpoint (likely `_next/data/<buildId>/...json` or an internal search/poll API).
- Async aggregator pattern likely: results stream in from multiple sources (long-poll/SSE/incremental). Capture must wait for results to populate, not just the initial request.
- Probe-safe endpoint: GET homepage (200, verified). No mutation endpoints will be probed.

## Top Workflows
1. Search listings by make/model + filters (year range, price range, mileage, location/zip + radius, body type, transmission, drivetrain, fuel, color, keyword).
2. Compare the same make/model across all sources in one result set (the core value prop).
3. Drill into a single listing's detail (price, mileage, location, source, link, seller).
4. Track a saved search over time — what's new, what dropped in price (not offered by the site for anonymous users → transcendence).
5. List which sources/marketplaces a given search hit.

## Table Stakes (must match incumbents)
- Full-filter search (make, model, year min/max, price min/max, mileage max, zip + radius, body style, sort).
- Per-listing fields: title, year, make, model, price, mileage, location, source site, listing URL, thumbnail.
- Pagination / result limit.
- Source attribution per listing (which marketplace).

## Data Layer
- Primary entity: `listing` (vehicle). Keyed by source + source-listing-id (or listing URL); VIN when available for cross-source dedupe.
- Secondary: `source` (marketplace), `search` (saved query + cursor).
- Sync cursor: per saved-search snapshot timestamp.
- FTS/search: offline full-text over title/make/model/location.
- Snapshots: price history per listing across syncs → price-drop detection.

## Codebase Intelligence
- Existing tools (to absorb): Coder-Boiiiiiii/AutoTempest-Scraper (Python, Selenium HTML scrape, fields: price/mileage/location/model/year → Excel); Apify ecomscrape/autotempest-cars-search-scraper (hosted actor + Python client). Neither exposes a documented JSON endpoint — confirms browser-sniff is the path to a clean replayable surface.
- No existing CLI, MCP server, or Claude skill for AutoTempest. Green field.

## User Vision
- "Just go." No specific cars named. Build the general-purpose car-search CLI; default filters sensible; St. Petersburg FL is a reasonable default location hint if the endpoint needs a zip (operator is Tampa Bay).

## Product Thesis
- Name: autotempest-pp-cli ("AutoTempest, from the terminal")
- Why it should exist: AutoTempest already unifies every car marketplace into one search — but only in a browser. No CLI, no agent-native output, no local store, no price-drop alerts for anonymous shoppers. A CLI that hits the same aggregated search, persists listings to SQLite, dedupes across sources, and tracks price drops over time gives shoppers and agents something the website and every existing scraper lack: queryable, compounding, offline car-search state.

## Build Priorities
1. Browser-sniff `/results` to capture the real search endpoint + response shape (Phase 1.7). This unblocks everything.
2. Data layer: `listing`, `source`, `search` tables + sync + FTS.
3. `search` command (full filters, --json, --select, source attribution).
4. `listing get` detail.
5. Transcendence: price-drop `watch`/`since`, cross-source dedupe, deal-score (price vs comparable matches), market-position aggregation.
