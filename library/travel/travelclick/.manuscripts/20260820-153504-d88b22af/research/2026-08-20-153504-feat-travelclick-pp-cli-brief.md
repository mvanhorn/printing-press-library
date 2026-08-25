# TravelClick CLI Brief

## API Identity
- **Domain**: TravelClick / iHotelier — a Central Reservation System (CRS) and direct booking-engine widget, now part of Amadeus Hospitality. Powers the *direct* booking page for thousands of independent and boutique hotels (not an OTA, not a metasearch aggregator — this is each hotel's own "book direct" widget).
- **URL pattern**: `bookings.travelclick.com/{hotelId}?hotelid={hotelId}#/...` — one numeric hotel ID per property. Example fixture: **Made Hotel NYC, hotel ID `102306`**.
- **Backend**: `api.travelclick.com`, split into versioned service prefixes discovered via live capture + main JS bundle cross-reference (`ibe-shop/v1`, `ibe-codes/v1`, `ibe-entity/v1`, `ibe-book/v1`, `ibe-guest/v1`, plus `loyalty/v2` and `cybersource/v2` for payment). Frontend is an Angular SPA ("booking-engine-4" / "BE4" / "Web4"), served by an Express host.
- **Users**: price-conscious travelers who want to check a hotel's own rates (vs. OTA markup), corporate/group travelers with a rate code, and repeat guests of a specific boutique property.
- **Data profile**: read-only rate/availability search, calendar-style lowest-rate scans, rate-code validation, and public hotel profile info (address, policies, check-in/out times). No official public developer API exists for this surface — TravelClick's documented API (`roli854/travelclick` on GitHub, `api-evangelist/travelclick-amadeus`) is a **separate, B2B-only SOAP/XML CRS-to-PMS integration** gated behind a hotel's own contract. The consumer widget's JSON API is undocumented.

## Reachability Risk
- **Low.** `probe-reachability` on the booking page returned `mode: standard_http`, confidence 0.95 — no WAF/JS challenge on the page shell.
- Direct `curl` to `api.travelclick.com/ibe-shop/v1/hotel/102306/avail` with **only** `Authorization: Bearer <token>` + standard headers returned a clean `200` with full JSON — **no Akamai Bot Manager telemetry header required**, despite Akamai Bot Manager cookies (`_abck`, `bm_sz`, `bm_sv`) being set on the page. Akamai appears to be present for scoring/analytics, not as a hard gate on these read endpoints.
- No GitHub issues found reporting 403/blocked/rate-limited access to this specific consumer surface (searched `travelclick`, `ihotelier` on GitHub code search — only the B2B SOAP integration and API-Evangelist's documentation profile turned up, no consumer-widget wrappers, no blocked-access reports).
- **Auth is the real risk, not transport.** See Auth Gap below.

## Top Workflows
1. **Search availability & compare rate plans** for specific check-in/check-out dates, adults/children/rooms — the #1 workflow, mirrors what a human does on the widget itself (`rates search`).
2. **Find the cheapest night to check in** across a date range (up to ~60 days) before committing to specific dates (`rates calendar`) — the widget's own calendar view does exactly this client-side; we call the same endpoint directly.
3. **Validate a corporate, rate-access, or group-attendee code** before or instead of a full search (`codes validate-corporate`, `codes validate-group`) — useful for travelers who have a discount code but don't know if it's still active.
4. **Look up a hotel's profile**: address, geo-coordinates, check-in/out times, accepted cards, and — notably — the *textual* explanation of mandatory fees (e.g. Made Hotel's $35/night "Curation Fee") that the booking flow discloses but easy to miss (`hotel info`).
5. *(Cross-hotel, novel)* **Batch-compare multiple TravelClick-powered hotels** for the same dates — each hotel's widget is siloed to itself; nothing lets a user check 3 boutique hotels in the same city side by side without opening 3 browser tabs.

## Table Stakes
- Room type + rate plan listing with cancellation policy, guarantee policy, and promo/sale labeling (the widget shows "Lock It In / Non-Refundable", "Stay 3 Nights and Save", "Best Available Rate", "Breakfast Package" — all just labeled rate plans under the hood).
- Per-night price breakdown including the mandatory service/resort/curation fee, which is charged *in addition to* the displayed nightly rate (`totalServiceChargeExclusive`) — a frequent source of "why is my total higher than the rate I saw" confusion that the CLI should surface explicitly.
- Calendar/heatmap of lowest rate per day.
- Special/corporate/group code entry with clear validity feedback (the widget shows an inline "We could not find this code" message; the API returns a structured `errorCode` + `errorMessage` we should surface the same way).

## Data Layer
- **Primary entities**: `hotels` (id, alias, name, address, timezone — hotel IDs are not memorable, so aliasing matters), `rate_snapshots` (hotel_id, check_in, check_out, room_type, rate_plan, nightly_rate, fees, captured_at — for price-drift tracking, matching the pattern already used by this user's `hotel-tonight` and `1688` CLIs), `code_checks` (hotel_id, code_type, code, valid, error_message, checked_at).
- **Sync cursor**: none — this API is date-range/point-query driven, not paginated.
- **FTS/search**: hotel alias/name → hotel_id lookup (fuzzy), backed by `learn.entity_lookup_seeds` seeded with Made Hotel NYC and extensible by the user via `--save-hotel`.

## Auth Gap (read before generating)
- Confirmed: **OAuth2 `client_credentials` Bearer JWT**. Decoded claims: `iss: travelclick`, `app: TCBE4`, `mech/partner: WEB4`, `amr: client_credentials`, `exp - iat = 3600s` (~1 hour TTL). The same token works across `ibe-shop`, `ibe-codes`, and `ibe-entity` — it is an app-level credential, not tied to an individual user or hotel.
- **Not found**: the exact token-mint endpoint. It fires during Angular bootstrap before any interceptor (Performance API, XHR/fetch monkey-patch, or a `Page.addScriptToEvaluateOnNewDocument`-injected hook set up *before* navigation) could attach — evidence points to either (a) an extremely early XHR that filled the capture buffer before drain, or (b) server-side minting by `bookings.travelclick.com`'s own Express host with the token delivered through a call this session didn't isolate. Grepped all 5 JS bundles for `grant_type`/`client_credentials`/`oauth`/`connect/token` literals — none found, consistent with server-side minting.
- **Decision for v1**: ship with **manual token capture**. `TRAVELCLICK_TOKEN` env var / `--token` flag, ~1 hour validity, with copy-paste DevTools instructions in the README (Network tab → filter `api.travelclick.com` → copy the `Authorization` header value). This is honest about the gap rather than guessing a token endpoint that could be wrong. Flagged as the top `emboss`/follow-up candidate once someone captures the bootstrap call with a longer-running network trace.

## Reachability Gate
- Decision: PASS
- Evidence: Live discovery calls to `GET /ibe-shop/v1/hotel/102306/avail`, `POST /ibe-shop/v1/hotel/102306/basicavail/multi-room`, and `GET /ibe-entity/v1/hotel/102306/info` each returned `200` with real JSON bodies using a captured Bearer token and no other special headers. This satisfies the Phase 1.9 gate's "2xx/3xx -> PASS" row directly; no additional probe needed before Phase 2.

## Source Priority
- N/A — single source (no combo CLI).

## Product Thesis
- **Name**: `travelclick` (matches slug convention).
- **Why it should exist**: TravelClick/iHotelier is one of the most widely deployed direct-booking engines for independent and boutique hotels, many of which advertise "book direct and save" price-match promises against OTAs. No tool — official or reverse-engineered — lets a traveler check rates, find the cheapest date, validate a discount code, or compare several such hotels without opening a browser per property. This CLI is strictly **search/info only**: no reservation, no guest PII, no payment data ever touches it, matching the existing `capital-one-travel` and `concur` precedent in this same personal CLI library.

## Build Priorities
1. `rates search` — core workflow, room+rate-plan comparison with fee-inclusive pricing and policy text.
2. `rates calendar` — cheapest-night finder over a date range.
3. `codes validate-corporate` / `codes validate-group` — code validation with clear valid/invalid surfacing.
4. `hotel info` — property profile, policies, fees explained.
5. *(Novel, local data layer)* hotel aliasing (`--save-hotel`), rate-snapshot price-drift history, multi-hotel batch compare.
