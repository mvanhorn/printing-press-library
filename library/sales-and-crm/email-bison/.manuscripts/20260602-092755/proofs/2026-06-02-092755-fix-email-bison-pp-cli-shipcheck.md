# Email Bison CLI — Shipcheck

## Final verdict: ship

All six shipcheck legs pass; scorecard 89/100 (Grade A); live sample probe 7/7 (100%). No known functional bugs in shipping-scope features. Novel features behaviorally verified against a seeded local fixture (live smoke skipped — user declined the API key).

## Leg results (final)
| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS (no manifest) | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

## Scorecard 89/100 (Grade A)
- 10/10: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, Local Cache, Breadth, Vision, Workflows, Insight, Path Validity, Auth Protocol, Sync Correctness.
- 9/10: MCP Quality, Agent Workflow. 7/10: MCP Token Efficiency, MCP Tool Design, MCP Surface Strategy, Data Pipeline Integrity. 5/10: MCP Desc Quality, MCP Remote Transport, Cache Freshness. 4/5 Type Fidelity, 5/5 Dead Code.
- Live API Verification: N/A (no key; omitted from denominator).

## Blockers found and fixed
1. **verify-skill FAIL → PASS**: README/SKILL "Launch a campaign" recipe used `campaigns resume 6`, but the generated command nests the op as `campaigns resume campaign <id>`. Fixed the recipe in research.json + SKILL.md + README.md to `campaigns resume campaign 6 --dry-run`. Re-ran: clean.
2. **Command-tree collisions (fixed in Phase 3)**: duplicate `replies` stub parent and orphan `senders` stub parent removed; novel subcommands rewired under the real `replies` and `sender-emails` parents.

## Behavioral correctness (fixture-verified)
Seeded a representative fixture into the local store; every novel feature computed correctly:
- `campaigns headroom`: cap 100, sent_today 1, headroom 99, under_cap.
- `sender-emails health`: disconnected sender flagged (1 bounce, healthy=false); healthy sender clean.
- `replies interested --since 7d`: returned the interested reply only.
- `replies triage`: returned the uncategorized unread inbox reply only.
- `leads stale --days 7`: returned only the lead with an old send and no reply (excluded the freshly-sent lead).
- `campaigns variants 6`: reply_rate 0.10, interested_rate 0.04.
- `campaigns preflight 6`: all four presence checks pass; caught the missing `{PRODUCT}` merge tag; ready=false.

## Before/after
- verify pass rate: PASS throughout (mock mode).
- scorecard: 89/100 both runs (stable).
- verify-skill: FAIL (2 errors) -> PASS.

## Known gaps
- No live API verification (user declined key). Warmup endpoints excluded (undocumented paths). MCP description/transport dims are mid-range — candidates for the polish pass.
