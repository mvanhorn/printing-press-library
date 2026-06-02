# Zapmail CLI Shipcheck

## Umbrella verdict: PASS (6/6 legs)

| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

## Scorecard: 92/100 - Grade A
- Output Modes 10, Auth 10, Error Handling 10, README 10, Doctor 10, Agent Native 10, MCP Quality 10, MCP Remote Transport 10, Local Cache 10, Workflows 10, Path Validity 10, Auth Protocol 10, Sync Correctness 10, Type Fidelity 5/5, Dead Code 5/5.
- Soft spots: MCP Token Efficiency 7, Cache Freshness 5, Insight 7, Data Pipeline Integrity 7. None blocking.

## Sample output probe: 4/7 passed
- PASS: all 4 `analytics --type` variants (read the local store, work without a key).
- FAIL (expected, not a defect): `mailboxes idle`, `mailboxes failed`, `exports watch` returned typed exit 4 with `401 Invalid API key` because no `ZAPMAIL_API_KEY` was set during shipcheck. Each emitted the correct actionable hint. These verify in Phase 5 with a real key.

## Fixes applied this phase
- Array-body flags accept CSV (root-cause fix; was JSON-only), via `parseListFlag`.

## Ship recommendation: ship (pending Phase 5 live smoke with the user's key)
- All ship-threshold conditions met. No known functional bugs in shipping-scope features. The 3 live novel commands need a key to confirm real output, which is the Phase 5 gate.
