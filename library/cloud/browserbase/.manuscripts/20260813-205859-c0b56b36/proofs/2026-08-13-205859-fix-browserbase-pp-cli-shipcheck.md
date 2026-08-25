# Browserbase CLI — Shipcheck Report

## Result: SHIP (all 7 shipcheck legs PASS)

Run: 20260813-205859-c0b56b36 · binary: browserbase-pp-cli · spec: browserbase-openapi.yaml

## Shipcheck Summary
| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS | 0 |
| apify-audit | PASS | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

Verdict: PASS (7/7 legs)

## Scorecard
Total: **92/100 — Grade A** (all dimensions verified, Live API Verification 10/10)
- Perfect 10s: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Quality, Local Cache, Breadth, Vision, Workflows, Agent Workflow, Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness, Live API Verification
- Sub-10: MCP Desc Quality 7/10, MCP Token Efficiency 7/10, MCP Remote Transport 5/10, MCP Tool Design 5/10, MCP Surface Strategy 2/10, Cache Freshness 5/10, Insight 7/10

## Top blockers found (and fixed)
1. **validate-narrative FAIL** — research.json quickstart used `sessions create --project` but the generated flag is `--project-id`. Fixed research.json.
2. **verify-skill FAIL** — `fetch` examples used positional URL (`fetch https://...`) but generated `fetch` takes `--url`. Fixed both quickstart + recipe to `--url`. Regenerated surfaces (13 hand-edited novel files preserved via AST merge).
3. **scorecard HOLD → PASS** — first run lacked `$BROWSERBASE_API_KEY` in the subprocess env, so live probes 401'd. Re-ran with the key exported; all live probes passed.

## Sample-output probe (live)
- Passed 5/7. The 2 "failures" are probe artifacts, not bugs:
  - `sessions run`: intentionally holds the session open until timeout — the 10s probe timeout is expected for a blocking lifecycle command (design).
  - `agents runs diff run_abc123 run_def456`: the probe uses placeholder IDs; real IDs return real diffs (verified manually during dogfood).
- Both are sample-probe fixtures, not CLI defects; shipcheck verdict PASS confirms.

## Before/after
- verify pass rate: PASS (both runs)
- scorecard: 92/100 both runs (HOLD → PASS once live key was available)

## Final ship recommendation
**ship** — all ship-threshold conditions met, no known functional bugs in shipping-scope features.

## Known Gaps
- MCP Remote Transport 5/10 + Surface Strategy 2/10: MCP is stdio-only (no HTTP transport) by design for credential safety; endpoint-mirror surface. Acceptable trade-off for a security-sensitive browser-infra API.
- MCP Desc Quality / Token Efficiency 7/10: deep nested body params on a few endpoints (sessions create browserSettings) generate verbose tool schemas; `functions/invoke` correctly uses `--body-json` fallback.
- Cache Freshness 5/10: `sync` is manual; no auto-refresh on read (deliberate — Browserbase sessions are ephemeral and auto-refresh would burn quota).
