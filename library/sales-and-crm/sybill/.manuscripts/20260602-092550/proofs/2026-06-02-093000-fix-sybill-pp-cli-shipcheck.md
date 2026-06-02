# Sybill CLI — Shipcheck

## Umbrella verdict: PASS (6/6 legs)
| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS (no manifest) | 0 |
| verify-skill | PASS (all checks + canonical sections) | 0 |
| scorecard | PASS | 0 |

## Scorecard: 92/100 — Grade A
Strong: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Quality, Local Cache, Workflows, Insight all 10/10. Domain Correctness: Path Validity, Auth Protocol, Data Pipeline, Sync Correctness all 10/10; Dead Code 5/5.

Weaker (not blockers):
- MCP Remote Transport 5/10, MCP Tool Design 5/10, MCP Desc 7, Token Efficiency 7 — 33-tool stdio endpoint-mirror surface. Lifting these needs spec regeneration (mcp.transport [stdio,http] + code orchestration), which would overwrite the hand-built novel commands. Left as-is; stdio MCP works for Claude Desktop.
- Cache Freshness 5/10, Type Fidelity 4/5 — minor.

## Sample Output Probe: 5/6
- 5 novel features sampled clean.
- crm-autofill --deal deal_123 → exit 4, HTTP 403 "Not authenticated". EXPECTED: --deal with a deal not in the local store triggers a live detail fetch, which needs SYBILL_API_KEY. No key was set during shipcheck. Not a code defect — the all-deals store path and the 6 behavioral acceptance tests confirm crm-autofill logic is correct. Error message is actionable.

## Behavioral verification (no API key needed)
internal/cli/novel_sybill_test.go + webhook_test.go: 7 tests, all PASS. Assert content (dark-deal membership, digest grouping + next-step extraction, crmAutofill diff values, account rollup joins, per-owner activity, pattern match counts + negative test, Svix signature valid/tamper/stale). Cross-entity join logic proven correct against a synthetic store.

## Fixes applied this phase
- None required; shipcheck was green on first run after Phase 3 build.
- Added webhook verify (absorbed feature #14) + test during build, before shipcheck.

## Ship recommendation: SHIP
All ship-threshold conditions met. The single probe miss is documented expected behavior (live fetch without a key), not a functional bug in shipping scope.
