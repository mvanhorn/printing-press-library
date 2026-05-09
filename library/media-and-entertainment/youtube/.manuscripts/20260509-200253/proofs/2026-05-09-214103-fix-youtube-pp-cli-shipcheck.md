# Shipcheck — youtube-pp-cli

## Verdict: `ship`

## Per-leg results

| Leg | Result | Detail |
|-----|--------|--------|
| dogfood | PASS | 22/22 root commands; 100% pass rate |
| verify | PASS | 100% |
| workflow-verify | PASS | No workflow manifest declared (acceptable for v1) |
| verify-skill | PASS | 0 errors after fixing `--times` IntVar binding, flattening `topic crossover`, and editing the stale recipe |
| validate-narrative | PASS | 10/10 narrative commands resolve, all full examples succeed under `PRINTING_PRESS_VERIFY=1` |
| scorecard | PASS | 82/100 — Grade A |

## Scorecard breakdown
- Output Modes 10/10
- Auth 10/10
- Error Handling 10/10
- Terminal UX 8/10
- README 8/10
- Doctor 10/10
- Agent Native 10/10
- MCP Quality 10/10
- MCP Token Efficiency 4/10 (gap: spec's `mcp.orchestration: code` block did not flow through the generator)
- MCP Remote Transport 5/10 (gap: stdio-only, would benefit from `[stdio, http]`)
- MCP Tool Design 5/10
- MCP Surface Strategy 2/10 (gap: 76 endpoint-mirror tools without code orchestration; same root cause as MCP Token Efficiency)
- Local Cache 10/10, Cache Freshness 5/10, Breadth 10/10, Vision 6/10, Workflows 10/10, Insight 7/10, Agent Workflow 9/10
- Domain: Path Validity 10/10, Auth Protocol 9/10, Data Pipeline Integrity 7/10, Sync Correctness 10/10, Type Fidelity 3/5, Dead Code 4/5

## Sample Output Probe (live samples)
- 4/8 passed. The 4 failures fall into 3 buckets, all expected:
  - Trending diff: needs >= 2 snapshots in the local store. No snapshots captured during shipcheck (no API key in mock mode).
  - Subscriptions sweep: OAuth-gated. No OAuth bearer in mock mode → honest "oauth required" error.
  - Cross-corpus FTS / topic crossover: empty store → no hits to match query tokens.

  None indicate broken code; they reflect the absence of populated data during mock-mode shipcheck.

## Top blockers found and fixed mid-shipcheck

| Blocker | Fix | File |
|---------|-----|------|
| `quota plan --times` declared via plain `Flags().Int` (verify-skill couldn't find declaration) | Convert to `Flags().IntVar(&times, ...)` | `internal/cli/transcend_quota.go` |
| `topic crossover --regions` declared on parent → flag inheritance failed | Flatten command, declare on leaf | `internal/cli/transcend_trends.go` |
| Stale recipe in SKILL.md / README.md from earlier research.json | Edit recipe text to match real CLI surface | `SKILL.md`, `README.md`, `research.json` |
| FTS5 syntax errors on apostrophes in user queries | Wrap user input in `ftsQuote()` (FTS5 phrase quotes) | `transcend_corpus.go`, `transcend_trends.go` |
| Auth wiring: generator picked OAuth-only (no API key support) | Hand-wire `APIKey` field in Config; add `?key=` query in client.go; teach doctor about the OR-relationship | `internal/config/config.go`, `internal/client/client.go`, `internal/cli/doctor.go` |

## Before/after
| Metric | Before | After |
|--------|--------|-------|
| verify-skill | 8 errors | 0 errors |
| build | green | green |
| scorecard | 82/100 | 82/100 (unchanged — gaps require Cloudflare-orchestration regen, deferred to polish) |
| narrative validation | 1 missing + 2 failed examples | 10/10 ok |

## Final ship recommendation: `ship`

All ship-threshold conditions met. Phase 5 live dogfood was skipped at user request (no API key available). The CLI is verified against mocks and the scorecard's structural quality bar.

## Known gaps (eligible for polish or v0.2)
- MCP orchestration mode (`mcp.orchestration: code`) didn't flow into the generated MCP server — would lift `mcp_token_efficiency`/`mcp_surface_strategy` from 4/2 to 10/10.
- OAuth login flow scope-set unverified end-to-end (`auth login` exists but real Google OAuth flow not exercised in this run).
- Comments aren't synced into `yt_comments` (no dedicated `sync-comments` command); `corpus search` over comments only finds rows hand-inserted via `youtube comments-list` if the user opts in.
