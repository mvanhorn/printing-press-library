# digg-pp-cli shipcheck

## Final verdict: PASS (6/6 legs)

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| dogfood | PASS | 0 | 2.295s |
| verify | PASS | 0 | 24.426s |
| workflow-verify | PASS | 0 | 14ms |
| verify-skill | PASS | 0 | 209ms |
| validate-narrative | PASS | 0 | 1.042s |
| scorecard | PASS | 0 | 40ms |

## Scorecard: 80/100 — Grade A

| Dimension | Score |
|---|---|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 8/10 |
| README | 8/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 10/10 |
| MCP Token Efficiency | 4/10 |
| MCP Remote Transport | 5/10 |
| Local Cache | 10/10 |
| Cache Freshness | 5/10 |
| Breadth | 6/10 |
| Vision | 5/10 |
| Workflows | 4/10 |
| Insight | 2/10 |
| Agent Workflow | 9/10 |
| Path Validity | 10/10 |
| Data Pipeline Integrity | 7/10 |
| Sync Correctness | 10/10 |
| Type Fidelity | 3/5 |
| Dead Code | 5/5 |

Note: omitted from denominator (not applicable for this CLI): mcp_description_quality, mcp_tool_design, mcp_surface_strategy, auth_protocol, live_api_verification.

## Verify pass rate

100% (27/27 endpoints), 0 critical failures. Verify ran live against https://di.gg/api/trending/status and the HTML scrape paths.

## Two issues discovered and fixed in-session

### 1. `--alert` flag referenced in SKILL but missing on `watch`

`verify-skill` flagged the gap: SKILL.md mentioned `digg-pp-cli watch --alert 'rank.delta>=10'` but the command had only `--min-delta`. Fix: added `--alert string` flag with a `PreRunE` that parses `rank.delta>=N` shorthand into `--min-delta`.

### 2. Recipe examples used shell substitution

`validate-narrative` could not parse `digg-pp-cli evidence $(...)` shell-substitution recipes. Two recipes (Why is the top story, Cross-reference today's #1) failed because the `$(...)` syntax was treated as a literal arg containing `--limit`. Fix: replaced both recipes with concrete examples using a real clusterUrlId (`65idu2x5`) and added a sentence pointing the reader at `top --json --select clusterUrlId` to discover their own IDs.

Both fixes were 1-3 file edits each, applied in-session per the printing-press fix-before-ship rule.

## Gaps surfaced by scorecard (not blockers)

- `mcp_token_efficiency` 4/10 — MCP tool descriptions are verbose. Polish opportunity.
- `workflows` 4/10 — Could add a workflow_verify.yaml manifest for end-to-end agent flows.
- `insight` 2/10 — Could add more derived insights commands (e.g., `digg-pp-cli analyze rank-volatility`).
- `cache_freshness` 5/10 — Could opt into the machine-owned freshness contract.

These are quality polish opportunities, not correctness issues. Recommended for a v0.2 polish pass via `/printing-press-polish digg`.

## Behavioral correctness verification

Live runs confirmed:
- `sync` parses 5-6 clusters from real /ai HTML, stores 34+ pipeline events from /api/trending/status
- `top --json` returns structured cluster data including currentRank, delta, clusterUrlId, tldr, scoreComponents
- `search` FTS5 returns relevant matches (`search "buddhism"` finds the Buddhism cluster, `search "AI"` finds multiple)
- `evidence` returns scoreComponents JSON with impact/conversation/influence/evidence/impressions sub-scores
- `sentiment` returns posLast values when sync data has them
- `events --type cluster_detected` filters event stream correctly
- `crossref` with non-existent ID returns clean error message
- `doctor` reports auth=not-required, API=reachable, cache=ok
- `open` defaults to print-only (`would launch: ...`); `--launch` actually opens browser

## Final ship recommendation: `ship`

All ship-threshold conditions met:
- shipcheck umbrella exit 0
- verify 100% pass rate
- workflow-verify pass
- verify-skill pass
- scorecard 80/100 (above 65 threshold)
- All flagship features (events, replaced, evidence, crossref, authors top, etc.) produce correct, useful output
- No known functional bugs
- No mid-build downgrade from shipping-scope to stub

Read-only ethical scope locked in: no vote/bookmark/comment/post-as-X automation, identifying User-Agent, default 1 req/sec rate limit.
