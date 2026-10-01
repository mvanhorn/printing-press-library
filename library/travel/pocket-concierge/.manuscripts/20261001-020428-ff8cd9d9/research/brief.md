# Pocket Concierge research and agreed scope

Observed 2026-09-30 UTC (2026-10-01 Singapore). Sole builder; exactly one fresh-context reviewer. The user preapproved focused scope and ordinary gates, and prohibited additional planning workers or shared/global config changes. Those instructions override the Press novel-feature-worker and global-update defaults.

## Source and economics
- https://www.pocket-concierge.jp/en/ returns 200 HTML shell; its public guest bundle exposes POST /graphql queries. GET /graphql returns HTML, not API JSON.
- Unauthenticated POST query returns areas, cuisines, venuesSearch, venue, courses, availabilityCalendar, availabilitySearch. No API key, paid account, cookies or CSRF needed for these reads. Account, payment, reservation mutation and secret-seat surfaces excluded.
- Source contract: https://www.pocket-concierge.jp/packs/js/guest-bundle-f6f35f471f7ac878aa23.js . Extract only public read documents; do not archive full HTML/auth headers.
- First-party FAQ: https://www.pocket-concierge.jp/lp/aboutus/index.html . Payment/account requirements apply to booking on the website, not CLI discovery. Restaurant prices are JPY; course fixedPrice is per group, costPerGuest per guest as rendered by the first-party UI.
- Sample search: 856 currently published results; pagination metadata includes currentPage, limitValue, totalCount, totalPages. This is a snapshot, not a permanent count or exhaustive dining coverage.
- English content does not establish English-speaking service. Source services flags and verbatim reservationTerms are the evidence.
- Masumasu Masuda 245672 / course 182402 shows both a course inclusion statement and a reservation-policy JPY 1,000 charge. Preserve both; never infer an all-in total.
- ReservableAvailability has id; WaitlistableAvailability has no source session id. Preserve null, start/end/course identity, party bounds and distinct waitlist status. realTimeBooking controls instant confirmation versus reservation request; neither is an existing reservation.

## Reachability / coverage
Standard HTTPS public reads work. Sandboxed shell DNS is disabled; research/live checks use authorized network execution. The public website is an undocumented, changeable API, so schema/protection failures must exit predictably, never become zero results. No public rate limit guarantee found. Finite timeout, one retry, serial requests, bounded pages and response bodies. Request availability freshly by default; opt-in short cache visibly reports age.

## Users and workflows
Japan fine-dining travelers and agents shortlist restaurants by place/cuisine/date/party/budget, inspect course/policy details, compare actual sessions, then hand the canonical first-party restaurant link to the diner. No booking or account writes.

## Ecosystem and positioning
Searches for Pocket Concierge API/SDK/CLI/MCP/npm/PyPI found no maintained source-backed wrapper to absorb. An API Evangelist profile and commercial scraper marketing provide no usable first-party contract. TableCheck and Tabelog offer adjacent restaurant workflows but are excluded providers. No same-product entry/lock was found in the Press library/registry. Focus on public source evidence, bilingual identity, compact output, explicit freshness and safe handoff.

## Data and implementation
Restaurant and course IDs, Japanese/English names, canonical URLs; on-demand detail. Small bounded response cache, no SQLite dependency or automatic whole-catalog sync. Inventory refresh is explicit. Live search pages bounded to 1–3, limit 1–50. Filters resolved by source IDs, not invented geographic mappings.

## Build priorities / acceptance
1. filters and restaurants search/get: source-native pagination/filtering, Japanese names matched by ID.
2. courses list: per-guest and fixed-group JPY values; exact summaries and policy fee statements.
3. availability dates/slots: requested date/party, Asia/Tokyo, session/course identity, request/instant/waitlist distinction.
4. booking handoff: validate venue/course/session ownership and provide canonical page; no guessed booking URL parameters.
5. Agent controls: select projection, compact JSON/errors, dry-run, freshness metrics, bounded cache/refresh, doctor, schema/context/help.
6. Deterministic consequential domain/error/cache tests, live query-relevance/correctness matrix, cached/uncached metrics, independent review/fixes, Press acceptance/promotion.
