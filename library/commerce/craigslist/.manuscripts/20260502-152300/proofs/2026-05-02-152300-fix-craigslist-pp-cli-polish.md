# craigslist-pp-cli Polish Report (Phase 5.5)

## Delta

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Scorecard | 85/100 | 86/100 | +1 |
| Verify pass-rate | 100% | 100% | 0 |
| Tools-audit pending | 5 | 0 | -5 |
| Live-check pass | 4/10 | 5/10 | +1 |
| Dogfood verdict | WARN | WARN | (false positive — see retro candidates) |
| go vet | 0 | 0 | 0 |

## Fixes applied

1. **FTS5 single-quote crash in `median` and `reposts`** — queries with embedded apostrophes (e.g. `"men's"`, `"iphone 15'"`) crashed with `SQL logic error: fts5: syntax error near "'"`. Added a `quoteFTS` helper in `internal/cli/cl_storehelp.go` that wraps user input as an FTS5 phrase, sanitizing apostrophes and reserved characters. Threaded through both call sites. Verified live-check median + reposts now exit 0.

2. **5 tools-audit thin-short fixes** — improved MCP-tool descriptions to add parameter context:
   - `areas list`: now describes filters and 707-area scope
   - `categories list`: now describes 178-category scope and filters
   - `favorite list`: adds local-table context + ordering
   - `favorite remove`: adds positional + table context
   - `watch list`: adds query/sites/category context

## Skipped findings (false positives or out-of-scope)

- **Dogfood `reimplementation_check` WARN on `search --negate` and `cities heat`.** Both files import `internal/source/craigslist` and call `c.Search(...)` / `c.FreshListingsWindow(...)`. The generator's `siblingInternalImportRe` pattern only matches flat `internal/<name>` packages, missing the nested `internal/source/<name>` shape. Generator-side regex gap, not a printed-CLI defect. Logged as retro candidate.
- **`mcp_token_efficiency: 0/10`** — structural scorer issue. The synthetic spec has one resource (`postings`) with one typed endpoint (`search`), so the typed-MCP-tool surface has 1 tool. The token-efficiency dimension penalizes low tool counts; for a single-endpoint spec it can't lift. Not addressable in polish.
- **Live-check token-presence misses on `median`/`reposts`/`since`** — the live-check probe looks for query tokens in stdout, but after the FTS fix these correctly return `[]` for unmatched queries (no apostrophes-in-test-data) and `since` returns URL+pid pairs that don't echo "24h". Live-check probe limitation.
- **Live-check failure on Smart watch example** — example chains 3 commands with `&&`; live-check harness invokes only the first segment then complains about `--seed-only` not being a `watch save` flag. Harness limitation.
- **Live-check timeout on `cities heat`** — command takes ~13s to fan out across 20 cities; default 10s timeout. Genuine API work, not a CLI defect.
- **`mcp:read-only: true` on locally-mutating watch + favorite commands** — author left an explicit comment in `watch.go` documenting the deliberate choice ("read-only is true because no external state is touched"). Polish does not aggressively re-decide documented intent.

## Retro candidates (for `/printing-press-retro`)

1. Generator: `siblingInternalImportRe` regex in `internal/pipeline/internal_packages.go` should match `internal/source/<name>` (nested) in addition to flat `internal/<name>`. Causes false-positive reimplementation findings on every multi-source-style CLI.
2. Generator: there's no `// pp:client-call` directive analogous to `// pp:novel-static-reference` for the case where the heuristic misses a real API call. Spec authors should be able to escape-hatch when the regex doesn't recognize the call shape.
3. Skill prose: editing SKILL.md or README.md narrative directly silently reverts on next shipcheck (dogfood resyncs from research.json). Should be loud about "edit research.json, not the rendered files" — surprised this run.

## Ship recommendation: **ship**

Phase 4 verdict held: shipcheck PASS (5/5 legs). Phase 5 acceptance gate PASS. Polish delta positive (+1 scorecard, -5 tools-audit pending, FTS bug fixed). No remaining ship blockers.
