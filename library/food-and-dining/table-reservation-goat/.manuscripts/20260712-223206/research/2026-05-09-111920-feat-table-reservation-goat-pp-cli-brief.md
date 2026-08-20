---
api_name: table-reservation-goat
sources: [opentable, tock]
primary: opentable
secondary: tock
generated_at: 2026-05-09
---

# Table-Reservation-GOAT CLI Brief

A combo CLI uniting OpenTable and Tock — the two largest US restaurant-reservation networks that share zero data with each other — under one local store, one search index, and one set of agent-friendly commands.

## API Identity
- **Domain:** restaurant reservations and prepaid culinary experiences
- **Users:** diners hunting for tables (Resy/Yelp/SeatGeek-adjacent power users), agents booking on behalf of users, restaurant savants tracking cancellations, anyone who has ever opened both OpenTable and Tock in adjacent tabs to compare availability for the same night.
- **Data profile:**
  - Restaurants/venues (slug, name, cuisine, neighborhood, geo, price band, network)
  - Availability slots (date, time, party size, slot token, attributes — patio/bar/highTop/standard/experience)
  - Experiences (Tock's specialty: prepaid tasting menus, chef counters, pop-ups, exclusives)
  - Reservations (id, party, time, payments, refunds, notes, ratings — Tock has 33+ data-model definitions for this alone)
  - Authenticated user state (saved restaurants, reservation history, dining points, gift cards, credits)

## Reachability Risk
**Medium.** Both consumer sites sit behind Cloudflare with TLS fingerprinting; vanilla `stdlib` HTTP clients get 403 instantly. `printing-press probe-reachability` confirmed both clear via Surf (Chrome TLS):
- `www.opentable.com` → `mode: browser_http` (stdlib 403, Surf 200, confidence 0.85)
- `www.exploretock.com` → `mode: browser_http` (stdlib 403 with `cf-mitigated: challenge`, Surf 200, confidence 0.85)

Implication: **the printed CLI ships Surf transport, not browser-clearance**. No `auth login --chrome` needed for clearance cookies. Cookie import is still wanted as an opt-in for authenticated commands (real bookings, my-reservations) since OpenTable's `make-reservation` POST requires a logged-in session.

Secondary risk: OpenTable's persisted-query sha256 hashes drift over time (Autocomplete/RestaurantsAvailability/ExperienceAvailability). A dedicated `pp doctor --refresh-hashes` path or browser-sniff capture is the durable answer; pinning hashes in code without a refresh strategy is the recipe for silent breakage. Two open issues confirm this is real (`chrischall/opentable-mcp #1`, 2026-04; `Henrymarks1/Open-Table-Bot` issues #32–35, 2024-Q1).

## Top Workflows
1. **Find a table tonight** — "Where can I get 4 covers at 7pm in the West Village, $$$ or under?" Cross-network search beats OpenTable's siloed view.
2. **Find prepaid/experience availability** — Alinea, French Laundry, Atomix — Tock-only territory. OpenTable users miss these entirely today.
3. **Earliest available** — Pick a venue (or 5), find the soonest open slot in the next N days across both networks.
4. **Watch for cancellations** — A hot restaurant is fully booked; poll until a slot opens, alert. Resy's "Notify" is the gold standard; OpenTable+Tock have no equivalent.
5. **Book a reservation** — Authenticated, requires session. Print-by-default `--launch` opt-in pattern keeps the verifier safe.
6. **My reservations** — List/cancel reservations across both accounts in one place.

## Table Stakes (absorbed from competing tools)
From `21Bruce/resolved-bot` (Go), `xunhuang/yumyum-v2`, `whoislewys/opentable-reservation-bot`, `spudtrooper/opentable`, `singlepatient/tablehog` (Rust), `Henrymarks1/Open-Table-Bot`, `jrklein343-svg/restaurant-mcp` (12-tool MCP), `chrischall/opentable-mcp` (browser-extension capture), `azoff/tockstalk` (Cypress sniper), `michel-adelino/scrapping`, plus MCPs `duaragha/opentable-mcp` and `bedheadprogrammer/reservationserver`:

- Restaurant search (autocomplete + faceted by date/time/party size/cuisine/price/neighborhood)
- Restaurant detail (address, phone, hours, cuisine, price band, ratings, photos)
- Availability check (slots for a specific restaurant + date + party size)
- Experience listings (Tock's prepaid lane)
- Make a reservation (slot lock + book with payment for Tock; slot token + book for OT)
- List my upcoming reservations
- Cancel a reservation
- Watch/snipe for slots opening (every 30s–5m polling; instant book when match)
- Push notifications on cancellations (Slack webhook or local notification)
- Search by location (lat/lng/metro/neighborhood)
- Save/star/favorite restaurants

## Data Layer (ALL synced to local SQLite)
- **Primary entities:** `restaurants` (cross-network), `availability_slots`, `experiences`, `reservations`, `watches`, `restaurants_fts` (FTS5)
- **Sync cursor:** per-network last-synced-at timestamps; restaurants synced on demand (search-driven), availability synced on watch-driven cadence
- **Cross-network linking:** `restaurants` table has `network` (`opentable`|`tock`), `network_id`, plus optional `match_signature` (normalized name + geo) for fuzzy cross-linking — letting `pp restaurants list "alinea"` return both networks if the venue is on both
- **FTS5:** restaurant name + neighborhood + cuisine; allows offline regex-friendly search
- **Reservation history & ratings:** stored locally so `pp insights` can compute "where I've eaten the most", "average party size", "favorite cuisine", etc.

## Codebase Intelligence (community ground truth)
**OpenTable consumer surface (5+ wrappers agree):**
- Base: `https://www.opentable.com/dapi/fe/gql` — GraphQL persisted-query
- Required headers: `x-csrf-token` (extract from any HTML page's `__CSRF_TOKEN__` global), Chrome TLS fingerprint, optional `ot-page-group`/`ot-page-type` for newer captures
- Operations confirmed: `Autocomplete`, `RestaurantsAvailability`, `ExperienceAvailability`, `LocationPicker`, `HeaderUserProfile`, `BookDetailsExperienceSlotLock`
- Booking REST: `POST /dapi/booking/make-reservation` with `slotAvailabilityToken` + `slotHash` from availability call (tokens expire in minutes); needs `authCke` cookie
- HTML pages: `/s` (search results page with `__APOLLO_STATE__`), `/booking/restref/availability?rid=<id>` (restaurant landing)
- Spreedly tokenization for paid experiences

**Tock surface (no public REST API found in any wrapper):**
- All wrappers (azoff/tockstalk, michel-adelino/scrapping, jacktrossi/OxfordReserve) use **browser automation** (Cypress, Selenium, or mocked).
- `jacktrossi/OxfordReserve` claims a `/api/v1/venues/...` Bearer-auth surface but the code path is gated by `if (!TOCK_API_KEY) return getMockAvailability()` — **fictional**.
- `api.exploretock.com/docs/latest/reservation.swagger.json` exists (we fetched it: 34KB Swagger 2.0, 33 definitions, 0 paths) — pure data-model spec, no callable surface. Title: "Tock Reservation Model v2.7" by `eng@tockhq.com`.
- The Tock SPA at `exploretock.com/{venue}` (e.g. `/alinea`) and `/{venue}/experience/{id}/{name}` *must* hit XHR endpoints — they just haven't been documented anywhere public. Browser-sniff is the only path to discover them.
- Selectors confirmed in tockstalk: `[data-testid=consumer-calendar-day]`, `[data-testid=search-result-time]`, `[data-testid=purchase-button]`, `[data-testid=receipt-confirmation-id]`, `[data-testid=email-input]`, `[data-testid=password-input]`.

**MCP servers in the wild:**
- `jrklein343-svg/restaurant-mcp` (TypeScript, Node) — covers Resy + OpenTable, 12 tools incl. `snipe_reservation` (poll every 500ms after release time)
- `duaragha/opentable-mcp` — Playwright, browser-driven
- `bedheadprogrammer/reservationserver` — Playwright, both networks

## User Vision
User-confirmed: a "GOAT" CLI for table reservations. Combo of OpenTable + Tock with logged-in browser sessions on both. No partner key. Live smoke testing in Phase 5 will use the user's actual logged-in browser cookies for real-account validation.

## Source Priority
**Confirmed via the priority gate.** Marker file at `$API_RUN_DIR/source-priority.json`:
- **Primary: OpenTable** — community wrappers + 5+ documented consumer endpoints + medium-confidence reachability via Surf. Auth: anonymous for search/availability; cookie session for booking.
- **Secondary: Tock** — no public REST API, but rich domain (33-def Swagger), differentiated experiences (prepaid/exclusive), fully browser-discoverable. Auth: cookie session via Surf.
- **Economics:** Both free at the user level (consumer sites). No paid keys required for either.
- **Inversion risk:** **Real.** Tock's documented Swagger could be misread as "Tock has the better spec, make it primary" — but that swagger has zero callable paths, just a webhook payload model. OpenTable's GraphQL is the only network with a known callable surface as of Phase 1. Do not invert.

## Product Thesis
- **Name:** `table-reservation-goat-pp-cli` (binary), display name **"Table Reservation GOAT"**
- **Why it should exist:** No tool today gives diners (or agents acting on their behalf) one query that hits both OpenTable and Tock with offline search, structured JSON, watch/snipe for cancellations, cross-network restaurant matching, and a local store of every search and reservation history. Resy's Notify is the closest comparable; it covers exactly one network. The combo CLI compounds: see when Alinea (Tock) frees up at the same minute you see French Laundry (OpenTable) move on the calendar; pick whichever fits.

## Build Priorities
1. **Surf-based clients** for both networks — `internal/source/opentable/` (GraphQL persisted-query + REST booking) and `internal/source/tock/` (browser-sniffed XHR; if browser-sniff yields nothing, HTML extraction from venue pages with structured-data fallback)
2. **Local store** with cross-network restaurant model + FTS5 — `restaurants`, `availability_slots`, `experiences`, `reservations`, `watches`
3. **Anonymous read commands** — `restaurants search`, `availability`, `experiences list` (work without login)
4. **Authenticated commands** behind `auth login --chrome` cookie-import — `book`, `my-reservations list`, `cancel`
5. **Transcendence (built in Phase 3, not the generator):**
   - `pp goat <query>` — single command across both networks, ranked by combined relevance + earliest availability
   - `pp watch <restaurant> --party 2 --window "Friday 7-9pm"` — local sniper, polls both networks, alerts on slot
   - `pp earliest <venue1>,<venue2>,<venue3> --party 4 --within 14d` — cross-venue earliest-available
   - `pp insights` — local-store aggregations on the user's reservation history
   - `pp drift <restaurant>` — what changed since last sync (new experiences, slot price moves)
6. **Per-source rate limiting** — `cliutil.AdaptiveLimiter` for both sources; surface `*cliutil.RateLimitError` instead of empty JSON when 429s exhaust retries
