# Roadside America — Polish (Phase 5.5)

**Ship recommendation: `ship`** · further polish: no.

| Metric | Before | After |
|--------|--------|-------|
| Scorecard | 89/100 | **92/100 (Grade A)** |
| Verify | 100% | 100% |
| go vet | 0 | 0 |
| gosec (hand-authored) | 1 | **0** |
| tools-audit | 1 pending | **0** |
| MCP Desc Quality | 3/10 | **10/10** |

## Fixes applied
1. **gosec G104 (hand-authored)** — unhandled `tw.Flush()` error in `internal/cli/roadside_shared.go` now checked/propagated (the one hand-authored security finding).
2. **Output-review format bug** — malformed distance strings (`"- Location Approximate - \n (~26 mi. away"`) normalized to clean `~26 mi. away` via `normalizeDistanceLabel` in `internal/roadside/parse.go`; verified live on `trip` (0/261 malformed, was 2/261). Added `TestNormalizeDistanceLabel`.
3. **MCP descriptions** — enriched `raw_detail`/`raw_by-state`/`raw_nearby` via `mcp-descriptions.json` + mcp-sync; MCP Desc Quality 3→10, MCP Quality 7→9.

## Deferred / retro candidates (generator-owned, can't fix in polish)
- 35 gosec findings in generator-emitted DO-NOT-EDIT files (file perms 0644→0600, `math/rand`→crypto in random/stats/trip, parameterized SQL in store helpers).
- dogfood WARN: generated no-op `sync` stub left unregistered after replacement by hand-authored `newRoadsideSyncCmd` (its helpers are consumed by other generated files, so the stub can't be deleted).
- novel-feature "top picks" wording in generated `compare.go` is an alphabetical sample, not ranked (RoadsideAmerica exposes no notability score).
- Structural scorecard dims (Cache Freshness, Breadth, Vision, Token Efficiency) calibrated for larger APIs; no non-gaming fix for a single-purpose scraper.

All retro candidates noted for `/printing-press-retro`.
