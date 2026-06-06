# Atlas Obscura CLI — Shipcheck

## Verdict: ship

## Shipcheck umbrella: PASS (6/6 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative (--strict --full-examples) | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 86/100 — Grade A
- Output Modes 10, Auth 10, Error Handling 10, Terminal UX 9, README 10, Doctor 10,
  Agent Native 10, MCP Quality 9, MCP Desc 10, MCP Remote Transport 10, Local Cache 10,
  Workflows 10, Agent Workflow 9.
- Weaker (polish targets): MCP Token Efficiency 7, Cache Freshness 5, Breadth 7, Insight 7,
  Data Pipeline Integrity 7, Type Fidelity 2/5.

## Live Sample Output Probe: 7/7 (100%)
Fixed three behavioral issues found on first pass:
1. gaps/near/route geocoding: Open-Meteo doesn't parse "City, State" → added candidate
   fallback ("Portland, Oregon" → "Portland"). Verified gaps "Portland, Oregon" works.
2. visited mark: added slug+id to output envelope.
3. export: empty/nonexistent trip now returns exit 0 with an empty envelope (read-empty is
   not an error), instead of notFoundErr.

## MCP surface
- 3 typed endpoint tools (places_get, categories_places, destinations_places) + runtime
  Cobra-tree mirror exposing all hand-authored commands (search, near, show, route, trip,
  visited, gaps, cluster, surprise, export). readiness: full. transport: stdio + http.

## Behavioral correctness (flagship + approved features)
All 7 novel features + 4 headline commands live-verified returning correct, non-empty output.

## Known gaps: none blocking. Heuristic score + no structured hours documented honestly.
