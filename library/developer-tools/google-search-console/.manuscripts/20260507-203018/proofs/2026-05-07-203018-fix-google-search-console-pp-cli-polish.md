# Phase 5.5 Polish — google-search-console-pp-cli

## Polish skill output

| | Before | After | Delta |
|---|---|---|---|
| Scorecard | 74/100 | 71/100 | -3 (scorer brittleness, see below) |
| Verify pass rate | 100% | 100% | 0 |
| Dogfood verdict | WARN | PASS | +1 tier |
| Workflow-verify | pass | pass | 0 |
| Verify-skill errors | 0 | 0 | 0 |
| Tools-audit findings | 0 | 0 | 0 |
| Go vet issues | 1 | 0 | -1 |

Polish ship recommendation: **ship-with-gaps**. `further_polish_recommended: no` (remaining gaps are scorer-internal or structural, won't change with another pass).

## Polish-applied fixes

1. **Removed dead helper `paginatedGet`** from `internal/cli/helpers.go` (flagged by dogfood; not called).
2. **Removed dead helper `extractResponseData`** from `internal/cli/helpers.go` (flagged by dogfood; not called).
3. **Real bug fix in `sitemap-health` SQL placeholder count** (`internal/cli/novel_inspection.go`): the dynamic `where` clause was interpolated 3× but only 2 `cf.site` values were bound; would have produced a SQL placeholder mismatch when invoked with `--site`. Now binds 3. Also resolves go vet's "append with no values" warning.
4. **Added `## Known Gaps` section to README** documenting cache-freshness, search-appearance dimension, and orchestration scope (mandated for ship-with-gaps).

## Additional fixes applied here (Phase 4.8/4.9 errors polish skipped)

The agentic SKILL/README review surfaced 4 documentation errors polish did not address. All were 1-file edits:

5. **README Authentication section rewritten.** Removed fictional `--credentials` flag, `GOOGLE_APPLICATION_CREDENTIALS` handling, and `gcloud auth print-access-token` shell-out integration — none of which exist on the binary. Replaced with the real two-path story: `auth login` interactive OAuth2 flow, OR set `GOOGLE_SEARCH_CONSOLE_OAUTH2C` from any minted access token (with `gcloud auth print-access-token` documented as a token *source*, not as an integration).
6. **README Quick Start example fixed.** `google-search-console-pp-cli sites list --json` → `google-search-console-pp-cli webmasters sites-list --json` (the spec-derived path).
7. **README Output Formats examples retargeted.** Demo invocations of `url-inspection` (which has required flags) → `webmasters sites-list` (no required flags); the demos now run cleanly without contrived setup.
8. **SKILL.md `--select` demo retargeted** likewise.

The auth narrative in `research.json` was also corrected so future regenerations render the right content.

## Skipped findings (won't fix)

- **Scorer regression `sync_correctness` 5→0 and `output_modes` 10→9.** Scorer grades on token presence (`strings.Contains(content, "paginatedGet")`, `"page_fetch"`); removing genuinely dead code rewarded by dogfood (`dead_code` 3→5) is *punished* by these dimensions. Scorer contradicts itself. Net real quality is higher (less dead code, real bug fixed) despite the lower numerical score. Retro candidate.
- **MCP token efficiency 7, MCP remote transport 5, MCP tool design 5.** Fix is `spec.yaml` `mcp:` block edits + regenerate; polish runs in working dir post-generation. For an 11-endpoint API, default endpoint-mirror is the right shape anyway.
- **Vision 4/10, Workflows 4/10, Insight 4/10, Cache Freshness 0/10, Type Fidelity 3/5.** Structural — would require new features (orchestration intents, cache freshness flag, more domain commands). Out of polish scope.
- **12 verify commands at score 2.** All are `kind=read` novel commands (cannibalize, decay, momentum, etc.) that need a populated store to "execute"; live-check ran them with the empty store and all 11 returned valid empty envelopes. Environmental, not a defect.
- **`sitemap-health` returns `"site": ""` when no `--site` flag.** Intentional cross-property design.

## Final verdict

**ship-with-gaps.** All gates pass:

- ✅ shipcheck umbrella exits 0 (5/5 legs PASS post-polish + post-doc-fixes)
- ✅ verify 100%, 0 critical
- ✅ dogfood PASS (was WARN before polish)
- ✅ verify-skill 0 errors
- ✅ workflow-verify pass
- ✅ scorecard ≥ 65 (got 71)
- ✅ Known Gaps documented in README
- ✅ All 11 novel features sample-probe PASS (Phase 4.85)
- ✅ Documentation accuracy errors fixed (auth narrative, Quick Start, Output Formats)

Promotion to library is appropriate.
