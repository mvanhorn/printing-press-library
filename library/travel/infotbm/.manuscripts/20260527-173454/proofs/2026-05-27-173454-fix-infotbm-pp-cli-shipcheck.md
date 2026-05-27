# Shipcheck Report: infotbm-pp-cli

## Summary

- **Verify**: PASS — 100% (28/28 passed, 0 critical)
- **Validate-Narrative**: PASS — 10/10 commands resolved, full examples passed
- **Dogfood**: PASS — 8/8 novel features survived, 10/10 examples, 0 dead flags/functions
- **Workflow-Verify**: PASS (no workflow manifest)
- **Verify-Skill**: PASS — all flag-name, flag-command, positional-arg, unknown-command checks passed
- **Scorecard**: 89/100 Grade A

## Fix Loop 1

### Blockers Found
1. `--stop-ref` flag referenced in research.json, README.md, SKILL.md — actual flag is `--stop-id`
2. `arrivals` command referenced in SKILL.md Hand-written Extensions and README.md bash recipe — command was never created (uses `realtime stop` instead)

### Fixes Applied
1. research.json: `--stop-ref` → `--stop-id` in quickstart and recipes
2. research.json: `stops search` → `stops --name` in troubleshoot
3. README.md: `--stop-ref` → `--stop-id` (2 occurrences)
4. README.md: `arrivals bordeaux:...` → `realtime stop --stop-id bordeaux:...`
5. SKILL.md: removed `arrivals` from Hand-written Extensions section
6. SKILL.md: `--stop-ref` → `--stop-id` (1 occurrence)

### Before/After
- Verify: PASS → PASS (no change)
- Validate-Narrative: FAIL (2 failed examples) → PASS (10/10 ok)
- Verify-Skill: FAIL (5 errors) → PASS (0 errors)
- Scorecard: 89 → 89 (no change)

## Scorecard Breakdown

| Dimension | Score |
|-----------|-------|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 10/10 |
| README | 10/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 8/10 |
| MCP Desc Quality | 5/10 |
| MCP Token Efficiency | 7/10 |
| MCP Remote Transport | 10/10 |
| Local Cache | 10/10 |
| Cache Freshness | 5/10 |
| Breadth | 9/10 |
| Vision | 9/10 |
| Workflows | 10/10 |
| Insight | 4/10 |
| Agent Workflow | 9/10 |

## Sample Output Probe Notes

Failures are expected without API key and synced database:
- Ghost Service Detector, Timetable Change Diff: need local SQLite database (sync first)
- Alert Impact Filter: needs API key (HTTP 401)
- Line Stop Sequence, Headway Frequency Report: need API data (empty response without key)

These will be tested in Phase 5 live dogfood.

## Final Verdict: ship
