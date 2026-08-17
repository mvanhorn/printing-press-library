# Shipcheck: grubhub-pp-cli

Verdict: ship

## Legs (all PASS, 6/6)
verify PASS · validate-narrative PASS · dogfood PASS · workflow-verify PASS · verify-skill PASS · scorecard PASS

## Scorecard: 91/100 — Grade A
Notable: Output Modes/Auth/Error/Terminal/README/Doctor/Agent/MCP Quality/Local Cache/Workflows all 10/10.
Soft (by design): Cache Freshness 5/10 (location-scoped search, no time cursor — auto-refresh intentionally off), Breadth 7/10 (read-only v1, order history deferred), MCP Token Efficiency 7/10.
Domain Correctness: Path Validity 10/10, Auth Protocol 10/10, Sync 10/10, Dead Code 5/5.

## Live sample probe: 4/4 (100%)

## Known Gaps
- Order history / re-order and cart/checkout require a credentialed account login; out of v1 scope (read-only marketplace browsing). Documented in README.
- Raw `restaurants` dogfood fixtures (restaurant/item ids) are live values that may rot on a future reprint; refresh pp:happy-args if so.

Ship recommendation: ship
