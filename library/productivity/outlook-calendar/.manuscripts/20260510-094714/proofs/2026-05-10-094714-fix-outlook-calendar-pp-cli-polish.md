# Phase 5.5 Polish — outlook-calendar-pp-cli

## Delta

|  | Before | After | Delta |
|--|--------|-------|-------|
| Scorecard | 79/100 | 84/100 | **+5** |
| Verify | 100% | 100% | 0 |
| Dogfood | FAIL | PASS | fixed |
| Publish-validate | FAIL | PASS | fixed |
| Tools-audit | 0 pending | 0 pending | 0 |
| Go vet | 0 | 0 | 0 |

## Fixes applied
1. Replaced `applyAuthFormat("Bearer {token}", ...)` with a `bearerPrefix = "Bearer "` const in `internal/config/config.go`; removed dead `applyAuthFormat` helper. Fixed dogfood auth-protocol detector and lifted **Auth Protocol 3/10 → 8/10**.
2. Ran `printing-press mcp-sync` to generate the missing `tools-manifest.json` (publish-validate's MCP-package-metadata gate required it).
3. Added `printer: "brennaman"` field to `.printing-press.json` (resolved from GitHub noreply email); publish-validate manifest gate now passes.
4. Copied `phase5-acceptance.json` into `<cli>/.manuscripts/<run-id>/proofs/` (publish-validate's Phase 5 gate looked there).
5. Authored rich `mcp-descriptions.json` overrides for `calendars_delete` and `categories_list`; re-ran mcp-sync. **MCP Desc Quality 7/10 → 10/10**.

## Skipped (with rationale)
- **type_fidelity 1/5** — heuristic awards points for `MarkFlagRequired` count and `StringVar` id-flags on alphabetical-first 10 cli/*.go files. This CLI uses positional args (a legitimate Cobra pattern); adding scaffolding flags purely to lift the score would degrade UX. Logged for retro: heuristic does not credit positional-arg patterns.
- **mcp_remote_transport 5/10, mcp_token_efficiency 7/10, mcp_tool_design 5/10** — all are spec-edit + regenerate fixes (`mcp.transport: [stdio, http]`, `mcp.endpoint_tools: hidden`, `mcp.orchestration: code`). Polish runs mid-pipeline and does not regenerate; future regen will pick these up.

## Ship recommendation: **ship**
Further polish recommended: no. All hard gates green; surviving sub-max scorecard dims are structural or spec-regen-only.
