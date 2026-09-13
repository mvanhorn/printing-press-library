# Immoweb CLI Brief

## API Identity
- Domain: Belgian real-estate classifieds (immoweb.be, Axel Springer / Aviv group). Largest BE listing portal: ~50k houses for sale, apartments, land, garages, commercial, industrial, rentals; FR/NL/EN.
- Users: home buyers & renters hunting in specific communes; small investors (rental yield, life-annuity "viager", public sales); agents/analysts watching market prices; relocators (expats) comparing communes.
- Data profile: read-only public classifieds. No official public API (partner feed API only, requires agency contract — user has no access). The website exposes clean, unauthenticated JSON endpoints used by its own Vue front-end.

## Discovered surface (direct HTTP, no cookies, no browser)
All verified 2026-09-12 with plain curl (HTTP 200, `application/json`), Cloudflare in front, DataDome JS tag present on pages but not enforced on JSON routes (10-request burst all 200, Go default UA also 200).
- `GET /{lang}/search-results?countries=BE&propertyTypes=HOUSE&transactionTypes=FOR_SALE&page=N&orderBy=...` → `{criteria, sort, totalItems, range, marketingCount, results[30]}`. Pure query-param form works (no path slugs needed). `totalItems` caps at 9969 (333 pages); exact totals come from the count endpoint.
- `GET /{lang}/search-results-count?<same criteria>` → `{"classifiedsCount": N}` (exact, e.g. 50,430 houses for sale).
- `GET /{lang}/search-results-map?<criteria>` → up to 200 results per call with lat/lng — bulk sync path.
- `GET /{lang}/classified/get-result/{id}` → `{classified:{...}}` full detail JSON: property (bedrooms, bathrooms, surfaces, garden, terrace, parking, building condition, construction year, facades), energy (heating type, heat pump, PV), certificates (EPC/PEB score, kWh/m², reference, renovation obligation), sale (price, cadastral income, VAT), rental (monthly rent + costs), publication (creationDate, lastModificationDate), statistics (viewCount, bookmarkCount), customers (agency name/phone/email/website), media (pictures, floor plans, virtual tour), flags (isNewPrice, isNewClassified, isUnderOption, isPublicSale, isLifeAnnuitySale, isSoldOrRented).
- `GET /{lang}/annonce/{id}` (HTML) redirects to canonical URL; page embeds `window.classified = {...}` (same object) — fallback.
- `GET /{lang}/search/autocomplete?query=ixel` → locality/postal-code resolution: `[{group, results:[{type, label, queryParam:"postalCodes", queryValue:"BE-1050", slug}]}]`.
- `GET /{lang}/search-rendered-similar-results?classifiedId=ID` → similar listings (JSON embedded in HTML attribute; HTML-entity encoded).
- Filters verified via count endpoint: propertyTypes (HOUSE, APARTMENT, LAND, OFFICE, GARAGE, COMMERCIAL, INDUSTRY, OTHER, comma lists), transactionTypes (FOR_SALE, FOR_RENT), postalCodes (1050 or BE-1050, comma list), provinces (NAMUR...), districts (LIEGE...), min/maxPrice, min/maxBedroomCount, min/maxSurface, min/maxLandSurface, min/maxConstructionYear, min/maxFacadeCount, min/maxGardenSurface, min/maxTerraceSurface, min/maxParkingPlaces, epcScores (A,B,...), hasGarden, hasSwimmingPool, hasTerrace, hasLift, isNewlyBuilt, isALifeAnnuitySale, isAPublicSale, isImmediatelyAvailable, isFurnished; orderBy relevance|newest|cheapest|most_expensive|postal_code. Invalid enum values (e.g. `regions=BRUSSELS`) return an HTML error page, not JSON — CLI must detect and report.
- `price.immoweb.be` (old price map) now redirects to the homepage — defunct; market stats must be computed locally.

## Reachability Risk
- Low. Direct JSON endpoints answer plain HTTP 200 without cookies; burst of 10 sequential calls, all 200 at ~0.5 s. DataDome tag exists on HTML pages (`window.ddjskey`), so heavy concurrent crawling could trigger challenges → CLI uses adaptive rate limiting and surfaces a typed rate-limit error instead of empty results. Competitor scraper issues: see ecosystem report (pending).

## Top Workflows
1. Search listings with precise filters for a commune/postal code set ("apartments to rent in Ixelles < 1500 €, 2 bedrooms, newest first") and scan results compactly.
2. Open one listing's full details (EPC, cadastral income, surfaces, agency contact, photos) by ID or URL.
3. Watch a saved search: what's new since last check, which listings dropped their price, which disappeared (sold/rented).
4. Judge a price: €/m² vs the commune's current median, days on market, view/bookmark counts.
5. Market snapshot of a commune: listing count, median price & €/m² by type and bedroom count; compare communes.

## Table Stakes
- Search with all site filters + pagination, sort, JSON/CSV export (every scraper does CSV).
- Detail fetch with the full attribute set (bootcamp scrapers flatten ~20 fields: price, locality, type, subtype, rooms, surface, kitchen, furnished, fireplace, terrace, garden, land, facades, pool, state of building).
- Locality → postal code lookup.
- New-listing alerts (Telegram/email notifier bots are the most common community project).

## Data Layer
- Primary entities: `listings` (id, type, subtype, transaction, price, rent, bedrooms, surface, land, postal code, locality, lat/lng, agency, EPC, construction year, flags, first_seen, last_seen), `price_history` (id, observed_at, price), `saved_searches` (name → criteria), `search_runs` (search, run_at, ids seen).
- Sync cursor: a named saved search; each run upserts listings and appends price observations; `last_seen` marks disappeared listings.
- FTS/search: FTS5 over title + description + locality + agency.

## User Vision
- User asked for "the most complete thing"; no partner access → website-backed CLI.

## Product Thesis
- Name: immoweb-pp-cli ("Immoweb CLI")
- Why it should exist: Immoweb has no public API, no price history, and its alerts are email-only and stateless. Scrapers in the wild are one-shot CSV dumps that break when HTML changes. This CLI talks to the same JSON endpoints the site uses (fast, no browser), exposes every filter as flags, and keeps a local SQLite history so it can answer questions the site cannot: what's new since yesterday, which listings cut their price and by how much, how long has this been on the market, is this €/m² cheap for this commune.

## Build Priorities
1. P0: spec for search / count / map / detail / autocomplete; local store for listings + price history.
2. P1: `search` with all filters (+ commune names resolved via autocomplete), `listing get`, `locate`, `count`, CSV/JSON export, sync of saved searches.
3. P2: `watch` (new / price-drop / gone since last run), `price-drops`, `market` stats per commune, `compare` communes, `deal` score (€/m² vs local median), `stale` (days on market), `yield` estimate for investors.

## Ecosystem (research agent, 2026-09-12)
- No dedicated Immoweb CLI, MCP server, or PyPI package exists. npm: only an n8n node wrapping an Apify actor.
- Best-in-class hobby tool: lawrensylvan/immoweb-keeper (29★, JS): saved searches, incremental sync (sort newest, stop at first unchanged), full listing JSON store, photo download, lastSeen/disappearanceDate, liked/visited, map/table UI, Biddit auctions.
- Alert bots: othellodesutter/immoweb-telegram-bot (poll search JSON, Telegram + photos), vincentcox/immoweb-zimmo-scraper (SQLite seen IDs, Pushover, silent first run), adamkorompai/immoweb_bot (38 Brussels searches, private-owner filter, phone extraction, seen-IDs TTL 30d), simon324/belgium-rental-watcher (LLM relevance score vs free-text brief, daily digest).
- Data scrapers: pierodellagiustina (loop postcodes, bedroom/surface ranges, CSV), g-coomans/ImmoWeb (multi-site, SQLite, "is it gone" check), ptrues antwerp-rentals (search JSON → GeoJSON, distance-to-POI filter, stale-data status), OxyHQ/Homiio provider (JSON-first, block-page detection).
- Apify actors (~10): mass scraper by search URL with new/delisted monitoring, view/bookmark counts, contacts; EPC, images; monitoringMode; days on market; postcode→province mapping; RSS export.
- Browser extensions: ImmowebFilter (hide/highlight sold / under-offer), immoweb-chromium-extension (hide dismissed houses, synced hidden list).
- Pain points: no history once sold/rented vanish; sold/under-offer clutter; Immoweb alerts fire on everything (want relevance ranking); private-owner-only + phone; hide seen/dismissed; dedupe across overlapping searches; 30/page caps; brittle selectors; want Zimmo/Immovlan too.

## Reachability Risk (updated)
- Low for JSON endpoints, High for HTML pages. 2026 reports: DataDome blocks datacenter IPs for HTML (rental-watcher May 2026, Apify actors require BE residential proxies), Cloudflare JS challenge on pages (localizer June 2026). Counter-evidence: Homiio (Jul 2026) and ptrues (Sep 2026) use plain requests on the JSON endpoints; our own probes 2026-09-12 all 200.
- Soft throttling signature: HTTP 200 with empty `results` before `totalItems` is reached (ptrues). CLI must treat this as a typed rate-limit error, not end-of-results.
- Send `Accept: application/json` + `X-Requested-With: XMLHttpRequest` + browser UA; detect HTML/challenge bodies on JSON routes.

## Auth context
- User is logged in to Immoweb in Chrome; CLI is for a client who must have a seamless experience. Decision: public-first CLI; optional `auth login --chrome` only if authenticated endpoints (favorites/saved searches) are discovered AND cookie replay validates outside the browser. Otherwise ship public only (user's explicit fallback).
