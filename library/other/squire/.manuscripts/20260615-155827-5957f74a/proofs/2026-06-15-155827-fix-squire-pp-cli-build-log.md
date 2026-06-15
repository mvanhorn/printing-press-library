# Squire CLI Build Log

Manifest transcendence rows: 5 planned, 5 built. All transcendence rows ship.

## Plan
All 5 novel commands are hand-code (live multi-fetch, except watch = live + local snapshot diff).
- soonest (live) — rank barbers across shops by nextAvailableTime
- compare (live) — stacked compare of 2+ shops on price/rating/staff
- cheapest (live) — cheapest service-by-category across shops
- watch (live+local) — snapshot diff via generic resources store
- roster (live) — rank a city's shops by rating × log(reviews)
Shared: internal/cli/squire_helpers.go (fetch + resolve slug→UUID).

## Built
(updated as rows complete)
