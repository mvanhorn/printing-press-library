# Agilix Dawn CLI — Shipcheck

## Verdict: ship

## Legs (7/7 PASS)
| leg | result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 92/100 — Grade A
Strong across Output Modes, Auth, README, Doctor, Agent Native, MCP, Local Cache, Workflows, Insight.
Lower dims: MCP Token Efficiency 7, Cache Freshness 5 (no upstream pre-read refresh — intentional,
this is a read-through wrapper), Type Fidelity 2/5 (Dawn returns deeply nested dynamic objects; typed
models cover the high-gravity scalar fields).

## Sample Output Probe: 6/6 (100%)

## Fixes applied
- verify-skill: `config` is promoted (single-endpoint) so the invocation is `config`, not `config get`.
  Fixed narrative quickstart (research.json) + README to match; re-verified PASS.
- scorecard sample probe: `course outline` md header now includes the concept id + status for traceability.

## Behavioral verification (live, against drivered.agilixdawn.com)
All 6 novel commands + generated endpoints run correctly against the live API:
- course stats: 34 sections / 392 instructions / 1223 interactions / 617 pts / 47.0h (matches raw API analysis)
- course tree / outline: real structure rendered/exported
- roster export, purchase reconcile, catalog diff: real joined/exported data
- concept list/get, config, user me, doctor: all green

## Known gaps
None blocking. progress/conversation endpoints return little for admin tokens (learner-scoped) — honest, documented.
