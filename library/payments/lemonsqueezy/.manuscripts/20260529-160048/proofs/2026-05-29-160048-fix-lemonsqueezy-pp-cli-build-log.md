# Lemon Squeezy CLI — Phase 3 Build Log

Manifest transcendence rows: 8 planned, 0 built. Phase 3 will not pass until all 8 ship.


## Phase 3 Implementation

All 8 transcendence rows now built:

| Command | LoC | Data source | Highlights |
|---|---|---|---|
| revenue-snapshot | ~180 | local | Joins denormalized store counters with refund-adjusted local orders |
| mrr-trend | ~190 | local | ISO-week buckets; new/renewal/refunded split with week-over-week delta |
| churn-watch | ~190 | local | Status-set filter (past_due/unpaid/cancelled/expired) over a window with customer email + last-invoice exposure |
| dunning-alert | ~150 | local | SQL join: invoices.status=failed AND subscription.status IN (active, past_due) |
| license-rollup | ~210 | local | Triple-table join (license-keys × instances × variants) with per-variant aggregates + top-N keys |
| refund-cascade | ~190 | auto | Live cascade: GET order → license-keys filtered by order_id → PATCH disabled=1 (only with --apply) |
| campaign-watch | ~180 | local | Per code: used vs cap, 24h velocity, linear sellout projection |
| webhook-audit | ~190 | local | URL-host grouping with stale-host heuristic (localhost/ngrok/loca.lt/serveo/*.test/*.local/*.internal) |

Shared helpers added to internal/cli/ls_helpers.go: toFloatLS, toStringLS, toBoolLS, parseLSTime.

Manifest transcendence rows: 8 planned, 8 built. Phase 3 Completion Gate PASS:
- Per-row Cobra resolution: PASS (8/8)
- Dogfood backstop: planned=8, found=8

Build status:
- go build ./... → exit 0
- go vet ./... → clean (no warnings emitted by build)
- All 8 novel commands respond to --help and --dry-run correctly
