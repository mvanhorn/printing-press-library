# Polish — youtube-pp-cli

## Result

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Scorecard | 82/100 | 83/100 | +1 |
| Verify | 100% | 100% | +0 |
| Dogfood | WARN | PASS | improved |
| Tools-audit | 0 pending | 0 pending | +0 |
| go vet | 0 | 0 | +0 |

## Fixes applied
- Removed dead helper `extractResponseData` from `internal/cli/helpers.go` (flagged as defined-but-never-called).
- Added missing `Example:` to `transcripts langs` command (10/10 commands now have examples).
- Added unit tests for `internal/quota` package: `Cost`, `All`, `Endpoints`, `HashKey`, `isWriteMethod`, `DailyDefault` — 9 test functions covering the pure-logic surface.
- Added unit tests for `internal/transcripts` package: `PlainText` covering nil/empty, VTT cue stripping, inline-tag stripping, deduplication; `EnsureYTDLP` error-shape.

## Skipped findings (with reasoning)
- `live_check` failures on `corpus search`, `topic crossover`, `trending diff`, `subscriptions sweep` — environmental (empty store, no OAuth bearer in mock mode, scorer's tokenizer artifact). Not CLI defects.
- `mcp_token_efficiency=4`, `mcp_surface_strategy=2`, `mcp_remote_transport=5`, `mcp_tool_design=5` — spec already declared `transport: [stdio, http]`, `endpoint_tools: hidden`, `orchestration: code`. Generator did not propagate the orchestration mode through to the emitted MCP server. **Retro candidate** (machine-level fix, not polish).
- `mcp_description_quality=0` and `live_api_verification=0` — both in `unscored_dimensions`; require an `mcp-descriptions.json` override and a live API key respectively. Out of polish scope.
- `type_fidelity=3` — generator-driven on a 76-endpoint Google Discovery spec; not polish-level.
- `path_validity 0/0` in dogfood — structural (10 global query-params filtered from all 76 endpoints), by-design.
- `data_pipeline_integrity=7` — search uses direct FTS5 SQL by design (cross-corpus union). Working as intended.

## Verdict
- `ship_recommendation: ship`
- `further_polish_recommended: no`
- Reasoning: dogfood went WARN→PASS, all hard gates clean (verify 100%, verify-skill 0 findings, workflow-verify pass, tools-audit 0 findings, go vet clean). Remaining low-scoring dims are structural/heuristic gaps that respond to generator-level work, not another polish pass.
