# SEA Airport Parking CLI Brief

## API Identity
- **Domain:** `reservesea.portseattle.org` — the Port of Seattle's official advance-reservation portal for **on-airport garage parking** at Seattle-Tacoma International (SEA).
- **Platform:** White-label **AeroParker** (a Metropolis Technologies company; formerly SP+). Stateful Java servlet app (`JSESSIONID; Path=/book`), nginx + AWS ALB. The same engine powers IAH/HOU, LAS, ORD, ORF, and Charlotte — the CLI is multi-airport-reusable by swapping host + airport code.
- **Users:** SEA travelers who want to guarantee an on-airport space (Floor 4 of the main garage, contactless QR entry/exit) rather than gamble on drive-up availability.
- **Data profile:** One reservable product — **"Reserved Parking"** (code `RP`, id `335`, carParkId `105`). Priced per-day (~$47/day; General drive-up is $37/day and NOT reservable here). Availability is date-range-dependent and sells out before peak dates.

## Reachability Risk
- **None.** Runtime mode `standard_http`. A plain GET bootstraps cookies (`AWSALB`, `JSESSIONID`, `parkingReservationGuid`); a POST with a normal Chrome UA replays cleanly. No bot wall, no clearance challenge, no login required for availability/quote. Confirmed live 2026-07-13.
- Probe-safe endpoint used: `GET /book/SEA/Parking?parkingCmd=collectParkingDetails` then `POST /book/SEA/Parking`.

## The Contract (verified live)
```
GET  /book/SEA/Parking?parkingCmd=collectParkingDetails      # bootstrap session cookies
POST /book/SEA/Parking
     parkingCmd=collectParkingDetails & progressToNextStep=1
     & entryDate=MM/DD/YYYY & entryTime=HH:MM
     & exitDate=MM/DD/YYYY & exitTime=HH:MM
     & promocodes=<optional>
```
- **Available** → 303 to `?parkingCmd=selectProduct`; HTML embeds JSON: `quotes:[{"id":335,"price":188.0,"originalPrice":"$188.00"}]`, a **per-date price array** `[{"date":"2026-08-12T00:00:00Z","price":47.0,"rollupPrice":0.0}, ...]`, travel-extras (`shopProductAvailabilityDetails`), and `data-product-name="Reserved Parking" data-product-code="RP" data-product-id="335"`. Parse the embedded JSON directly — no browser at runtime.
- **Sold out** → selectProduct page with greyed SOLD OUT + "Sorry, this product is unavailable for the dates you requested."
- **Invalid dates** → re-renders collectParkingDetails with inline errors: "minimum 2 day stay is required.", "must be made at least 6 hours in advance.", "AT LEAST 14 DAYS IN ADVANCE FOR BEST AVAILABILITY".
- **Constraints:** max 120 days ahead; min 6h before entry; min 2-day stay; free modify/cancel up to 6h before entry. Height limit 6'10". Times are 1h granularity plus `00:01`/`23:59`.

## Top Workflows
1. **Quote a date range** — "how much and is it available for Aug 15-18?" → the headline command. Auth-free.
2. **Sweep candidate dates** — quote several entry/exit combos to find the cheapest / any-available date. No competitor can do this for the official garage.
3. **Watch a sold-out date** — poll until Reserved becomes available, then notify. The #1 unmet need.
4. **Track price/availability over time** — persist every quote to SQLite; show drift across snapshots.
5. **Quote → booking handoff** — build the prefilled booking (guest checkout, no login) and stop before payment; never auto-submit a card.

## Table Stakes
- Availability + price check by entry/exit datetime (aggregators do this for *off-airport* lots; none can quote the official Floor-4 product).
- Promo-code price surfacing (`--promo` → `promocodes`).
- Availability calendar / multi-date view.
- JSON / agent-native output.

## Data Layer
- **Primary entities:** `product` (Reserved Parking: id, code, carParkId, color, reviews), `quote` (entry/exit datetime, total price, original price, per-date prices, promo, availability, captured_at), `watch` (a saved date-range being polled).
- **Sync cursor:** none upstream; the store IS the value — every quote snapshot is a row, enabling price/availability history that exists nowhere else.
- **FTS/search:** low priority (one product); the store's value is time-series quotes, not text search.

## Reachability / Auth Notes
- Core value (availability, quote, sweep, watch, history) is **auth-free**. Guest checkout means even booking needs no account.
- User confirmed a logged-in session, but the authenticated surface (`/book/SEA/MyAccount?cmd=login`, existing-booking management) is secondary. Recommend scoping v1 to the auth-free read/quote/watch surface + quote-and-handoff booking. Confirm at absorb gate.

## Competitive Position (why this should exist)
- **reservesea is the SOLE channel for the on-airport Floor-4 Reserved product.** SpotHero, ParkWhiz, Way.com, BestParking list off-airport lots only — none can quote or book this space. A CLI here is the only programmatic access that exists.
- No public CLI/script targets reservesea (only unrelated "Parkalot" hits on GitHub).
- Omar has an internal `browser-use` skill (`chief-of-staff:sea-parking`) that drives a headed browser; this CLI is a strict upgrade — the quote/availability step needs only HTTP, and it adds price history + sold-out alerts the skill lacks. Carry over its entry/exit-time heuristics (departure −2h, arrival +1h).

## User Vision (from briefing)
- User is logged in and initially wanted authenticated endpoints captured. Core value is auth-free; booking is safest as quote+handoff (mirror the "never auto-submit payment" rule from the existing skill).

## Product Thesis
- **Name:** `sea-airport-parking` (binary `sea-airport-parking-pp-cli`). Prose display: "SEA Airport Parking".
- **Why it should exist:** The only way to check, track, and get alerted on the official SEA on-airport Reserved garage from the terminal — with a local price/availability history and sold-out watch that no tool, aggregator, or the site itself offers.

## Build Priorities
1. Auth-free HTTP client: GET-warm session → POST dates → parse embedded `quotes` + per-date JSON. SQLite quote store.
2. `quote` / `availability` (headline), `products`, `doctor`.
3. Transcendence: `sweep` (cheapest/any-available across dates), `watch` (poll sold-out → notify), `history`/`drift` (price+availability over time), `calendar` (per-date price view), `book --quote` (prefilled guest-checkout handoff, no auto-pay).
