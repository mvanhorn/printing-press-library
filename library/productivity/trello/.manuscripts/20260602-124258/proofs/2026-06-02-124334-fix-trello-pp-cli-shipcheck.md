# Trello CLI Shipcheck

## Verdict: ship

## Shipcheck legs (6/6 PASS)
- verify: PASS
- validate-narrative: PASS
- dogfood: PASS (novel_features_check found==planned, 8/8)
- workflow-verify: PASS
- verify-skill: PASS (after fixing README `sync --full` backtick quoting)
- scorecard: PASS

## Scorecard: 97/100 Grade A
- Output Modes 10, Auth 10, Error Handling 10, README 10, Doctor 10, Agent Native 10
- MCP Remote Transport 10, MCP Tool Design 10, MCP Surface Strategy 10 (Cloudflare pattern, 324 endpoints)
- Local Cache 10, Breadth 10, Vision 10, Workflows 10, Insight 10
- Cache Freshness 5/10 (no pre-read auto-refresh; manual sync by design)
- Terminal UX 9, MCP Quality 8, Agent Workflow 9

## Novel features built (8/8, all verified against seeded data)
overdue, workload, velocity, cycletime, bottleneck, checklist-progress, blocked, churn

## Fixes applied
- spec collision: search/types resources -> x-pp-resource overrides (search_resource, types_resource)
- removed duplicate novel `stale` (framework `stale` covers it)
- renamed novel `checklists` -> `checklist-progress` (collision with generated resource command)
- README `sync --full` quoting fixed for verify-skill

## Sample probe: 8/8 live command samples pass (100%)
