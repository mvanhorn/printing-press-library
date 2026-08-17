# Build Log — sea-airport-parking-pp-cli

Manifest transcendence rows: 5 planned, 5 built. Phase 3 will not pass until all 5 ship.

## Built
- internal/parking/client.go — AeroParker warm-GET→POST→parse flow (embedded quotes JSON, per-date prices, sold-out + invalid detection), adaptive rate limiting, verify-env short-circuit.
- internal/store/quotes.go — parking_quotes snapshot table (lazy CREATE IF NOT EXISTS), insert + range query (NULL-safe scans, drain-first).
- internal/cli/quote.go — headline: quote price/availability + nightly breakdown, persists. (absorbed, hand-built)
- internal/cli/book.go — guest-checkout handoff, quotes then prints prefill steps; --launch opens browser (verify-safe); never auto-pays. (absorbed, hand-built)
- internal/cli/sweep.go — transcendence 1: flexible-date cheapest/any-available, scan-cap + note.
- internal/cli/watch.go — transcendence 2: poll until available/under-price, --notify, dogfood single-poll curtail.
- internal/cli/history.go — transcendence 3: local snapshot list, missing-mirror guard.
- internal/cli/drift.go — transcendence 4: computed first/latest/min/max + availability flips.
- internal/cli/calendar.go — transcendence 5: live per-entry-day grid for a month.
- internal/parking/client_test.go — parser table tests (available/soldout/invalid/dedup/snapTime/billedNights).

## Live-validated
- quote 3-night stay -> $141 (3×$47), correct nightly array, available=true.
- invalid short stay -> invalid=true with site message.
- quote x2 -> drift snapshots=2, currently_available=true.

## Deferred (per approved manifest)
- Booking auto-submit / payment (quote+handoff only).
- Authenticated my-bookings management.
- Multi-airport (single-airport SEA v1; parking.Airport const).
