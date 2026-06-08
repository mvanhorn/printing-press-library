# NinjaOne CLI Shipcheck

## Umbrella verdict: PASS (6/6 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 96/100 — Grade A
- Strong: Output Modes, Auth, Error Handling, README, Doctor, Agent Native, MCP Remote/Tool/Surface, Breadth, Vision, Workflows, Agent Workflow (all 10/10).
- Weaker: Insight 4/10, Cache Freshness 5/10 (intentional — cache disabled for this rate-limited API), MCP Quality 8/10, Terminal UX 9/10.
- Domain Correctness: Path Validity, Auth Protocol, Data Pipeline, Sync all 10/10; Type Fidelity 5/5; Dead Code 5/5.

## Live sample probe: 2/8 passed — NO-CREDENTIALS ARTIFACT
All 6 "failures" are `HTTP 400 missing_header` — the novel commands correctly call the live API and surface a clean error when no client credentials are present in the shipcheck env. Not correctness bugs. Real behavioral validation deferred to Phase 5 dogfood with credentials.

## Ship recommendation: ship (pending Phase 5 live dogfood with creds)
