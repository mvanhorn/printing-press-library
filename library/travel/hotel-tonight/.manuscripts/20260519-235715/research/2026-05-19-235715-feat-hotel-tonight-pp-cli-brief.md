# HotelTonight CLI Brief

## API Identity
- Domain: Last-minute / same-night hotel booking (launched 2011, **acquired by Airbnb 2019**). Curated, time-boxed, geo-local discounts on boutique/independent hotels. Signature feature: **Daily Drop** — once-a-day app-only flash deal.
- Users: Travelers booking tonight/tomorrow/weekend; deal-hunters; the geo-local "I'm here now, where's cheap" loop.
- Data profile: Geo + date + rooms → time-decaying deal/rate inventory. Highly ephemeral by design.

## API Surface (unverified — needs live capture)
- Consumer feed host: `api.hoteltonight.com`, REST, path-versioned. Only consistently-cited endpoint: `GET /v6/inventory` (rooms/deals feed).
- **Do NOT target** `api-docs.hoteltonight.com` / `api-htx.hoteltonight.com` — those are the supply-side hotelier/partner portal behind HTTP Basic auth, not the consumer API.
- No official public/developer API. Endpoints are undocumented app-backend calls discovered via HAR; "may stop working at any time."
- REST assumed; one stray GraphQL mention, unconfirmed. Expected params (geo lat/lng or HT location id, check-in/out or tonight/tomorrow/weekend enum, rooms, currency/locale) and required app headers (app version, UA, possible `x-ht-*` key / anon session) are NOT publicly documented — must come from capture.

## Reachability Risk
- **HIGH.** Evidence:
  - Airbnb-owned → shared anti-bot infra. Airbnb documented as Akamai Bot Manager user; 2026 dominant vector is TLS fingerprinting (JA3/JA4), which 403s a plain Go HTTP client regardless of headers.
  - Geo-gating confirmed by a dedicated bypass repo (bettyipxiu/htotn-vpn): deals/pricing vary by IP/geo, flash sales region-locked, fraud systems flag shifting/non-residential IPs.
  - No community success stories — only commercial "we'll HAR-scrape it for you" services (stevesie, thunderbit, realdataapi, travelscrape). The fallback everyone uses is HAR replay, not direct programmatic calls.
- **Consequence:** runtime is likely `browser_http` (Surf/Chrome TLS) at best, possibly `browser_clearance_http`, possibly unreachable header-only. Confirm with `probe-reachability` + browser capture before generating.

## Top Workflows (public endpoints only, all read-only)
1. Find tonight's cheapest hotel near me (geo → inventory → rank by price / % off).
2. **Watch a city/geo for price drops over time** — snapshot prices, diff, alert. The biggest thing the CLI can do that the app cannot.
3. Compare deals across neighborhoods in one metro.
4. Track the Daily Drop longitudinally (record what the app makes ephemeral).
5. Date-shift scan: tonight vs tomorrow vs weekend for the same area.

## Table Stakes (competitors, not absorb)
- Priceline Tonight-Only / Express Deals — closest analog (app-only same-night, % off rack rate, neighborhood/star/amenity filters).
- Booking.com / Hotels.com last-minute — broad inventory, same-day filters.
- Must-have fields: neighborhood/geo filter, star rating, amenities, price, **% off rack rate as first-class**.

## Data Layer
- Primary entities: `hotels` (id, name, neighborhood, lat/lng, stars, amenities), `geos`/`cities` (HT location id ↔ lat/lng ↔ display name), `deals` (hotel × date window × quoted rate × %off × deal type), `rates` (per-room pricing/fees), **`price_snapshots`** (hotel_id, geo, checkin, observed_at, price, %off) — the high-value time-series table.
- Sync cursor: (geo + check-in date + observed_at). Poll-based, time-series-shaped, not delta-shaped (undocumented feed has no server cursor).
- FTS: hotel name, neighborhood, amenities, city display name.

## Ecosystem to Absorb
- **None.** Zero community CLI, SDK, API client, or MCP server. Greenfield. Feature set comes from product workflows, not prior art. The only HotelTonight GitHub repos are their own old infra (HTAutocompleteTextField, shameless, looker-slackbot) — irrelevant.

## User Vision
- "Let's go, but assume we aren't doing anything authenticated." → anonymous/public endpoints only. No booking, no account, no auth flows. AUTH_SESSION_AVAILABLE=false.

## Product Thesis
- Name: `hotel-tonight` (binary `hotel-tonight-pp-cli`).
- Why it should exist: HotelTonight's value is deliberately ephemeral and app-trapped — deals appear, drop, vanish; you only see *now, here*. A CLI flips that into a persistent, queryable, agent-drivable price-intelligence layer: sync the anonymous inventory feed into local SQLite, snapshot prices over time, ask "cheapest near me tonight," "alert when this neighborhood drops below $X," "Daily Drop history for this city" — offline, scriptable, none of which the app does. **Caveat that shapes the build:** behind Airbnb/Akamai bot defenses + IP geo-gating, so step one is live reachability proof; if not reachable from a CLI transport, scope shrinks or run HOLDs.

## Build Priorities
1. Local SQLite data layer for hotels/geos/deals/rates/price_snapshots + sync + FTS search + SQL.
2. Core read commands: search/list deals by geo+date, hotel detail, rank-by-price / rank-by-%off.
3. Transcendence: price-drop watch (snapshot diff), neighborhood comparison, Daily Drop tracker, date-shift scan — all powered by the local time-series store.
