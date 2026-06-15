# Squire CLI Brief

## API Identity
- Domain: Squire (getsquire.com) — barbershop booking & discovery marketplace (B2B SaaS for shops + consumer discovery/booking front end).
- Surface targeted: the consumer **discovery** app at `getsquire.com/discover/barbershop/{shopId}` (Next.js, basePath `/discover`).
- Users: people finding/booking barbershops; shop owners checking their public listing; competitive-intel on local shops (services, pricing, staffing, ratings).
- Data profile: per-shop profile, service menu (price+duration), staff roster + next availability, paginated reviews + AI summary, geo location.

## Reachability Risk
- **None/Low.** Anonymous `curl` (Chrome UA) returns HTTP 200 on both the page (372KB) and the data endpoint (289KB). Cloudflare is in front (`__cf_bm` cookie, `server: cloudflare`) but does not challenge plain HTTP with a browser UA.
- Probe-safe endpoint: `GET /discover/_next/data/{buildId}/barbershop/{shopId}.json?shopId={shopId}` → 200, full pageProps.
- No auth required for read/discovery. (Booking would require auth; out of read-only scope.)

## Replayable Surface (confirmed)
- **Next.js data route** is the canonical client surface: `GET https://getsquire.com/discover/_next/data/{buildId}/barbershop/{shopId}.json?shopId={shopId}` returns `{serverFlags, pageProps:{shop, services, barbers, reviewsResponse, ...}}`.
- `api.getsquire.com` does NOT resolve publicly (server-side only) — the data route is the surface, not a REST API.
- **buildId rotates per deploy** (currently `g2iQPKxK1pQ1ZdVRro5p3`). CLI must fetch current buildId from the page's `__NEXT_DATA__` (or `/_next/static/{buildId}/_buildManifest.js`) before calling the data route. Standard Next.js scrape pattern.
- serverFlags reveal a **city/location discovery** surface (`cityPagesV2`, `fetchCityPagesShopsFromApi`, `showBarbersOnCityPage`, `cityPagesReviews`) — shop search by location, to be confirmed via sniff.
- Reviews are paginated server-side (perPage=5, 61 pages for this shop) — a reviews-pagination endpoint exists to discover.

## Data Model (from live __NEXT_DATA__)
- **shop** (165 fields): id (uuid), name, route (slug), location {lat,lng}, phone, email, instagram, barberCount, userCount, yelpRating, googlePlaceId, googleReviewUrl, timezone, paymentProcessor, customerBookingFee, requiresPrepayment, BNPL config.
- **services** (33): name, duration (min), cost / costWithTaxes / costWithoutTaxes (cents), categories[], assignments[], depositType, costRange/durationRange (for variable services).
- **barbers** (8): barber{70 fields: firstName, lastName, route, instagram, enabled, assignedServices, gender, timezone, ...}, nextAvailableTime (ISO), nextAvailableTimeText ("Available Wednesday"/"From Jul 2nd"), defaultServiceName, instagramImages[].
- **reviewsResponse**: averageRating, numberOfRatings (301), summary (AI-generated prose), reviews[5], pagination{count,page,perPage,totalPages}.

## Concrete Anchor (Barber Theory, the URL's shop)
- 8 barbers, 10 users, 301 reviews @ 5.0 avg, Toronto (43.666, -79.353), stripe payments, $1.00 booking fee.
- Prices in cents: Haircut $45.00 (45min), premium Haircut $60–70, Haircut & Beard $55 (60–75min).

## Top Workflows
1. **Look up a shop** by slug → full profile (services, staff, ratings, contact, location).
2. **Browse the service menu** with prices + durations, filter by category, sort by price.
3. **See staff + next availability** ("who can cut my hair soonest").
4. **Read reviews** + AI summary; page through all reviews; filter by rating.
5. **Discover/search shops** by city/location (lat/lng), compare across shops.

## Table Stakes (vs Booksy / Fresha / Vagaro / StyleSeat / Schedulicity)
- Shop search by location, service menu with prices, staff listing, availability lookup, reviews/ratings display, shop contact + map link. (Booking itself is auth-gated and out of read-only scope.)

## Data Layer
- Primary entities: shops, services, barbers, reviews. Local SQLite mirror enables: cross-shop price comparison, cheapest-service-by-category, soonest-available-barber, review sentiment over time, watch a shop for price/staff changes (snapshot diff).
- Sync cursor: per-shop fetch (no global list without location search); reviews paginate.
- FTS: services by name/category, shops by name/neighborhood, review text search.

## Product Thesis
- Name: **squire-pp-cli** — "Every barbershop on Squire, queryable from the terminal."
- Why it should exist: the website shows one shop at a time with no compare, no price sort, no 'soonest available across shops', no historical price/review tracking. A local SQLite mirror unlocks cross-shop and over-time queries the site never built.

## Build Priorities
1. Foundation: HTTP client with dynamic buildId resolution; SQLite store for shops/services/barbers/reviews; sync + search + sql.
2. Absorb: shop lookup, services list (filter/sort), barbers list + availability, reviews (paginate/filter), shop search by location.
3. Transcend: cross-shop price compare, cheapest-by-category, soonest-available-barber across shops, review-summary + rating trend, watch-shop price/staff drift (snapshot diff).
