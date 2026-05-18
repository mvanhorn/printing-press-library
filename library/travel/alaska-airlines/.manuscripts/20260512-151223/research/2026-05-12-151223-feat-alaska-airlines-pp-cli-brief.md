# Alaska Airlines CLI Brief

## API Identity
- Domain: Air travel — flight search, booking, mileage plan, seat selection
- Users: AS frequent flyers (Mileage Plan members), award travelers, families coordinating multi-pax bookings
- Data profile: Live XHR (search/seat-map/availability), session-cookied account state (Mileage Plan, reservations), stable references (airports, partners, award charts)

## Reachability Risk
- **Mixed**. Homepage probed `standard_http` (stdlib GET 200, no protection). But:
  - Search/shopping XHR is JS-driven; needs cookie session + likely XSRF/CSRF headers
  - Booking POST is the riskiest — almost certainly anti-replay (nonce/CSRF/timing) + DataDome/Akamai-class WAF
  - Mileage Plan account endpoints need authenticated cookie (user has session in Chrome — usable)
- AS GitHub orgs (AlaskaAirlines, Alaska-ECommerce, Alaska-ITS) publish only the Auro design system — no booking/SDK published as open source
- AS NDC API exists (developer.alaskaair.com / Microsoft Azure API Mgmt) but is **partner-only** (agency/IATA accreditation required, no consumer access, currently no ARC settlement)

## User Vision (verbatim, captured in briefing)
> Flight search + reservations is the key thing. Picking a SEAT, being able to say who's traveling with me with natural language that might be saved on my profile, be able to store CSV files in a text file locally for my credit card so it can book on it's own without me typing. I want full soup to nuts BOOKING. Like I want to be able to say "Hey book my family on this date to this date, give me flight options, then book everyone and choose seats" type thing.

**Update post-briefing**: card stored on alaskaair.com (card-on-file). CLI does NOT store card data — it submits booking using AS's saved card token. Removes biggest security risk.

## Top Workflows
1. **Search flights** (origin, destination, date/range, pax count) — return options with fare + miles + cabin
2. **View seat map** for a flight — picking specific seats per pax
3. **Build itinerary + book** for a saved passenger group — POST to AS reservation endpoint with card-on-file
4. **Mileage Plan** — balance, recent activity, MVP/MVP Gold progress, upgrade list, segment history
5. **Award space search** — partner award space (Cathay, JAL, Hainan, etc.) using AS Mileage Plan miles
6. **Manage reservations** — view, change, cancel existing PNRs

## Table Stakes (every existing tool has these — we MUST match)
- Flight search (route + date) → fares + cabin classes (flightplan, awardwiz, seats.aero)
- Award availability search (revenue or miles) (flightplan, awardwiz, seats.aero, awardgeek)
- Multi-airline support — competitors search 5–10 airlines (we focus on AS, but must beat AS-specific UX)
- Headless-browser scraping for JS-rendered content (flightplan = Puppeteer; awardwiz = Arkalis)
- Persisted credentials (flightplan stores in config/accounts.txt — we'll use macOS keychain)

## Data Layer
- **Primary entities**:
  - `passengers` — saved traveler profiles (name, DOB, KTN, gender, MP#)
  - `airports` — IATA + name + city + region (3-letter codes used everywhere)
  - `searches` — search params + result snapshot (cached for re-quote, scoring, watchlist)
  - `flights` — from search results: flight#, dep/arr time, equipment, cabin, fare, miles
  - `seat_maps` — per flight: row/seat → status (open/taken/exit/preferred), price if extra
  - `reservations` — PNRs we've seen (number, pax, status, segments)
  - `mileage_plan_account` — balance, status (MVP/MVPG/MVP100K), tier progress, recent activity
  - `award_space_snapshots` — per partner+route+date: cabin availability + miles cost
  - `booking_attempts` — every attempted booking: request, response, success/failure, timestamp
- **Sync cursor**: per-resource last_synced_at; reservations/mileage-plan use account-wide cursor
- **FTS/search**: searches (text), reservations (PNR/pax), passengers (name)

## Codebase Intelligence (from prior art)
- **flightplan-tool/flightplan** — Node.js + Puppeteer/Headless Chrome. Subcommands: `search` / `parse` / `import` / `cleanup` / `stats` / `client` / `server`. Login via `cx.initialize({ credentials: ['account_id', 'password'] })`. Stores creds in `config/accounts.txt`. Supports AS award search.
- **lg/awardwiz** — TypeScript. Custom "Arkalis" scraping engine with anti-bot mitigations. WiFi + lie-flat detection. Points-to-cash via Skiplagged. Searches AS, AA, Aeroplan, Delta, JetBlue, Southwest, United.
- **seats.aero** — commercial site (no public scraper) — sells access to AS award data with availability calendar UI
- **igolaizola/flight-award-scraper** — Apify actor for award scraping incl. AS Mileage Plan, REST API + Node/Python SDKs, exports JSON/CSV/Excel

## Architectural Constraint
PP CLIs ship as static Go binaries with optional Surf transport (browser-fingerprint HTTP). They do NOT ship a headless Chrome.

Implication for AS:
- ✅ **Search/award/seat-map/MP**: replayable via cookie session + Surf. JSON XHR endpoints behind authenticated cookies — browser-sniff once, replay forever.
- ⚠️ **Booking POST**: high uncertainty. AS's actual `OrderCreate` (or web equivalent) probably uses session-bound CSRF/nonces. May require a fresh in-browser navigation that we cannot replay headlessly without bundling Chromium (which PP doesn't do).
- 🟡 **Mitigation paths if pure replay fails**: (a) ship "book" as `would-book` + opens prepared URL in user's browser — the user clicks final submit; (b) shell out to `osascript`/`open -a` to drive the user's logged-in Safari/Chrome to a deep link with itinerary pre-loaded; (c) HOLD if neither works.

## Product Thesis
- **Name**: alaska-airlines-pp-cli
- **Why it should exist**:
  - Existing AS tools (flightplan, awardwiz) are bulky Node+Chrome stacks meant for power users running servers. None ship as a single binary.
  - Nothing combines **search + book + seat select + Mileage Plan** in one CLI for the AS-only flyer
  - Agent-native shape: an LLM agent should be able to say "find me 4 seats together LAX→ANC June 12, prefer extra-leg-room" and get JSON back, then say "book it" and have it execute
  - Local SQLite enables "watchlist" and "drift" features no website offers (e.g., "tell me when seat 7A opens up on AS123")

## Build Priorities
1. **P0 (foundation)**: data layer for all 9 primary entities; cookie-import auth via `auth login --chrome`; doctor; agent-native flag stack (`--json`, `--select`, `--csv`, `--quiet`, `--dry-run`)
2. **P1 (absorb table stakes)**: flight search, fare details, schedule lookup, status, award availability, partner award charts (static data), seat-map view, Mileage Plan balance + activity + upgrade list, reservation list/get/cancel-link
3. **P2 (transcend — these define the CLI)**:
   - `book` — full booking attempt using card-on-file (best-effort replay; honest fallback if anti-replay blocks)
   - `passengers` — natural-language passenger groups ("my family", "wife and kids")
   - `seat watch` — watch seat map for openings (first row, exit, window pair)
   - `search drift` — track price/availability changes for saved searches
   - `family seat-find` — find N seats together (rare on full flights — agent-native query)
   - `award sweet-spot` — flag partner routes priced below new dynamic pricing (AS now dynamic, partners still chart-based)

---

## 2026-05-12 Re-run Findings (browser-use Phase 1.7 capture)

Successful Phase 1.7 capture via CDP Input.dispatchMouseEvent (real OS-level mouse — bypasses Auro's shadow-DOM listener bindings, which were the blocker for the prior run).

### Rebrand: Mileage Plan -> Atmos Rewards
The loyalty program was rebranded to "Atmos Rewards" with tiers like Atmos Gold / Atmos Diamond. CLI naming, env vars, and docs must use "Atmos Rewards" canonically; "Mileage Plan" stays as a historical reference only.

### Booking funnel fully mapped
Home -> /search/results (SvelteKit SSR, URL-deeplinkable) -> outbound fare select -> return fare select -> POST /search/results?/handleAddToCart -> /search/cart (URL-deeplinkable, /__data.json available) -> POST /search/cart?/checkout -> /book/guest-info (passenger info). Stopped before passenger data entry per safety rule.

### Key technical findings
- `/search/results/__data.json?O=...&D=...&OD=...&DD=...&A=N&RT=true|false` is the SvelteKit data endpoint — returns the full fare matrix as JSON. No HTML parsing needed.
- `/search/cart/__data.json?A=...&F1=ORIG|DEST|MM/DD/YYYY|FLIGHT,CARRIER|FARECLASS|...&F2=...&Slices=...&FARE=N&FT=rt|ow` for cart state.
- `/search/api/citySearch/lookup/codeshare/{IATA}` and `/search/api/citySearch/getAllAirports` for airport reference data.
- `/search/api/shoulderDates` POST for nearby-date pricing matrix (flexible-dates feature).
- `apis.alaskaair.com/1/marketing/loyaltymanagement/wallet/wallet/balance?mileagePlanNumber=N` for Atmos points balance.
- Cookie auth via Chrome import; Auth0 issuer at auth0.alaskaair.com with JWT in `guestsession` cookie (30-min expiry, refreshable via `/account/token`).

### Booking POST reality
The final pay submit (`/book/payment` or similar) was NOT captured (stopped before personal data entry). Assumed to be CSRF-tokened + 3DS-payment-authed + possibly CAPTCHA-gated. **Not replayable from a Go binary.** Final book step ships as `book prepare` that emits a deeplink URL (`/search/cart?A=N&F1=...&F2=...&FARE=...&FT=rt`) and opens it in the user's browser; the user clicks "Continue to checkout" and completes the final pay.

### Anti-bot surface
`/_fs-ch-1T1wmsGaOgGaSxcX/check-detection` (FullStory ClientDetection) fires during page loads. Quantum Metric session beacons, AppDynamics RUM also present. The XHR API surface is NOT WAF-gated (probed cleanly via Surf fingerprint), only the rendered pages have client-side bot detection that doesn't affect direct API calls.
