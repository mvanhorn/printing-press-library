# Pagliacci CLI Polish Report (Phase 5.5, 2026-05-03)

## Polish Results

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Scorecard | 82/100 | 83/100 | +1 |
| Verify | 100% | 100% | 0 |
| Dogfood | WARN | PASS | improved |
| Go vet | 0 | 0 | 0 |
| Tools-audit | 0 pending | 0 pending | 0 |

## Fixes applied

1. **`compactListFields` allow-list widened** (`internal/cli/helpers.go`) — added suffix matching (`_id`, `_name`, `_at`, `_url`, `_count`, `_total`, `_price`, `_status`, `_state`, `_type`) plus exact fields (`price`, `total`, `amount`, `quantity`, `count`, `value`, `currency`, `address`, `city`) and a small strip blocklist symmetric with `compactObjectFields`. Fixes the Phase 4.85 finding: `slices today --agent` was returning 92–93 entries of empty `{}` because the previous allow-list (camelCase only) stripped every snake_case domain field on `SliceRow`. Now `--agent` returns rich rows with `store_id`, `slice_name`, `price`. Same fix applies to any list-shape novel command emitting domain-specific snake_case JSON tags.

2. **Removed dead `extractResponseData` function** (`internal/cli/helpers.go`) — flagged by dogfood. Dead Code 4/5 → 5/5; dogfood WARN → PASS.

3. **README Cookbook section added** with 11 recipes covering `slices today`, `store tonight`, `address best-time`, `orders plan`, `menu half-half`, `rewards stack`, `orders reorder`, `orders summary`, `search`, `sync`, and jq-pipe patterns. Every flag verified against `--help`. README 8/10 → 10/10.

## Skipped findings (structural scorer mismatches, not CLI defects)

- **mcp_token_efficiency 0/10**: Code-orchestration CLI with `endpoint_tools: hidden` ships a thin `<api>_search`+`<api>_execute` pair (~1K tokens) plus 3 framework tools. The scorer evaluates `tools.go` (handler implementations) instead of recognizing the orchestration mode from the manifest, so it attributes ~695 tokens/tool to the framework tools. The printed surface IS thin — the score is wrong. Retro candidate: either generator emits the `<API>_MCP_SURFACE` env switch when `mcp.orchestration:code`, or scorer detects orchestration mode from the spec/manifest directly.
- **insight 4/10**: pizza-ordering domain has limited insight-prefix matches (analytics.go + orders summary qualify). Not gameable without scaffolding.
- **auth_protocol 5/10**: composed cookie auth from browser-sniff is intentionally capped at 5 by `browserSessionUnverified` gate. Expected.
- **type_fidelity 3/5**: flag descriptions average 4.12 words; threshold is >5. Concise descriptions are accurate; not worth rewriting 34 flags to game one point.
- **data_pipeline_integrity 7/10**: dogfood flags generic Search; FTS5 over synced data is the standard pattern.
- **workflows 8/10**: no workflow manifest emitted; minor.
- **4 live_check failures**: environmental — test workspace has no "home" address label (only "Most Recent") and no past order history. CLI error messages are accurate ("Available labels: Most Recent. Pass --label primary…"). Not CLI defects.

## Retro candidate

`mcp_token_efficiency` scoring is misleading for code-orchestration CLIs missing `<API>_MCP_SURFACE` env switching in main.go. Should be tracked for a printing-press improvement.

## Verdict

**ship**. All shipcheck legs pass, all behavioral fixes applied, scorecard 83/100 Grade A.
