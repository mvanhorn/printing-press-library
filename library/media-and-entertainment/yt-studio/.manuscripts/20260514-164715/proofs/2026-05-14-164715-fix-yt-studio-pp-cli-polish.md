# yt-studio-pp-cli Polish Report

## Verdict
**ship** (polish: further_polish_recommended=no)

## Delta

| Metric | Before | After | Delta |
|---|---|---|---|
| Scorecard | 85/100 | 86/100 | +1 |
| Verify | 100% | 100% | 0 |
| Dogfood | WARN | PASS | improved |
| Tools-audit | 0 pending | 0 pending | 0 |
| PII-audit | 0 pending | 0 pending | 0 |
| Go vet | 0 | 0 | 0 |

## Fixes applied (7)

1. Renamed `channels info` → `channels get` (verb naming violation; updated
   file, function, Use, Example, MCP tool, README, SKILL).
2. Removed dead function `replacePathParam` from `internal/cli/helpers.go`.
3. Moved `title-patterns` SQL into `ytstore.OwnChannelTitleCTRs` to satisfy
   the `reimplementation_check` store-access carve-out.
4. Added `cliutil.AdaptiveLimiter` (Wait / OnSuccess / OnRateLimit) to
   `ytanalytics.Client`; 429 now routes through `cliutil.RateLimitError`.
5. Same rate-limiter pattern added to `ytstudio.Client`.
6. Added missing `Example:` blocks to `quota` and `watchlist remove`.
7. Disabled HTML escaping in `printJSONFiltered` (`SetEscapeHTML(false)`) so
   `<`, `>`, `&` in JSON prose fields render literally.

## Skipped (out-of-scope or structural)

- 10 verify-matrix commands score 2/3 (help + dry-run pass, full exec
  needs runtime data not available in mock mode). The verdict-level
  verify is PASS at 100%; these are environmental, not defects.
- MCP scorecard dims (token_efficiency 7, remote_transport 5, tool_design 5)
  require `mcp.transport=[stdio,http]` + `mcp.orchestration=code` enrichment
  in the spec. Out of scope for mid-pipeline polish; flagged for retro.
- Workflows (6/10), Cache Freshness (5/10), Type Fidelity (3/5) are
  structural floors below max — none closeable without spec changes.

## Remaining issues

None.

## Note

Polish ran in mid-pipeline (Skill-tool) mode, so its publish-validate gate
was skipped (`publish_validate_before/after: skipped (mid-pipeline)`).
The main skill owns publish at Phase 6.
