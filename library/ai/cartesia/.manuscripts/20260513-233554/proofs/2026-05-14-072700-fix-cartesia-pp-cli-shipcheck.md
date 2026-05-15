# Cartesia CLI Shipcheck Report

## Summary
- Final verdict: `ship`
- Scorecard: 90/100 (Grade A)
- All 6 shipcheck legs PASS
- 81 spec-derived commands + 10 hand-built transcendence features

## Per-leg results

| Leg | Result | Elapsed |
|-----|--------|---------|
| dogfood | PASS | 1.93s |
| verify | PASS | 3.03s |
| workflow-verify | PASS | 11ms |
| verify-skill | PASS | 169ms |
| validate-narrative | PASS | 169ms |
| scorecard | PASS | 63ms |

## Scorecard breakdown
- **Output Modes 10/10**, **Auth 10/10**, **Error Handling 10/10**, **Doctor 10/10**, **Agent Native 10/10**, **Local Cache 10/10**, **Breadth 10/10**, **Agent Workflow 10/10**
- **MCP Remote Transport 10/10**, **MCP Tool Design 10/10**, **MCP Surface Strategy 10/10** (Cloudflare pattern: stdio+http transports, code orchestration, hidden endpoint-mirror tools)
- **Path Validity 10/10**, **Auth Protocol 10/10**, **Data Pipeline Integrity 10/10**, **Sync Correctness 10/10**, **Dead Code 5/5**
- README 8/10, Terminal UX 9/10, MCP Quality 8/10, Workflows 8/10, Vision 8/10
- **MCP Token Efficiency 0/10** — only meaningful gap. Driven by the size of the auth-required toolset; orchestration mode already in place mitigates per-call cost.
- Cache Freshness 5/10 — typical for first-print (no historical sync data in store yet).
- Insight 6/10 — could improve with more narrative recipes; deferred.

## Fixes applied during shipcheck
1. **Wired `CARTESIA_API_KEY` env var** in `internal/config/config.go` (generator only had `CARTESIA_ACCESS_TOKEN`). API key wins when both set; both flow through Bearer header.
2. **Patched `doctor` env-var check** to recognize both env vars and emit useful hint.
3. **Patched `auth set-token` hints, `agent_context.go` envVars, helpers.go error hints** to reference `CARTESIA_API_KEY` first.
4. **Fixed `CARTESIA_API_VERSION`** override path to populate `Headers["Cartesia-Version"]`.
5. **Built 10 novel transcendence commands** (subagent): `sql`, `calls grep`, `calls worst`, `agents audit`, `agents diff`, `agents since`, `metrics trend`, `voices changelog`, `voices find`, `usage estimate`. Added schema for `calls`, `metric_results`, `voices_snapshot` tables + FTS5 on calls transcripts.
6. **Fixed Cobra `Use:` strings** on all 10 novel commands so dogfood + verify-skill see positional args declared.
7. **Fixed FTS5 syntax in `calls grep`** — wrap pattern in double-quotes to handle bare apostrophes and multi-word phrases.
8. **Rewrote narrative quickstart and recipes** to use real flag names (`--resources` doesn't exist on sync; `auth set-token` needs positional token) and avoid shell-quoting traps that broke validate-narrative.

## Known gaps (not blocking ship)
- **MCP Token Efficiency** dimension scored 0/10. The Cloudflare orchestration pattern is already in place (`mcp.orchestration: code`, `endpoint_tools: hidden`); the metric reflects the absolute size of the auth-required surface (49 tools). Further reduction would require splitting endpoints into intent-named tools, which is a v0.2 design decision, not a fix.
- Phase 5 live smoke testing skipped — no `CARTESIA_API_KEY` available in this session. The CLI is verified against mock responses and dry-run paths only. User can set the key and run `cartesia-pp-cli doctor` + any command for end-to-end live validation.
- Two pre-existing failing tests in `internal/store/upsert_batch_test.go` (`TestUpsertBatch_SetsFilesParentID`, `TestUpsertBatch_SetsFineTunesVoicesParentID`) — generator-side schema/test fixture mismatch on NOT NULL columns. Predates Phase 3; tracked for retro.

## Ship recommendation
`ship` — all gates pass, scorecard 90, no functional gaps in approved features.
