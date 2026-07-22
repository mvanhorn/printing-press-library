# Wahoo CLI — Polish Pass (Phase 5.5)

Invoked `printing-press-polish` (forked, mid-pipeline, STANDALONE_MODE=false).

## Delta
| Metric | Before | After | Δ |
|--------|--------|-------|---|
| Scorecard | 93/100 | **95/100 (A)** | +2 |
| MCP Desc Quality | 3/10 | **10/10** | +7 |
| Verify | 95.65% (PASS) | 95.65% (PASS) | 0 |
| Tools-audit pending | 10 | 0 | -10 |
| Gosec (hand-authored) | 6 | 0 | -6 |
| PII pending | 0 | 0 | 0 |

## Fixes applied
- Enriched all 28 MCP endpoint-tool descriptions in `mcp-descriptions.json` (spec-grounded: action, required/optional params, return shape, disambiguation) + `mcp-sync`. Cleared 10 thin-mcp-description findings; MCP Desc Quality 3→10.
- `backup.go`: dir perms 0o755→0o700 (G301), file perms 0o644→0o600 (G306) — private ride data; explicit `_ =` on best-effort cleanup (3× G104).
- `load.go`: propagate `tw.Flush()` error through `printLoadHuman` (G104).

Build/vet/tests re-confirmed green after edits.

## Skipped (structural / justified, non-blocking)
- MCP Token Efficiency 7/10 & MCP Tool Design 5/10: the collapse-to-search+execute pattern is calibrated for >50-endpoint APIs; at 28 tools discrete tools are better agent UX, and the 6 novel commands already serve as intent compositions over genuine CRUD.
- Cache Freshness 5/10: cache intentionally disabled (rate-limited API; staleness hints instead).
- gosec: 39 findings remain, all in generator-emitted DO-NOT-EDIT files (perms + unchecked cleanup errors) → Printing Press generator retro candidates, not hand-fixable.

## Result block
```
ship_recommendation: ship
further_polish_recommended: no
remaining_issues: (none)
```

## Retro candidate
Generator templates emit 0o755/0o644 file perms and unchecked cleanup-path errors across many generated files (config.go, cache.go, client.go, store.go, deliver.go), producing the 39 generated-file gosec findings. Fixing the templates clears them at source for every future CLI.
