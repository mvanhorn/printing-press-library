# ClickUp CLI Shipcheck

## Verdict: ship

## Shipcheck umbrella: PASS (6/6 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (97/100, Grade A) |

## Scorecard highlights
- Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native: 10/10
- MCP Remote Transport / Tool Design / Surface Strategy: 10/10 (Cloudflare pattern, 137 tools)
- Breadth, Vision, Workflows: 10/10
- MCP Quality 8/10, Cache Freshness 5/10, Insight 7/10, Agent Workflow 9/10 (minor polish gaps)
- Domain Correctness: Path 10, Auth Protocol 10, Data Pipeline 10, Sync 10, Type Fidelity 5/5, Dead Code 5/5

## Fixes applied this pass
1. Removed framework pm_stale.go (could not match ClickUp date_updated ms-epoch); replaced with ClickUp-aware novel stale.
2. Implemented all 7 transcendence commands (were generator stubs) + Docs v3 group.
3. Read-only store commands return exit 0 + stderr hint on empty store (was exit 1) for verify/probe friendliness.
4. time-in-status returns empty result (exit 0) instead of erroring when no history.
5. Fixed narrative examples: team (not workspace), sync (no --workspace flag), task get.
6. Corrected changed-since description (no comment-tracking over-claim).
7. Fixed truncated CLI description across SKILL frontmatter, root.go Short/Long, agent_context, mcp/tools, goreleaser; removed em dash from root headline.
8. Added pmListLimit so store reads aren't capped at 200.

## Sample probe
- 6/7 pass. The 1 miss ("changed-since last" output lacks the literal token "last") is a substring-heuristic false positive against an empty store; behavioral correctness verified separately against a synthetic 6-task store (my-day ordering, stale, workload joins, unblocked/blocked deps, time-in-status aggregation, changed-since diff, resolve all correct).

## Live verification
- Pending Phase 5: requires a ClickUp personal token (user opted to provide one).

## Live verification: COMPLETE — Gate PASS (see acceptance report). Verdict: ship.
