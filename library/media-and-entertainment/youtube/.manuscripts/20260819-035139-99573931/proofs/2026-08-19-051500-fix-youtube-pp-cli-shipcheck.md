# Shipcheck proof — youtube reprint 20260819-035139

## Umbrella results (final rerun, dead env key stripped)
| leg | result |
|---|---|
| verify | PASS (auto-fix loop clean) |
| validate-narrative (strict, full examples) | PASS |
| dogfood | PASS (novel_features_check 10/10; wiring clean) |
| workflow-verify | workflow-pass (no manifest) |
| apify-audit | pass (n/a) |
| verify-skill | PASS incl. canonical-sections |
| scorecard | 96/100 grade A after fixes; HOLD only on live_api_verification (clears in Phase 5 live dogfood) |

## Live sample probe: 10/10 PASS (100%) with the working stored key exported

## Blockers found and fixed this phase
1. Spec carried dangling `Oauth2` security references from Google's full OpenAPI conversion → stripped per-operation security from all 12 GETs (API-key auth is global). Scorecard unblocked (was hard error).
2. Dead generated helper `successfulNoop` (framework emission, unreferenced) → removed; dead_code 4→5/5. RETRO CANDIDATE: generator emits an unused helper.
3. error_handling 8→10: HTTP 409 branch now names the conflict ("already exists") in helpers.go.
4. data_pipeline_integrity 7→10: domain wrapper `Store.SearchYoutube` (new durable file internal/store/youtube_search.go) + 3 call sites.
5. mcp_token_efficiency 4→7: trimmed 16 oversized framework Long strings (teach 2.9k, learnings_candidates 2.4k, sync 2.1k chars …) + tightened all 10 novel Longs to their redirect paragraphs. RETRO CANDIDATE: framework ships help text that alone blows the token budget. Remaining weight = spec-derived endpoint flag descriptions → left for polish.
6. Two thin Shorts (platform_client list, teach learnings list) → ≥60 chars.
7. Live sampler initially failed 7/10: root causes (a) sampler env had no usable key — config.toml key is ALSO dead; the working key lives in credentials.toml (data dir) and verified HTTP 200; (b) examples for `workspace use`/`auth keys use` referenced entities that don't exist in a fresh environment → examples now self-sufficient (list forms); (c) intermittent SIGBUS crashes ONLY inside the sampler (roaming across commands, never reproducible directly, incl. 9-process concurrency tests on fresh and existing databanks). Hardened EnsureAnalystSchema with BEGIN IMMEDIATE + existence fast-path anyway. RETRO CANDIDATE: sampler binary staging/refresh appears to race its own concurrent probe execution (macOS mmap invalidation signature).

## Before/after
- Scorecard: 89 (error state) → 91 → 95 → 96/100 grade A.
- Verify pass rate: PASS both rounds; dogfood verdict WARN→PASS at umbrella level (remaining WARN: framework `sync` is a no-op because the API has no bulk list endpoints — by design; the analyst data path is `backfill`/`monitor`, which the docs advertise).

## Ship recommendation: ship (pending Phase 4.8/4.85/4.9/4.95 review gates + Phase 5 live dogfood)
