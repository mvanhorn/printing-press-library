# WeWork (WorkplaceOne) Desk-Bookings CLI Brief

## API Identity
- **Product:** WeWork WorkplaceOne member portal (`members.wework.com/workplaceone`)
- **Surface built from:** the authenticated web app itself (browser-sniffed via the user's logged-in Chrome session on the "Book a desk" flow). No official public API / OpenAPI spec exists.
- **Base URL:** `https://members.wework.com/workplaceone/api`
- **Backend:** internal "wework-yardi" + "spaces" + "common-booking" services behind the members portal BFF.
- **Users:** WeWork members / on-demand desk users who book common-area desks by the day across WeWork buildings.
- **Data profile:** cities (~842) → buildings/locations (geo, hours, amenities, availability) → bookable desks/shared workspaces (capacity, credits, price, seat availability, cancellation policy) → the member's own bookings.

## Reachability Risk
- **Low (auth-gated, not bot-gated).** The API host `members.wework.com/workplaceone/api/*` answered 200 to replayed XHR with the composed-bearer headers. No Cloudflare/WAF/CAPTCHA challenge seen on the API host itself — the gate is authentication, not bot-protection. Reachability mode: `browser_clearance_http` (token comes from the browser session).
- **Token expiry is the real risk.** The `authorization` header is a short-lived auth0 access-token JWT. Replays work only until it expires; the CLI must re-import a fresh token from a live browser login.

## Auth (the defining constraint)
- **Composed bearer.** Every data endpoint requires three request headers:
  - `authorization: Bearer <auth0 JWT>` (short-lived, from the auth0 SPA session)
  - `weworkuuid: <account/member uuid>`
  - `weworkmembertype: <member type code>`
- **Token source:** the auth0 SPA cache in the browser (`@@auth0spajs@@…` localStorage), plus `CurrentAccountUUID` / `WWMemberType`. There is no long-lived API key.
- **CLI implication:** auth is `composed` / browser-session style. The CLI authenticates by importing the token + uuid + member-type from a logged-in Chrome session (the skill's `auth login --chrome` / `press-auth` companion pattern), storing them locally, and replaying until expiry. Env fallback: `WEWORK_TOKEN`, `WEWORK_UUID`, `WEWORK_MEMBER_TYPE`.

## Top Workflows (read-first, per approved scope)
1. **Find desks in a city on a date** — the headline flow: pick city + date → search bookable desks with credits/price, capacity, amenities, availability, hours.
2. **Browse WeWork buildings/locations** in a city with availability counts and operating hours.
3. **List my upcoming bookings.**
4. **List bookable cities** (with geo) — the entry point that also supplies the lat/lng used to build search bounding boxes.
5. **Filter/inspect amenities** for locations.

## Table Stakes
- Search desks by city + date, filter by capacity/amenities/location type.
- Show price (credits + currency amount), seat availability, hours, cancellation policy per desk.
- List the member's bookings.
- Offline/agent-native output: `--json`, `--select`, typed exit codes, local store of cities/locations for fast repeat queries.

## Data Layer
- **Primary entities:** `city` (name, marketgeo lat/lng, country, nearby_location), `location`/building (uuid, name, address, geo, hours, amenities, availability count), `desk`/shared-workspace (uuid, capacity, credits, productPrice, seat availability, cancellation policy, hours, amenities), `booking` (upcoming).
- **Sync cursor:** cities and locations are slow-changing → good local-store candidates (sync once, query offline). Desk availability is time-sensitive → live fetch.
- **FTS/search:** city name, location name, address, amenity name.
- **Geo trick:** `get-affiliate-cities` returns each city's marketgeo lat/lng → derive the `boundnwLat/Lng` + `boundseLat/Lng` bounding box that `get-spaces` / `get-locations-by-geo` require. This is what makes a city-name-driven CLI possible without the map UI.

## Endpoints captured (all GET, all 200, all auth-required)
| Command intent | Path | Key params |
|---|---|---|
| List cities | `/wework-yardi/location/get-affiliate-cities` | (none) |
| Search desks | `/spaces/get-spaces` | bounds, city, date, capacity, duration, locationType, locationUUIDs, limit, offset, roomTypeFilter |
| Locations (avail) | `/spaces/get-affiliate-locations` | bounds, city, date, endDate, type |
| Locations by geo | `/wework-yardi/ondemand/get-locations-by-geo` | bounds, city, accountUUID, userLat/Lng |
| Amenities | `/wework-yardi/booking/start-locations-for-amenities-list` | amenity_uuids, location_uuids, page_size |
| My bookings | `/common-booking/upcoming-bookings` | (none) |
| Recents/favorites | `/recent-and-favorite/v2/get-recents-and-favorite-location-data` | requestType, spaceType |

## Not captured (deliberate)
- **Booking create/confirm (POST).** Not triggered — it would place a real desk reservation on the user's account. Read-first scope. Can be captured later with explicit consent by walking the confirm step.

## Product Thesis
- **Name:** `wework-pp-cli` ("wework")
- **Why it should exist:** WeWork's desk booking is a map-and-modal web UI with no CLI and no public API. A member who wants to script "is there a desk near me tomorrow, and what will it cost in credits?" has no option today. This CLI turns the reverse-engineered read API into a scriptable, agent-native desk-finder: `wework desks --city "New York, NY" --date 2026-08-13 --json` → structured desk list with credits/price/availability — plus a local store of cities/buildings so repeat queries are instant and offline.

## Build Priorities
1. **Auth + data layer.** Composed-bearer auth with Chrome/token import; local SQLite store for cities + locations.
2. **`cities`** (verify-safe, no params) and **`bookings`** (my upcoming) — the two credential-only, no-geo reads. Great first commands and smoke tests.
3. **`locations`** and **`desks`** — city+date search with bounding-box derivation from the city geo. The headline value.
4. **Transcendence:** offline city/location search (FTS), "cheapest desk near a city" ranking by credits, availability-aware filtering, and agent-native compact output.

## User Vision
- User pointed at `.../bookings/desks` and said "I'm logged into WeWork, scrape the cookie on my Chrome session." Scope confirmed: **desk bookings, read-first** (no mutation commands required to ship).
