# Agoda CLI Brief

## API Identity
- Domain: Online travel agency (OTA), hotel/accommodation booking. Scope for this run is
  **hotel booking only** (user-directed): search, destination resolution, property detail,
  room/rate availability, pricing, reviews, and the authenticated booking-adjacent surfaces
  (trips, wishlist/favorites, AgodaVIP tier, AgodaCash). Flights, activities, airport
  transfers, and car rental are explicitly OUT OF SCOPE.
- Users: Leisure and business travelers, heavily APAC-weighted. Agoda's competitive edge is
  APAC inventory depth (Thailand, Japan, Indonesia, Vietnam, Korea) where it routinely beats
  Booking.com on both coverage and price.
- Data profile: Property catalog (largely static), rates/availability (highly volatile,
  date- and occupancy-scoped), reviews (append-only, high volume), destinations (static),
  authenticated user state (trips, wishlist, VIP tier, AgodaCash balance).

## Reachability Risk
- **Low-to-medium.**
- `probe-reachability` => `mode: standard_http`, confidence 0.95, `needs_browser_capture: false`,
  `needs_clearance_cookie: false`. Both stdlib HTTP and Surf/Chrome-TLS returned 200 on the
  homepage and on a real dated search URL.
- CDN/WAF is Akamai (`akamai-grn` response header). No active challenge, no CAPTCHA, no
  clearance cookie observed on any probe. The literal string "CAPTCHA" in the search HTML is
  a CSS class name for the login form, not an active challenge.
- The real risk is **rate limiting**, not blocking. Community scraping guidance (scraperly,
  Apr 2026) rates Agoda "medium, 3/5", citing custom WAF + rate limiting and recommending
  residential proxies at scale. Mitigation: the printed CLI must ship an adaptive limiter and
  surface typed rate-limit errors rather than empty results.
- Tier/permission hints from 4xx body: none. No 4xx was produced by any probe.
- Probe-safe endpoint used: `GET /search?city=9395&checkIn=...&los=2&adults=2&rooms=1` (200),
  plus verified `POST /api/cronos/property/review/ReviewComments` (200 with real payload).

## Architecture Finding (decisive for this build)
Agoda is a **fully client-rendered app backed by GraphQL**, unlike Booking.com which is
SSR-dominant. This inverts the discovery strategy relative to the closest peer CLI:
- `/search?...` returns 389KB of JS/CSS bootstrap with **zero** hotel data. `hotelId` token
  count 0, JSON-LD blocks 0.
- `/<slug>/hotel/<city>-<cc>.html` returns 88KB, also data-free. No JSON-LD, and the
  `<script data-selenium="script-initparam">` element that older tools scraped is **gone**.
- `/graphql` and `/graphql/search` are live and return well-formed GraphQL errors, but
  **introspection is disabled**: `"Query reducing error: Introspection is not allowed."`
- A REST sidecar namespace `/api/cronos/**` exists and works unauthenticated.

Consequence: HTML parsing is a dead end for search and detail. Browser capture is required to
learn GraphQL operation names, query documents, and variable shapes. Once captured, those
operations **replay fine over plain HTTP** — no browser sidecar, no clearance cookie. This is
the ideal printed-CLI shape and is why the runtime stays `standard_http`.

## Verified Endpoints (pre-sniff, unauthenticated)
- `POST /api/cronos/property/review/ReviewComments` — body `{hotelId,page,pageSize,sorting}`.
  Returns `comments[]` (rating, reviewComments, helpfulVotes, checkInDateMonth, providerId,
  hotelReviewId), plus `reviewPageUrl` (reverse-maps hotelId -> slug) and `reviewsSortOptions`.
  Verified live against hotelId 7708. Real sort enum: 1=most recent, 2=rating high->low,
  3=rating low->high, 7=most helpful.
- `POST /graphql`, `POST /graphql/search` — live, introspection blocked, operations TBD by sniff.
- `GET /api/cronos/search/GetUnifiedSuggestResult` — 200, likely destination autocomplete.

## Top Workflows
1. **Search hotels for a destination + dates + occupancy**, filtered (price ceiling, star
   rating, review score, free cancellation, breakfast) and sorted. The primary workflow.
2. **Judge one property**: full detail, room/rate options, and the review distribution behind
   the headline score.
3. **Find the cheapest dates** for a property or destination across a flexible window.
4. **Compare a shortlist** of 2-3 finalists on price, score, amenities, and cancellation terms.
5. **Work the authenticated account**: upcoming trips + cancellation deadlines, wishlist,
   AgodaVIP tier and what it actually saves.

## Table Stakes (must match, from booking-com peer CLI + Apify + scrapers + review MCP)
- Destination resolution (free text -> stable city/dest id)
- Hotel search with dates/occupancy/rooms + filters + sort + pagination
- Property detail (address, star rating, review score, amenities, images, coordinates)
- Room/rate list with cancellation policy and breakfast flags
- Paginated reviews with sort options and per-review metadata
- Currency and language control
- Deep booking URL back to Agoda
- Authenticated: trips list, wishlist, loyalty status
- Agent-native: `--json`, `--agent`, `--select`, `--dry-run`, typed exit codes
- Local persistence + offline search

## Known User Pain Points (research-sourced, these drive the novel features)
1. **Displayed price != price paid.** Taxes and a separate "tax recovery charge" appear only
   at checkout. Widely reported (e.g. a ~$200 two-night stay billing near $260).
2. **Resort/facility fees payable at property**, not prepaid, and easy to miss when scanning
   (e.g. $120/night room + $45/night resort fee in Las Vegas / Honolulu). Travelers believe
   they are fully prepaid and get surprised at the front desk.
   Agoda's own URL vocabulary exposes `finalPriceView=1`, which is the lever for this.
3. **Cancellation and refund friction**: non-refundable traps, missed free-cancellation
   deadlines, slow refunds, hard-to-reach support.
4. **VIP/logged-in price divergence**: AgodaVIP (Bronze/Silver/Gold/Platinum/Diamond) grants up
   to ~25% off VIP-listed hotels, so the anonymous price is often not the user's real price.
5. **AgodaCash vs PointsMAX is an unguided either/or.** PointsMAX earns into 46 external
   loyalty programs instead of AgodaCash; nobody helps the user value the two against each other.

## Data Layer
- Primary entities: `properties`, `destinations`, `rates` (price observations, time-series),
  `reviews`, `trips` (auth), `wishlist` (auth), `search_runs`.
- Sync cursor: per-(property, checkin, checkout, occupancy, currency) rate observations stamped
  at fetch time; reviews by `hotelReviewId` high-water mark.
- FTS/search: FTS5 over property name + description + amenities + destination + neighborhood.
- The rate time-series is the substrate for every price-history, cheapest-date, and drop-alert
  feature. It is what a stateless scraper structurally cannot do.

## Codebase Intelligence
- `birariro/agoda-review-mcp` (Java/Spring AI): sole Agoda MCP. Gave the exact ReviewComments
  contract. **Its hotelId extraction is broken today** (relies on the now-absent
  `script-initparam` element), and it mislabels the sort enum. We match its capability and
  beat it on correctness, sort fidelity, and pagination.
- `ScraperHub/agoda-property-listing-scraper`: needs paid Crawlbase to render JS. We need
  neither, because we replay GraphQL directly.
- `seanbabalala/hotelrate-crawl`: multi-OTA MCP, requires Playwright — a resident browser.
  We ship plain HTTP.
- Apify `agoda-scraper` (parseforge/bovi): paid actor. Its field list is the de-facto
  search-card schema to match: name, image, address, neighborhood, rating, review count, star
  rating, current price + currency, original price, breakfast-included, free-cancellation,
  room type, distance-to-landmark, back-reference URL.
- `agoda-com/api-agent`: Agoda's own open-source universal GraphQL/REST->MCP proxy. Notable as
  a signal that Agoda is GraphQL-first internally. Not usable against agoda.com public traffic.

## User Vision
- User directive, verbatim: "the main feature i'm looking at is for hotel booking only. just
  focus on hotel booking." Hotels only. No flights, activities, transfers, or cars.
- User is already logged into Agoda in Chrome, so `AUTH_SESSION_AVAILABLE=true`: authenticated
  browser-sniff is available and the printed CLI should ship `auth login --chrome` cookie import.

## Product Thesis
- Name: `agoda-pp-cli`
- Why it should exist: Every existing Agoda tool either needs a paid rendering service
  (Crawlbase, Apify), a resident browser (Playwright), or is already broken (the review MCP).
  None of them keep state. Agoda's GraphQL replays cleanly over plain HTTP once observed, so a
  printed CLI can be both cheaper and more capable than all of them — and then do the thing no
  scraper can: remember prices over time and tell the user the *real* all-in cost, not the
  teaser rate. The headline is honest pricing plus compounding local rate history.

## Build Priorities
1. Browser-sniff the GraphQL surface (search, property detail, rooms/rates, autocomplete) and
   the authenticated surfaces (trips, wishlist, VIP) using the logged-in Chrome session.
2. Data layer: properties, destinations, rates time-series, reviews + FTS5.
3. Absorbed surface: destinations, hotels search/get, rooms/rates, reviews, trips, wishlist,
   account/VIP.
4. Transcendence: all-in true price, VIP delta, cheapest-date sweep, price-drop watch,
   cancellation-deadline alarm, AgodaCash-vs-PointsMAX valuation, shortlist compare.
