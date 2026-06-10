Manifest transcendence rows: 6 planned, 0 built. Phase 3 will not pass until all 6 ship.
(5 hand-code: drops, dedupe, deal, spread, watch | 1 spec-emits: auctions)
Foundational hand-build (absorbed, not transcendence): find (live token-signed search), sources.

## Plan
- internal/autotempest/client.go: token signing + per-source /queue-results fan-out + /api/facebookMarketplace + /api/searchtempest/direct + searchAfter pagination + listing parse.
- internal/store/autotempest_migrations.go: at_listings, at_price_snapshots, at_saved_searches tables (+ indexes).
- internal/cli/find.go: live search -> persist + snapshot -> emit.
- internal/cli/sources.go: static 10-source registry.
- Fill novel stubs: watch (add/ls/run/rm), drops, dedupe, deal, spread, auctions.
- root.go: wire find + sources.

## Built (Phase 3 complete)
Transcendence rows: 6 planned, 6 built (drops, dedupe, deal, spread, watch, auctions). novel_features_check: found 6 / planned 6 / missing none.
Foundational: find (live token-signed search, concurrent fan-out, async polling, pagination, persist+snapshot), sources.
Token signing verified live (server accepted self-computed token; 20 real Civic + 12 Tacoma listings returned).
Store: at_listings, at_price_snapshots, at_saved_searches (internal/store/autotempest_migrations.go).
Client: internal/autotempest/ (token.go, parse.go, listing.go, sources.go + tests).
Notes: st/fbm sources use divergent envelopes -> best-effort (tagged fetch_failures), never abort. queue-results is async (poll until populated).
