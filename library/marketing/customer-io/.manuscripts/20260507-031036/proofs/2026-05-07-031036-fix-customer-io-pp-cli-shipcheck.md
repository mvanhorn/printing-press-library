# Phase 4 shipcheck — customer-io-pp-cli

## Umbrella verdict: **PASS** (5/5 legs)

| Leg | Result | Elapsed |
|---|---|---|
| dogfood | PASS | 1.4 s |
| verify | PASS | 2.2 s |
| workflow-verify | PASS | 13 ms |
| verify-skill | PASS | 180 ms |
| scorecard | PASS — 88/100 Grade A | 62 ms |

## Top blockers found

| # | Finding | Class | Action |
|---|---|---|---|
| 1 | `sync` crashed at first-run with `duplicate column name: data` (broadcasts table had `data JSON NOT NULL` and `data TEXT` declared twice in the same `CREATE TABLE`, generated from the spec's `broadcasts.trigger.body.data` parameter) | Generator bug (retro candidate) — column-promotion logic should skip names that collide with the always-present `data` JSON column | Removed the duplicate `data TEXT` line from `internal/store/store.go`; sync now runs cleanly. |
| 2 | `mcp_token_efficiency` scored 4/10 | Scorer reads the spec's typed-endpoint count (45) instead of the runtime cobratree-walked tool count (19, post-Cloudflare-pattern enrichment). Confirmed via tools/list — runtime exposes the orchestration pair + 8 novel + 6 framework = 19 tools, not 45. | Logged for retro — the scorer should read the runtime MCP tool surface, not the spec count, when `mcp.orchestration: code` is set with `endpoint_tools: hidden`. |
| 3 | `cache_freshness 5/10`, `auth_protocol 8/10`, `data_pipeline_integrity 7/10`, `type_fidelity 3/5` | Minor polish items; well within the 65-threshold floor. | Defer to Phase 5.5 polish. |

## Per-leg details

### dogfood (PASS)
- Path validity: 0/0 (internal-yaml; paths validated at parse time)
- Auth protocol: MATCH (Bearer prefix consistent between spec and client)
- Dead flags: 0
- Dead functions: 0
- Examples: 10/10 commands have examples
- Novel features: **8/8 survived** (every transcendence command from the absorb manifest is present and registered)
- MCP surface: PASS (mirrors Cobra tree at runtime)
- Sync run: now succeeds end-to-end after the duplicate-column fix.

### verify (PASS, 100% pass rate)
- 26/26 commands tested across HELP, DRY-RUN, and EXEC stages — all 3/3 PASS
- 0 critical failures
- Initial run reported "Data Pipeline: FAIL: sync crashed"; resolved by the duplicate-column fix.

### workflow-verify (PASS)
- No workflow manifest (`workflow_verify.yaml`) is committed — verdict is `workflow-pass` by virtue of the no-manifest skip path. Live workflow verification will be exercised in Phase 5 against the real account.

### verify-skill (PASS)
- All checks passed: flag-names, flag-commands, positional-args, unknown-command, canonical-sections.
- README contains all 5 standard sections; SKILL.md examples resolve to real CLI subcommand paths.

### scorecard (88/100 Grade A)
- Output modes / Auth / Error handling / Doctor / Agent Native / MCP Tool Design / MCP Surface Strategy / MCP Remote Transport / Local Cache / Breadth / Workflows / Insight / Path Validity / Sync Correctness / Dead Code → all 10/10 (or 5/5 for Domain Correctness).
- Token efficiency dimension undercounts the runtime MCP tool surface; documented above.

## Before/after

- Initial verify pass rate: 100 % (with sync crash flagged separately as Data Pipeline FAIL)
- Final verify pass rate: 100 %, sync runs clean
- Initial scorecard total: 88/100
- Final scorecard total: 88/100
- 5/5 legs PASS for both runs

## Final ship recommendation: **ship**

All ship-threshold conditions met:
- shipcheck umbrella exits 0
- verify verdict PASS (>80 % pass rate, 0 critical failures)
- dogfood passes wiring + path + dead-flag + dead-function checks
- workflow-verify is `workflow-pass` (will be re-checked live in Phase 5)
- verify-skill exits 0
- scorecard 88/100 (≥65 threshold)
- No known functional bugs in shipping-scope features (sync was fixed before declaring ship)

## Known gaps (none blocking)

- `mcp_token_efficiency` scorer mis-counts; runtime is 19 tools, well within agent-friendly territory.
- Track API (Site ID + API Key, separate host) is not in v1; documented in README.
- Reverse-ETL endpoints return 403 on Essentials accounts; documented in README troubleshooting.
