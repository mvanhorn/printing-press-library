# Lemon Squeezy CLI — Shipcheck Report

## Umbrella Verdict: PASS (6/6 legs)

| Leg | Result | Elapsed |
|---|---|---|
| verify | PASS | 7.7s |
| validate-narrative | PASS | 158ms |
| dogfood | PASS | 1.0s |
| workflow-verify | PASS | 10ms |
| verify-skill | PASS | 763ms |
| scorecard | PASS | 752ms |

## Scorecard: 93/100 Grade A

- Output Modes / Auth / Error Handling / Terminal UX / README / Doctor / Agent Native / MCP Remote Transport / MCP Tool Design / MCP Surface Strategy / Local Cache / Breadth / Vision / Workflows: **10/10 each**
- MCP Quality: 8/10
- Cache Freshness: 5/10 (cache not yet enabled; deferred per skill guidance for stateful catalog CLI without auto-refresh path approval)
- Insight: 7/10
- Agent Workflow: 9/10
- Domain Correctness:
  - Path Validity 10/10, Auth Protocol 10/10, Data Pipeline Integrity 10/10, Sync Correctness 10/10
  - Type Fidelity 2/5 (api-evangelist spec is thin on schemas — known; out of scope to fix this run)
  - Dead Code 5/5

## Dogfood

- Novel Features: 8/8 survived (PASS)
- Path Validity: 4/4 valid (PASS)
- Auth Protocol: MATCH
- Dead Flags: 0
- Dead Functions: 0
- Examples: 10/10 commands have examples (PASS)

## Sample Output Probe (live live invocations against real LS API)

- Passed: 6/8 (75%)
- Failures (both NON-BLOCKING — correct behaviors that the probe heuristic flagged):
  1. **refund-cascade order_xyz123 --apply** → exit 5 with 404. The fixture order ID does not exist in the user's real LS account; the command correctly returned 404. **Fix in Polish**: change example to use `--dry-run` so the probe stays deterministic.
  2. **campaign-watch FOUNDING-1YR** → "no matching discount codes" output. The fixture discount codes do not exist in the user's real LS account yet (Early Access launch is upcoming). Command correctly reports no matches. **Fix in Polish**: relax the example to a generic invocation that doesn't depend on specific codes existing.

Neither failure is a flagship feature returning wrong output — both commands behave correctly for the inputs given. Recording for Polish.

## Verdict: ship

All ship-threshold conditions met:
- shipcheck umbrella exits 0
- verify PASS
- dogfood wiring and novel-feature checks PASS
- verify-skill exits 0
- scorecard 93/100 (well above 65 threshold)
- No flagship feature returns wrong/empty output (the two probe failures are example-data artifacts, not broken behavior)
