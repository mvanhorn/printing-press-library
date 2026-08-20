# StackAdapt — Phase 5.5 Polish

Ship recommendation: ship. further_polish_recommended: no.

| Metric | Before | After |
|---|---|---|
| Scorecard | 78/100 | 78/100 |
| Verify | 100% | 100% |
| go vet | 0 | 0 |
| gosec (hand-authored) | 3 | 0 |
| tools-audit pending | 6 | 1 (version, accepted) |
| PII | 0 | 0 |

Fixes applied:
- store.go: db-dir perms 0o755→0o750 (gosec G301); handled db.Close() error on schema-create failure (G104)
- sagraphql/client.go: explicitly discarded resp.Body.Close() return (G104)
- root.go: replaced truncated/garbled root Short/Long header with clean authored headline
- stackadapt_read.go: enriched 5 thin MCP list-tool Shorts (grounded in actual queries/flags)
- gofmt across tree

Skipped (retro candidates / structural, not polish-fixable):
- dogfood "novel features reimplemented / no client call" — heuristic false positive; the 4 transcendence cmds make live GraphQL calls via runQuery→sagraphql.Client.Query; GraphQL CLI doesn't use generated client.do()/store helpers
- 11 dead funcs + allowPartialFailure flag + 21 gosec findings — all in generated DO-NOT-EDIT files; /printing-press-retro candidates
- scorecard vision/cache-freshness/type-fidelity/data-pipeline-integrity — structural for a single-endpoint read-only GraphQL API
