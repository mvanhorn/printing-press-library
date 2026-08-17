# Pexels CLI — Shipcheck Report

## Verdict: ship

## Shipcheck umbrella: PASS (7/7 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS (9 narrative commands resolved + full examples) |
| dogfood | PASS (auth MATCH, 0 dead flags/funcs, data pipeline GOOD) |
| workflow-verify | workflow-pass |
| apify-audit | PASS |
| verify-skill | PASS (all flag/command/positional checks + canonical-sections) |
| scorecard | PASS |

## Scorecard: 93/100 — Grade A
Strong: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Quality/Desc/Remote, Local Cache, Workflows, Insight (all 10/10).
Soft spots (polish targets): MCP Token Efficiency 7, Cache Freshness 5, Breadth 7, Vision 9, Agent Workflow 9, Type Fidelity 2/5. MCP surface: 9 tools (8 public, 1 auth-required), readiness full.

## Bugs found and fixed before ship (fix-before-ship, not deferred)
1. **sync 401 "Missing API key" on public endpoints.** Root cause: `determinePaginationDefaults()` emitted generic `limit=100`/offset defaults; Pexels rejects an unknown `limit` query param with HTTP 401 even on public read endpoints. Fixed to `per_page=80`/page pagination. Sync now pulls 160 records across 2 pages. (Generated-file edit; retro candidate — profiler should derive page/per_page from spec.)
2. **Framework `search` live path returned empty.** Root cause: `extractSearchResults()` checked generic wrapper keys (data/results/items) but Pexels wraps results under `photos`/`videos`/`media`. Added those keys. Default `search "nature"` now returns live results; offline `search --data-source local` returns synced results. (Generated-file edit; retro candidate — extraction should include spec response_path values.)

## Sample Output Probe: 5/6 (83%)
The one remaining flag — "Dedup + rate-aware bulk download: output does not contain any token from query 'mountain lake'" — is a **false positive** of the substring-relevance heuristic. `download` correctly returns downloaded-file metadata (id, photographer, file_path), which by design does not echo the search query. Independently verified working: downloads 2 valid JPEGs + sidecars, dedup re-run skips both, ledger persists. Flagged for Phase 4.85 adjudication.

## Independent behavioral verification (live)
- resolve 2014422 (target 1280×720) → large2x 1880×1300 (no upscale), correct attribution
- download "mountain lake" → 2 valid JPEGs + 2 .meta.json sidecars; re-run → 0 downloaded, 2 skipped (dedup)
- attribution export → SOURCES.md with real photographer credits + Pexels links
- quota forecast → remaining 24671, fits true
- sync photos → 160 records; analytics group-by photographer → real counts; offline search → synced matches
- All 15 command paths resolve; novel_features_check 6/6

## Before/after
- Scorecard: 92 → 93 (after Insight improvement)
- Sample probe: 4/6 → 5/6 (after sync + search fixes)
- verify pass: PASS throughout

## Ship recommendation: **ship** — all threshold conditions met, no known functional bugs in shipping-scope features. No stubs. No `## Known Gaps`.
