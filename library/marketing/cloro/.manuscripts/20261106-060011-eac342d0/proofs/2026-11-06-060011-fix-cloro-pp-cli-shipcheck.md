# cloro-pp-cli shipcheck

Command: `cli-printing-press shipcheck --dir <cli-dir> --spec cloro-openapi.json --research-dir <run-dir> --no-live-check` (no API key in the environment; live behavior is proven by the Phase 5 live dogfood).

| Leg | Result |
|-----|--------|
| verify | PASS, 42/42 (0 critical) |
| validate-narrative | PASS, 9 narrative commands resolved, full examples passed |
| dogfood | PASS (WARN: one dead generated helper, `retainCLIQueryParams`; sync has no default resources because no endpoint is a syncable list) |
| workflow-verify | workflow-pass |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | 92/100, Grade A |

## Blockers found and fixed
1. Loop 1: verify-skill failed. A README troubleshooting line put `cited --engines` on the same line as `cloro-pp-cli credits`, so the checker read `--engines` as a `credits` flag. Fixed in research.json (the line no longer names the flag) and regenerated. Loop 2: 7/7 legs PASS.

## Scorecard gaps (not fixed)
- Cache Freshness 5/10: scrapes are paid and live by design; no sync-backed reads.
- Insight 6/10, MCP Token Efficiency 7/10, Data Pipeline Integrity 7/10, Dead Code 4/5 (one generated helper).

Later spec changes (auth description, help examples for goto and product, a JSON payload for the async create probe, and the `async` resource name) were each followed by a shipcheck re-run. Final run on the shipped tree: 7/7 legs PASS, verify 42/42, scorecard 92/100 (Grade A).

Verdict: ship
