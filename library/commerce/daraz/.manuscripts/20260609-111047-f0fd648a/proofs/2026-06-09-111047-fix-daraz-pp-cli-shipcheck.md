# Daraz CLI — Shipcheck

Verdict: PASS (6/6 legs) on rerun.

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (86/100, Grade A); live sample probe 7/7 (100%) |

## Blockers found + fixed
1. Search parser crashed on `totalResults` returned as a JSON string (some queries) — fixed with flexInt + map-based field stringification (productFromMap). Caught by live smoke.
2. `parseMoney` mishandled a currency prefix with a dot — fixed with a number regex. Caught by unit test.
3. verify-skill: `seller products` leaf name collided with top-level `products` search — renamed to `seller listings`.
4. validate-narrative: quickstart referenced non-existent `products search ... --limit 5` — fixed research.json + README/SKILL to `products --query ... --sort priceasc` (no --limit flag on the generated search).

## Scorecard notes
- Strong (10/10): output modes, auth, error handling, terminal UX, README, doctor, agent-native, MCP quality/desc/remote, local cache, workflows, insight.
- Softer: Cache Freshness 5 (no upstream auto-refresh hook — manual sync/watch by design), Breadth 7, Path Validity 5 (hand-coded commands not spec endpoints). All acceptable for v1.

Final recommendation: ship.
