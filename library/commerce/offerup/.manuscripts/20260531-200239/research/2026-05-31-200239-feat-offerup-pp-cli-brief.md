# OfferUp CLI Brief

## API Identity
- **Domain:** Local peer-to-peer marketplace (buy/sell used goods locally, mobile-first; LetGo merged in). ~90M users. Top categories: electronics, furniture, vehicles, clothing, sporting goods.
- **Users:** Local buyers hunting deals; sellers listing items; resellers/flippers doing price research and sourcing; deal-snipers watching for underpriced items.
- **Data profile:** Listings (title, price, condition, location/lat-lon, photos, seller, posted time), sellers/profiles (rating, response time, reply rate, inventory), categories, saved searches. No official public API — internal **GraphQL BFF** (Backend-For-Frontend: auth, aggregation, materialization, caching). Reachable surfaces are reverse-engineered.

## Reachability Risk
- **Medium.** OfferUp blocks fast/automated requests (unofficial API authors advise >1s between calls) and likely runs bot protection on the web app. However: multiple maintained scrapers extract "listings without login," and HAR-based capture is confirmed working in 2024. So a replayable surface very likely exists; runtime tier (standard vs Surf/browser-compatible vs clearance-cookie) to be settled by `probe-reachability` + browser-sniff.
- **Legal/ToS note:** A prior unofficial library (`planetzero/offerup`) was hit with a DMCA takedown. This is a personal-use CLI; respectful rate limiting and not redistributing scraped data are the mitigations. Not a technical blocker.
- Evidence: everettperiman/OfferupUnofficalAPI (rate-limit note), Apify "Extract Listings Without Login" scrapers, "No-Code OfferUp HAR scraping still works in 2024" (YouTube).

## Top Workflows
**Unauthenticated (headline — per user directive, these lead):**
1. **Search listings** by keyword + location (ZIP or lat/lon) + filters: radius, price min/max, condition, delivery (pickup/ship/both), sort. The #1 workflow.
2. **Item detail** — full description, all photos, condition, price-firm flag, seller, location for a listing URL/ID.
3. **Browse a category** nearby (electronics, furniture, vehicles, etc.).
4. **Seller / profile lookup** — a seller's reputation (rating, response time, reply rate) + their active inventory.
5. **Price research** — what's the going rate for an item in my area (aggregate across search results). Resellers' core need.

**Authenticated (only when required — secondary, clearly gated):**
- Saved/favorited items, messages/conversations, your own listings, posting an item, making/receiving offers. Need a browser session cookie; built only if the user opts in at the absorb gate.

## Table Stakes (what existing tools do — must match)
- Search by keyword + ZIP/coords (pyOfferUp, all Apify scrapers).
- Filters: radius, price range, condition, delivery, sort (everettperiman params).
- Return listing fields: id, title, price, location, image, url, condition, firm-price, vehicle miles, flags (pyOfferUp).
- Item full-detail fetch (planetzero getItem, scrapers' "full details" mode).
- Seller/user profile fetch (planetzero getUserProfile).
- Multi-source garage-sale aggregation exists (gs-scraper: Craigslist/LetGo/OfferUp/etc.) — out of scope for a single-source OfferUp CLI but informs the "price compare" idea.

## Data Layer
- **Primary entities:** `listings` (id, title, price, condition, lat/lon, location_name, category, seller_id, posted_at, is_firm, delivery, url, image_url, vehicle_miles, flags), `sellers` (id, name, rating, response_time, reply_rate, listing_count), `searches` (saved query + filters + location), `price_snapshots` (query/category + area → price stats over time for trends).
- **Sync cursor:** per saved-search query; paginate by `limit`/offset or GraphQL cursor. Re-sync appends new listings + records a price snapshot.
- **FTS/search:** offline full-text over `listings.title` + description; SQL-composable filters (price, condition, distance, seller).

## Codebase Intelligence
- Source: reverse-engineered repos (everettperiman/OfferupUnofficalAPI, planetzero/offerup, everettcaldwell/unofficial-offerup-api [GraphQL], oscar0812/pyOfferUp).
- **Auth:** anonymous works for search/browse/item/seller; account actions use a session cookie (no API key, no bearer token). Maps to `auth.type: cookie` with most endpoints `no_auth: true`.
- **Data model:** listing + seller as above; images via `thumbor.offerup.com`.
- **Rate limiting:** self-imposed >1s between calls; CLI must use `cliutil.AdaptiveLimiter` and a conservative default. Likely 429/403 bot-protection on bursts.
- **Architecture:** GraphQL BFF — browser-sniff should capture persisted GraphQL operations/hashes + the search/item/seller request shapes.

## User Vision
- **Prefer unauthenticated scenarios; only require authentication when the action genuinely requires it.** (User directive, verbatim intent.) Public browse/search/item/seller commands are the headline and need no login. Auth-gated commands (saved, messages, posting, my-listings) are secondary, clearly labeled, and opt-in at the absorb gate — not part of the default first-run experience.

## Product Thesis
- **Name:** `offerup-pp-cli` (display "OfferUp")
- **Why it should exist:** Every OfferUp scraper dumps a one-shot listing array. None of them persist, none track prices over time, none flag underpriced deals against the local median, and none are agent-native. This CLI puts the local marketplace in your terminal and your agent's toolbelt: search every listing with no login, keep them in a local SQLite store, watch saved searches for new drops, track price trends, and surface deals priced below the going rate before anyone else sees them.

## Build Priorities
1. **P0 data layer** — listings + sellers + searches + price_snapshots store; sync from search; FTS + SQL.
2. **P1 absorbed (unauth headline)** — `search` (full filter set), `item`/`listing get`, `category browse`, `seller`/`profile`, `sync`.
3. **P2 transcendence (local-store-only)** — `price-check` (local median/percentiles for a query+area), `watch`+`new-since` (time-windowed new-listing diff per saved search), `deals`/`underpriced` (below-median flagging), `price-drops` (cross-sync price-change detection), `seller-scan` (offline seller inventory + reputation).
4. **P3 polish** — agent-native flags everywhere (`--json`/`--select`/`--csv`/`--compact`/`--dry-run`), enriched flag help, tests for store/aggregation logic.
