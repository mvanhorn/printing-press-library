# Unify CLI — Phase 5.5 Polish Result

Invoked the printing-press-polish skill autonomously after Phase 5 dogfood passed.

## Delta

| Metric | Before | After | Delta |
|---|---|---|---|
| Scorecard total | 76/100 | 73/100 | -3 (env-driven; quality up) |
| Verify pass rate | 100% | 100% | +0% |
| Dogfood verdict | WARN | PASS | fixed |
| Tools-audit pending | 1 thin | 0 | -1 |
| Go vet | clean | clean | 0 |
| Output review warnings | 4 | resolved | -4 |

## Fixes applied

1. Removed 4 dead helper functions from `internal/cli/helpers.go` (paginatedGet, extractPaginatedItems, rawAtPath, extractResponseData) — the spec has no list endpoints, so these were emitted but never called.
2. Fixed `--agent`/`--compact` projection bug in `compactListFields`: expanded the allow-list to cover Unify novel-command output keys (object_name, domain, snippet, match_key, match_value, exists, error, record_id, etc.) and added a non-empty-row fallback so commands no longer return `[{}]`.
3. Fixed flagship SQL examples in `research.json`, `README.md`, and `SKILL.md`: `record_<object>` tables only have `id`/`created_at`/`updated_at`/`attrs` columns, so all examples now use `json_extract(attrs, $.<attribute>)` instead of selecting nonexistent direct columns.
4. Replaced literal `<record-id>` placeholder in trace examples with a real-looking UUID so the example actually runs when copy-pasted.
5. Re-classified `import-csv` as `mcp:read-only=false`: `--execute` mode runs upserts against the live API, so agents should see the permission prompt.
6. Expanded `watch list` Short to name what it returns ("object, match key, match value, age, optionally filtered by object") — fixes the only tools-audit thin-short finding.
7. Added README troubleshooting note explaining the json_extract pattern for the per-object record tables.

## Skipped findings (not actionable in polish)

- `sync_correctness` scored 0/10 because the scorecard runs `sync` against the live API without a credential and gets HTTP 401. Test-environment issue, not a CLI defect.
- `live_api_verification` N/A and `live_check` sql/schema-diff failures: same root cause (no API key + no prior schema snapshot in the scorers ephemeral DB). The sql probe also substring-matches the literal example query string, which the long json_extract example doesnt produce in result body.
- `mcp_token_efficiency` 7, `mcp_remote_transport` 5, `mcp_tool_design` 5, `mcp_surface_strategy` N/A, `cache_freshness` 0: structural — addressing requires spec edits (transport: [stdio, http], orchestration: code, intents) and a regen, out of scope for polish on a mid-pipeline CLI.
- Cobra Shorts on `auth`/`feedback`/`profile` subcommands lack `mcp:read-only` annotations: those parents are framework-skipped from MCP exposure (cobratree/classify.go), so annotations would have no effect.

## Ship recommendation: `ship-with-gaps`

Verify 100%, dogfood PASS, all flagship novel features verified live against the users real Unify workspace, and the only "gap" is the env-dependent scorecard floor (73 vs 75). Further polish would re-tread the same ground. Polish-skills `further_polish_recommended: no` confirms this.
