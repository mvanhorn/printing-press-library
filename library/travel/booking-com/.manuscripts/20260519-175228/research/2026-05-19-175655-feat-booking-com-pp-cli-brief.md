# Booking.com CLI Brief

## API Identity
- Domain: hotel/accommodation booking (also flights, cars, attractions; hotels are the headline)
- Users: leisure travelers (price-shoppers, deal-hunters, multi-city itinerary planners), business travelers managing their own bookings, and travel-research agents
- Data profile: properties, prices, availability windows, reviews, wishlist, trips (past + upcoming), search history, destinations (cities, neighborhoods, regions, landmarks)
- Scope decision: **public booking.com website** (cookie + CSRF). NOT the invite-only Partner/Demand API. User has a logged-in Chrome session and explicitly wants it used.

## Reachability Risk
- **Medium.** Booking.com is fronted by AWS WAF. `probe-reachability` on `https://www.booking.com/` returns:
  - stdlib HTTP -> `HTTP 202` with "AWS WAF marker" evidence (blocked)
  - Surf+Chrome TLS fingerprint -> `HTTP 202 text/html` (cleared)
  - mode: `browser_http`, confidence 0.85, no clearance cookie needed for unauth requests
- Auth-required surfaces (trips, wishlist, recently viewed, saved searches) need the user's Chrome cookies.
- Internal GraphQL endpoint requires `X-Booking-CSRF-Token` (extracted from `b_csrf_token: '...'` in search-page HTML) and matching `bkng`/`px3` cookies.
- Community scraper repos show occasional "empty response" issues but no wave of permanent 403s, consistent with the Surf-clearable WAF tier.

## Top Workflows
1. **Find a hotel for specific dates in a destination** (city/region/landmark, dates, guests, filters: price, rating, breakfast, free-cancellation, distance from center)
2. **Find the cheapest dates** for a given hotel or destination in a date window
3. **Pull full details** for one hotel (amenities, address, coords, photos, room types, cancellation policy)
4. **Track a watched hotel** for price drops over time (local diff against last-seen state)
5. **Browse "my trips"** (upcoming + past bookings) and **wishlist** (saved properties) from the logged-in account
6. **Resolve a destination** (autocomplete city/landmark -> `dest_id`/`dest_type`) so other commands can use stable IDs
7. **Read reviews** for a property with filters (language, score band, traveler type)

## Table Stakes (must match what exists)
- Hotel search by destination + dates + guest count, with offset pagination (25 per page)
- Star rating, review score, distance from center, lowest price per night, currency
- Detail page: amenities by category, address, coordinates, room types, photos
- Reviews list with pagination, score, language, date, traveler type
- Map markers for a search result set (lat/lon + price summary)
- Filters: price range, star rating, score, free cancellation, breakfast included, distance from center
- Sort: price (asc/desc), top-reviewed, distance, popularity
- Destination autocomplete -> `dest_id`/`dest_type`
- JSON output, scriptable filter flags

## Data Layer
- Primary entities: `destinations`, `properties` (hotels), `prices` (date-window samples), `reviews`, `wishlist_items`, `trips`, `search_results` (one row per snapshot)
- Sync cursor: per-destination + per-date-window search snapshots (no incremental cursor at Booking.com; we snapshot)
- FTS/search: properties (name, description, amenities), reviews (text)
- Diff tables: `price_history` (property_id, checkin, checkout, occupancy, currency, price, observed_at) -- enables price-drop detection nobody else has

## Codebase Intelligence
Sourced from `Booking.com-python-api-spider` (avkaz), `BookingScraper` (ZoranPandovski, 96 stars), `booking_scraper` (HexNio, 36 stars), `actor-booking-scraper` (dtrungtin), `EmilyThaHuman/booking-mcp-server`, `esakrissa/hotels_mcp_server` (RapidAPI-routed -- excluded as not real booking.com), Scrapfly 2026 blog, and The Web Scraping Club substack.

- **Endpoint:** `https://www.booking.com/dml/graphql?lang=en-us`
- **Operations seen in the wild:**
  - `FullSearch` -> paginated hotel listings + metadata
  - `MapMarkersDesktop` -> map pin data (lat/lon + summary price)
  - Review-pagination operations (exact names to confirm during sniff)
- **HTML surfaces:** `/searchresults.html?ss=&dest_id=&checkin=&checkout=&group_adults=&group_children=&no_rooms=&offset=` and `/hotel/<country>/<slug>.html`
- **Auth pattern:** `X-Booking-CSRF-Token` header extracted from `b_csrf_token: '...'` in search-page HTML; `bkng`, `_gcl_au`, and login cookies carry session
- **Search variables:** `destType`, `destId`, `dates: {checkin, checkout}`, `nbAdults`, `nbChildren`, `nbRooms`, `pagination: {rowsPerPage, offset}`
- **Rate behavior:** community scrapers report ~10-30 RPS without proxy; CSRF token refresh recommended every ~30 min for long sessions; 25 results per page is the soft cap

## User Vision
User is logged in to booking.com in Chrome and wants the browser session used for both discovery (browser-sniff) and the printed CLI's auth path. This means the runtime must support cookie import from Chrome (`auth login --chrome`) so the CLI's authenticated commands (trips, wishlist) work without manual cookie pasting.

## Product Thesis
- **Name:** Booking.com (display) / `booking-com-pp-cli` (binary)
- **Why it should exist:** Every existing tool is either (a) a scraper without local persistence, agent-native output, or wishlist/trips support, (b) a RapidAPI-routed MCP that pays per call and doesn't see auth-only data, or (c) a third-party Apify actor billed per run. The CLI that wins combines: real GraphQL endpoint hits via Surf+Chrome (no proxy fees), Chrome cookie import (auth-only data unlocked), SQLite price-history (cheapest-dates and price-drop detection nobody else has), JSON-first agent output, and offline FTS over scraped properties/reviews.

## Build Priorities
1. **Foundation:** Surf-backed client with Chrome TLS fingerprint + `auth login --chrome` cookie import; SQLite store for `properties`, `prices`, `price_history`, `wishlist_items`, `trips`, `reviews`; CSRF token extraction helper
2. **Absorb:** Hotel search (with full filter/sort matrix), destination autocomplete, hotel detail, reviews list, map markers, wishlist read, trips read -- match every public scraper feature
3. **Transcend:** Cheapest-dates sweep, price-drop watch with diff against last-seen, "wishlist drop alert", offline property search via FTS, recent-search analytics, "compare" two hotels side-by-side -- the commands that only work because we keep history in SQLite
