Manifest transcendence rows: 7 planned, 7 built. Phase 3 passed: all 7 ship.

# Hostex reprint build log (press 4.33.0, spec 3.15.0)

## What was built
- Fresh generation from official spec v3.15.0 (88 operations); 3.15.0 additions present as generated flags: `transactions query --reservation-code` (stay-code kept as deprecated alias), `transactions create` reservation_code body, `reservations create --channel-id`, Airbnb price_and_rules blackout-date settings.
- Seven hand-written transcendence commands carried over from the published tree (ops-gaps, stay-brief, inbox-sla, automation-preview, price-parity, oversell-watch, revenue-rollup) + novel_helpers.go, store/extras.go, hostex_envelope.go.
- Re-applied library-side fixes onto the fresh template: error_code envelope hook in client.go; sync offset=0 on first page, 365-day transactions window, reviews keyed by reservation_code (sync.go + store.go), default sync resources without automation/pricing-ratios, transactions-window help; listings update-* body-shape help; `which` write-intent guard (which.go hook + which_writes.go).
- Dropped as absorbed by the generator: json-body-scalars (typed flags), path-parameter encoding (cliutil.EscapePathParam), ambiguous-write retry gate (platform.CanRetryRequest), batch rollback count.

## Deferred / limitations
- MCP surface defaults to the Cloudflare orchestration pattern (86 endpoints > 50): endpoint tools hidden, transport [stdio,http].
- Novel-features subagent could not be spawned from the worker context; prior subagent output reused.
- regen-merge dry-run needed a synthesized 4.26.1 base (old press can't build under Go 1.27 on this machine).
