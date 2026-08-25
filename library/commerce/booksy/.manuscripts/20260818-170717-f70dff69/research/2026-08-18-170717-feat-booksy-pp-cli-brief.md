# Booksy CLI Brief

## API Identity
- Domain: Booksy — marketplace + booking platform for beauty/wellness/barber services. Poland is Booksy's origin market; `booksy.com/pl-pl/` is the Polish consumer marketplace.
- Users: Consumers finding and booking appointments (haircuts, barber, nails, spa) near a location; the CLI targets the consumer/customer surface, not the Booksy Biz partner surface.
- Data profile: Businesses (salons/barbers) with geo, rating, services (name/price/duration), staff/stylists, opening hours, reviews, photos; availability time-slots per service/date/staff; authenticated user's appointments and booking flow.

## Reachability Risk
- [Medium] No official public consumer API. The web app uses an internal per-country REST API (host family `booksy.com/api/pl/2/customer_api/...`) with a baked-in `x-api-key` header. Public read data (search, business detail, services, reviews, availability) is reachable anonymously with that key; booking requires the logged-in session token (`x-access-token` / Bearer). Live capture from the logged-in Chrome session is authoritative for exact PL paths, headers, and payloads.
- Bot protection: unknown until probe/capture. Runtime transport TBD by `probe-reachability` (likely `standard_http` or `browser_http`). Booking flow captured from the real logged-in session.

## Top Workflows
1. **Book a haircut** (headline, user-stated): search barbers/salons in a Polish city → open business → pick a haircut service → check availability for a date → select a slot → confirm booking. Booking is a real mutation; ship as print-by-default + explicit `--confirm`, refuse under any test harness.
2. Find barbers/salons near a location, filtered by service/category, sorted by rating/distance.
3. Inspect a business: services + prices (PLN), staff, hours, reviews, rating.
4. Check availability / open slots for a service on a date (and optionally a specific stylist).
5. Manage own appointments: list upcoming/past bookings, view detail, cancel.

## Data Layer
- Primary entities: businesses, services, staff, reviews, appointments (user), availability slots (ephemeral, live).
- Sync cursor: search is query/geo-scoped (not a full crawl); cache business detail + services + reviews per business id. Appointments sync from the authenticated account.
- FTS/search: local FTS over cached businesses (name, category, services, city) for offline recall of places the user has looked at.

## User Vision
- "Book a haircut through the CLI" — in Poland (`booksy.com/pl-pl`). The end-to-end booking path is the product's reason to exist. Auth: user is logged in to booksy.com in Chrome (authenticated capture available).

## Product Thesis
- Name: Booksy CLI (`booksy-pp-cli`)
- Why it should exist: Book a haircut (or any Booksy service) from the terminal/an agent — search, compare prices, check open slots, and confirm a booking without opening the app. No existing CLI does this; the value is an agent-native, scriptable path through Booksy's booking funnel plus a local cache of businesses/prices/availability for fast comparison.

## Build Priorities
1. Discovery: authenticated browser capture of the PL consumer API booking funnel (search → business → services → availability → book) + auth token/header construction.
2. Public read commands: search, business, services, reviews, availability (safe to dogfood live).
3. Auth + account: session import from Chrome, my appointments, appointment detail.
4. Booking: guided `book` command (print-by-default, `--confirm`, harness-refusal), plus cancel.
5. Transcendence: local price/availability comparison across cached barbers, next-open-slot finder, etc.
