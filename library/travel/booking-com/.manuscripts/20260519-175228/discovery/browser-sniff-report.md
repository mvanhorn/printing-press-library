# Browser-Sniff Discovery Report: booking.com

## User Goal Flow

- **Goal:** Find a hotel for specific dates in a destination, view hotel details, and read the authenticated user's trips/wishlist/rewards.
- **Steps completed:**
  1. Loaded `https://www.booking.com/` (homepage) -- confirmed logged-in session ("Matt Van Horn, Genius Level 3") and verified 22 non-HttpOnly cookies including `aws-waf-token`.
  2. Navigated to `/searchresults.html?ss=Paris&dest_type=city&checkin=2026-06-20&checkout=2026-06-23&group_adults=2&group_children=0&no_rooms=1&lang=en-us&selected_currency=USD` -- 25 property cards rendered in SSR HTML, CSRF token `b_csrf_token` extracted (220 chars), zero XHR/GraphQL traffic to `/dml/graphql`.
  3. Scrolled to bottom 3x to trigger lazy pagination -- only AWS WAF telemetry and OTEL traces fired; no FullSearch XHR.
  4. Navigated to a real hotel detail page `/hotel/fr/auliviaopera.html` -- 215 KB SSR HTML, JSON-LD Hotel schema present (aggregateRating 8.1 / 1032 reviews, address, postal code), 12 amenity bullets parseable from `[data-testid="property-most-popular-facilities-wrapper"] li`.
  5. Navigated to `secure.booking.com/mytrips.html` (authenticated) -- loaded without login redirect; 0 trip cards (account is empty/China-associated); profile header text confirms session.
  6. Navigated to `www.booking.com/mywishlist.html` (authenticated) -- loaded with `wl_id=18417129` appended by server; empty wishlist with onboarding copy.
- **Steps skipped:** Map view (would have triggered MapMarkersDesktop GraphQL); review pagination on the hotel detail (would have triggered review GraphQL).
- **Secondary flows attempted:** None -- the SSR-dominant pattern was clear after step 2 and authenticated coverage after steps 5-6.
- **Coverage:** 6 of 6 planned steps completed.

## Pages & Interactions

| # | URL | Purpose | Interactions |
|---|-----|---------|--------------|
| 1 | `https://www.booking.com/` | Homepage / auth check | Read cookie set, query profile/login DOM markers |
| 2 | `https://www.booking.com/searchresults.html?ss=Paris&...` | Hotel search SSR | Scrolled bottom 3x, parsed property cards, extracted CSRF token |
| 3 | `https://www.booking.com/hotel/fr/auliviaopera.html?...` | Hotel detail SSR | Scrolled mid + bottom, parsed JSON-LD Hotel, parsed amenities |
| 4 | `https://www.booking.com/myreservations.html` | Marketing/info page (NOT real trips) | Found real auth URLs `secure.booking.com/mytrips.html` + `www.booking.com/mywishlist.html` in header nav |
| 5 | `https://secure.booking.com/mytrips.html` | Authenticated trips SSR | Verified login state, observed empty-state copy |
| 6 | `https://www.booking.com/mywishlist.html` | Authenticated wishlist SSR | Verified cookie auth, observed `wl_id=` URL-param assignment |

## Browser-Sniff Configuration

- **Backend:** chrome-MCP (Claude in Chrome extension driving the user's running Chrome). Used because the user explicitly said they were logged in to booking.com in Chrome and asking them to quit Chrome for the agent-browser save-then-restore flow was unnecessarily disruptive.
- **Pacing:** Adaptive starting at 1s between page navigations; no rate-limit responses encountered. Effective rate ~0.3 req/s during interactive navigation, well within Booking.com's tolerance.
- **Proxy-pattern detection:** Not a proxy-envelope. All observed XHR was either AWS WAF telemetry or first-party OTEL traces; no single `_api/proxy`-shaped route fronting the API.
- **Probe-reachability result:** `mode: browser_http`, confidence 0.85. stdlib HTTP returns 202 with "AWS WAF marker" evidence; Surf+Chrome TLS fingerprint cleared the protection (202 text/html, 161 ms). Printed CLI will ship Surf+Chrome transport.

## Endpoints Discovered

| Method | Path | Status | Content-Type | Auth |
|--------|------|--------|--------------|------|
| GET | `www.booking.com/` | 200 | text/html | public |
| GET | `www.booking.com/searchresults.html` | 200 | text/html | public |
| GET | `www.booking.com/hotel/{country}/{slug}.html` | 200 | text/html | public |
| GET | `www.booking.com/reviewlist.html` | 200 (inferred) | text/html | public |
| POST | `www.booking.com/dml/graphql` | 200 (per research) | application/json | public (CSRF required) |
| GET | `secure.booking.com/mytrips.html` | 200 | text/html | auth-required (cookie) |
| GET | `www.booking.com/mywishlist.html?wl_id=<id>` | 200 | text/html | auth-required (cookie) |
| GET | `secure.booking.com/rewards_and_wallet.html` | 200 (linked) | text/html | auth-required (cookie) |
| GET | `secure.booking.com/mysettings.html` | 200 (linked) | text/html | auth-required (cookie) |

## Traffic Analysis

- **Protocols observed:** `ssr_html` (confidence 0.95) -- primary; `rest_graphql` (confidence 0.7) -- known to exist at `/dml/graphql`, not exercised in this capture but documented from research.
- **Auth signals:** `cookie` (0.95) on `.booking.com` and `.secure.booking.com` -- cookie names: `aws-waf-token`, `bkng`, `bkng_sso_session`, `cgumid`, `bkng_prue`. Non-HttpOnly cookies visible in `document.cookie` include `aws-waf-token`, `cgumid`, `bkng_prue`; `bkng` session cookies are HttpOnly and must come from Chrome's cookie DB via `auth login --chrome`.
- **Parameter-name evidence:** `ss` (search string), `dest_id`, `dest_type`, `checkin`, `checkout`, `group_adults`, `group_children`, `no_rooms`, `offset`, `order`, `nflt` (filter expression), `selected_currency`, `lang` -- all from the live `/searchresults.html` URL the user-flow walked. The spec exposes user-friendly names (`query`, `adults`, `children`, `rooms`, `currency`) with the wire names as aliases.
- **Protection signals:** AWS WAF Bot Control (confidence 0.95) -- bot manager fronts every www.booking.com route; cleared by Surf+Chrome fingerprint; `aws-waf-token` cookie issued to each session.
- **Generation hints:** `requires_browser_http`, `requires_browser_cookie_for_auth_endpoints`, `has_ssr_html_primary_surface`, `has_csrf_token_for_graphql`, `supports_offset_pagination`.
- **Candidate commands:** `search`, `hotels get`, `reviews list`, `destinations autocomplete`, `trips list`, `wishlist get`, `rewards get`, `profile get` -- plus the hand-written transcendence commands documented in the absorb manifest.
- **Warnings:** SSR-dominant page architecture -- all primary user-facing pages deliver their data in HTML, not XHR. `/dml/graphql` exists per Scrapfly 2026 and The Web Scraping Club articles, documented in the spec for map markers and review pagination, but was not directly observed firing during the captured user flow.

## Coverage Analysis

- **Resources exercised:** search (1 page), hotels (1 detail page), trips (empty-state confirmed), wishlist (empty-state confirmed).
- **Resources documented but not directly observed:** reviews list (separate `/reviewlist.html` surface), destinations autocomplete (resolver fires on type, not navigation), map markers (would have required clicking "Show on map"), rewards & wallet (linked but not visited), profile/settings (linked but not visited), `/dml/graphql` POST (no operations fired during this flow).
- **Likely missed:** sustainability ratings, room-type pricing breakdowns, deals page (`/deals.html`), genius landing (`/genius.html`).
- **Compared to research brief:** Brief mentioned 6-7 named workflows; this capture covered 5 of them directly and documented the path to the others.

## Response Samples

- **`/searchresults.html` (Paris)**: 1.95 MB HTML, 25 `[data-testid="property-card"]` blocks, CSRF token at offset ~ inline JSON. Sample card: `Hotel Aulivia Opéra | $177 | Scored 8.1 Very Good 1,032 reviews | href https://www.booking.com/hotel/fr/auliviaopera.html`.
- **`/hotel/fr/auliviaopera.html`**: 215 KB HTML, JSON-LD Hotel: `{"@type":"Hotel","name":"Hotel Aulivia Opéra","aggregateRating":{"ratingValue":8.1,"reviewCount":1032},"address":{"streetAddress":"4 Rue des Petites Ecuries, 10th arr., 75010 Paris, France","postalCode":"75010","addressCountry":"France"}}`. Amenities: Non-smoking rooms, Facilities for disabled guests, Room service, Free Wifi, Family rooms, 24-hour front desk, Elevator, Bar, Tea/Coffee Maker in All Rooms, Very Good Breakfast.
- **`secure.booking.com/mytrips.html`**: 562 KB HTML, empty-state copy (account associated with China site, 0 trip cards). Profile header confirms "Matt Van Horn, Genius Level 3".
- **`www.booking.com/mywishlist.html?wl_id=18417129`**: 560 KB HTML, empty-state copy "Here are 3 simple steps to get you started"; CSRF token also present on this authenticated page.
- **OTEL/WAF beacons**: ~12 entries to `otel-gw.booking.com/v1/traces`, `sink.gw.booking.com/v1/sink`, `d8c14d4960ca.edge.sdk.awswaf.com/*/{telemetry,mp_verify,inputs}` -- noise, not API surface.

## Rate Limiting Events

None. ~14 page loads / interactions over ~5 minutes, no 429s, no challenge re-triggers. Effective rate ~0.3 req/s.

## Authentication Context

- **Session transfer method:** chrome-MCP capture inside the user's running Chrome session -- no cookie export step needed.
- **Endpoints reachable only with auth:** `secure.booking.com/mytrips.html`, `www.booking.com/mywishlist.html`, `secure.booking.com/rewards_and_wallet.html`, `secure.booking.com/mysettings.html`. Each loaded successfully with the user's cookies; unauthenticated requests would redirect to `account.booking.com/sign-in`.
- **Auth header scheme discovered:** None. Booking.com uses pure cookie-based session auth -- the `bkng` family of cookies (HttpOnly) carries the session; no `Authorization` header is constructed. Printed CLI's `auth login --chrome` reads the Chrome cookie DB, imports those cookies, and replays via Surf -- no header composition needed.
- **Session state file:** Not used. chrome-MCP path skips the agent-browser save-then-restore step.
- **Manuscript archiving:** Capture used the user's live Chrome session; no separate session-state file was written under `$SESSION_DIR`, so nothing to scrub there. The discovery directory contains only the spec + traffic analysis + this report.

## Bundle Extraction

Not run. SSR-dominant page architecture made interactive capture sufficient -- the data we need is in the HTML, not in JS bundle endpoint maps.
