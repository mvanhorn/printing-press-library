# Forkable CLI Build Log

Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

## Built (Phase 3)
- Manifest transcendence rows: 7 planned, 7 built.
- All 7 novel features implemented as live GraphQL commands (pp:data-source live):
  served-history, preference-drift, why-picked, spend-trend, allowance-burn, upcoming-digest, venue-rotation.
- Shared helpers: forkable_gql.go (fetchGraphQL + envelope), forkable_dates.go (since parsing), forkable_csrf.go (client CSRF injection).
- CSRF handshake: one-line hand-edit in client.go request loop injects x-csrf-token on GraphQL POSTs (marked pp:hand-edit forkable-csrf).
- Read-only: 9 spec-emitted read commands + framework. No mutations.
- Tests: forkable_logic_test.go (table-driven for parseLooseSince, dateOnOrAfter, conflictTerms, periodKey, int64sToList, venue label). Scaffold help-wiring tests pass.

## Deferred / notes
- GraphQL arg-inlined queries (menus, meal-scores, venue-usage) ship with placeholder-0 default queries; require --query override with real ids. High-value commands work with zero args.
- No sync path for POST-query resources; novel features are live-fetch (correct for per-user small datasets).
