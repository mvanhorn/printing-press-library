# Toodledo CLI — Phase 5.5 Polish

| Metric | Before | After |
|---|---|---|
| Scorecard | 93 | **96 (Grade A)** |
| Verify | 100% | 100% |
| Dogfood | PASS | PASS |
| Tools-audit pending | 9 | **0** |
| MCP Desc Quality | 5/10 | **10/10** |
| MCP Remote Transport | 5/10 | **10/10** |
| Gosec (hand-authored) | 1 | **0** |

## Fixes applied (all durable — source/spec/override, not generated files)
1. Enriched 3 thin Cobra Shorts on the hand-authored task-write commands (complete/reopen/delete) → parameter-aware for the MCP agent surface.
2. `mcp-descriptions.json` overrides for 6 thin typed-endpoint descriptions (contexts/folders/goals/locations list+delete), grounded in spec entity schemas; applied via mcp-sync.
3. `mcp.transport: [stdio, http]` added to spec.yaml (34 endpoints is just above the 30-endpoint auto-http threshold) → remote HTTP transport for cloud agents; default stays stdio (additive, no compat break).
4. Narrow `// #nosec G304` on capture.go `os.Open(--file)` (the path is the user's own flag value).

## Deliberately not chased (would game the scorer)
- MCP Token Efficiency 7/10 — terser descriptions would undo the quality work.
- MCP Tool Design 5/10 — intents/code-orchestration are calibrated for 50+ endpoint APIs; Toodledo's GTD workflows are already exposed as rich novel commands.
- Cache Freshness 5/10 — no-auto-refresh is the *correct* design given the hard 100-call-per-token quota; `sync-cost` provides quota awareness instead.

## Retro candidates surfaced
- 35 gosec findings in generator-emitted files (store/client/auth/config/cobratree/cache).
- Cache Freshness scorer's quota-aware branch only recognizes a `lookup_log` table or `quota.go` helper — could also recognize a `sync-cost`-style preview as quota-aware design.

**ship_recommendation: ship · further_polish_recommended: no · remaining_issues: none**
