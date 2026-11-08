# SpotHero CLI Brief

## API Identity
- Domain: consumer parking reservations (search, quote, book, cancel, parking passes) in US and Canadian cities and airports.
- Users: drivers who reserve parking ahead of time: commuters who book the same garage repeatedly, travellers booking airport or event parking, assistants and agents booking on an owner's behalf.
- Data profile: public inventory (facilities, rates, quotes) with no login; private account data (reservations, vehicles, saved cards, profile) behind a session cookie.
- Homepage: https://spothero.com (the brand's own site).

## Reachability Risk
- Low. Direct HTTP with a plain client works for both hosts. No WAF challenge seen on `spothero.com/api/v1` or `api.spothero.com/v2` on 2026-11-07.
- Probe-safe endpoint used: GET /api/v1/search-params/ (public) and GET /api/v1/users/me/ (session).

## Session mechanism (established live, 2026-11-07)
- The `AUTH_TOKEN` cookie on `.spothero.com`, an RS256 JWT issued by accounts.spothero.com (Auth0) with a 30 day expiry, is the only thing the API needs. Sending that cookie alone returned 200 from `/reservations/` and `/users/me/`.
- Sending the same value as `Authorization: Bearer` or `Token` with no cookie returned 401. `appSession.*`, `sh-sso`, and `csrftoken` alone returned 401.
- Root cause of the false login: Chrome stores values in `encrypted_value`; the prior importer read the empty `value` column via sqlite3 when `browser_cookie3` was not installed, then saved empty values and reported success.
- `auth0`, `did` etc. are host-only on accounts.spothero.com and are never sent to spothero.com, so the importer must not include them.

## Top Workflows
1. Find parking near an address or point for a time window, compare by total price and distance.
2. Quote one facility and book it, with spend cap, gated for agents.
3. Check upcoming reservations and pull the parking pass details on the day.
4. Cancel a reservation when plans change, seeing whether it is refundable first.
5. Rebook the same garage for a recurring commute.

## Table Stakes
- Search by address or lat/lng, quote, book, list and get reservations, cancel, saved vehicles and cards.
- Time zone correctness for naive local datetimes.

## Data Layer
- Primary entities: reservations (private), facilities and cities and airports (public reference).
- Sync cursor: reservations are paged (`meta.next`).
- FTS/search: reservations by facility title, city.

## Booking and cancel contract (from the site's own checkout bundle, 2026-11-07; never executed)
- `POST /api/v1/checkout/` with `SpotHero-Version: 2026-10-06`. Prior CLI sent 2025-04-28.
- Body: `items[]{item_type:"rental", price, add_ons:[], item_context{facility, search_id, action_id, phone_number, quote_token, rental_source_title, starts, ends, is_event_rate, rule_group_id?, vehicle_profile_id | license_plate_str + license_plate_state | license_plate_unknown}}`, `payment{use_spothero_credit, cards[{card_external_id}]}`, `total_price`, `currency`, `email`.
- The site no longer sends item level `rate_id`, `quote_token`, `quote_mac`; the prior CLI did.
- `GET /reservations/{id}/refund-eligibility/` (verified live, read only) gives `original_source_refund.is_eligible`, `credit_only_refund.is_eligible`, `cancellation_reasons[]`.
- `POST /reservations/{id}/refund/` with body `{reason, access_key, refund_as_credit}`. Prior CLI sent an empty body. `reason` is the `reason` string of a cancellation reason.

## User Vision
- The owner wants to book and cancel for real: both are core features that must work, with gates: preview by default, `--confirm "PLACE SPOTHERO BOOKING"`, `--yes --confirm <token>` bound to the preview, a `--max-total` cap, no auto-retry after an ambiguous placement (read state and report). MCP exposes search, reservations and previews, not place or cancel.
- `auth login --chrome` must really capture the session with no manual token copying, and auth must verify against the API.

## Product Thesis
- Name: spothero-pp-cli
- Why it should exist: SpotHero has only a B2B partner API. Nothing else lets a person or agent search, book and cancel from a terminal, and an agent needs spend caps and explicit gates a web checkout does not offer.

## Build Priorities
1. Native Chrome session import, verified auth.
2. Search, quote, book and cancel on today's contract, gates preserved.
3. Reservations with pass details and a local store; vehicles, cards.
4. Agent ergonomics: JSON, select, typed exit codes, MCP read surface.
