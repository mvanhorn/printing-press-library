Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Booksy CLI — Build Log

## Built
- **8 generated read commands** from the hand-authored spec: `businesses search/get/reviews`, `discover suggest/locations/treatments`, `me profile/home`.
- **6 hand-built novel commands** (all shipped, none stubbed):
  1. `book <id>` — dry-run-by-default preview via Booksy's dry_run endpoint; `--confirm` posts the real booking; refuses under any test harness (`cliutil.IsAnyHarness`). (book.go)
  2. `availability <id>` — POST time_slots wrapped as clean flags; day-grouped open slots; token-gated. (availability.go)
  3. `services <id>` — flattens service_categories→services→variants to expose bookable `service_variant_id` + price + duration; public. (services.go)
  4. `earliest <id>` — soonest open slot in a `--within` window; typed exit 3 on no-slot; token-gated. (earliest.go)
  5. `compare <id> <id>…` — parallel fetch with partial-failure accounting; rating + cheapest matching service; public. (compare.go)
  6. `cheapest` — scan-and-filter with `--max-scan` cap + fan-out; ranks nearby businesses by cheapest matching service price; public. (cheapest.go)
- Shared domain parsers in `booksy_domain.go` (business/service/variant/time-slot structs, flatten, diacritic-insensitive EN→PL service matching).

## Auth model
- Public reads work with the baked public `x-api-key` constant (in `required_headers`) — no token.
- `me/*`, availability, earliest, book require `BOOKSY_ACCESS_TOKEN` (sent raw in `x-access-token`).
- Constant headers `x-api-key`, `x-app-version: 3.0`, `accept-language: pl`, `x-fingerprint` sent on every request.

## Booking safety
- `book` never commits during verification/dogfood (harness refusal). Preview is default; `--confirm` required to place a real appointment. The dry_run payload is captured/verified; the confirm payload mirrors it (never exercised by automated tests).

## Deferred / notes
- `discover locations` returns Booksy city-name string suggestions (not ids); location scoping in `cheapest`/`businesses search` uses `--location-id`/`--location-geo` when known. Resolving a city name → id is a minor future refinement.
- Confirm-booking response shape not captured live (user stopped before confirming, by design); `book --confirm` surfaces any server error verbatim.
