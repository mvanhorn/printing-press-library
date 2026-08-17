# Browserbase CLI — Polish Pass

## Polish Results for browserbase-pp-cli

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Scorecard | 92/100 | 93/100 | +1 |
| Verify | 100% (85/85) | 100% (85/85) | 0 |
| Live matrix | exercised | exercised | - |
| Tools-audit | 5 pending | 0 pending (2 accepted) | -5 |
| gosec (hand-authored) | 3 (G301/G306/G304) | 0 (G304 accepted as intended) | -3 |

## Fixes applied
- Created `mcp-descriptions.json` with rich descriptions for 5 thin generated MCP tools (certificates_list/delete, contexts_delete, extensions_delete, projects_list); applied via mcp-sync.
- Accepted 2 brief-but-precise thin-short findings (list on platform_client.go, teach.go) in the tools-polish ledger.
- Fixed G301/G306 file-permission findings in fetch_batch.go checkpoint write (0o700/0o600).
- Documented G304 (user-supplied --file path) as intended behavior.

## Skipped findings
- G304 in fetch_batch.go: user-supplied URL-list file path is the documented feature, not a vulnerability.
- G101/G404/G201/G202/G204/G302 in generated files (platform/gate.go, jobs.go, store.go, auth.go, client.go, learn/journal.go): generator-emitted code, retro candidates.
- Dogfood WARN "usage trend advertised as usage but registered as projects usage": benign resolver ambiguity between root `usage trend` (novel) and generated `projects usage`; both commands work and serve different purposes. 7/7 novel features found.

## Remaining issues
- None blocking. The dogfood depth-mismatch WARN on `usage trend` is documented above.

## Ship recommendation
ship (mid-pipeline; publish-validate deferred to main SKILL Phase 6)
