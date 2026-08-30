Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

## Phase 3 complete (2026-08-06)
Manifest transcendence rows: 7 planned, 7 built. All resolve via --help with proper Usage lines; dogfood novel_features_check: planned=7 found=7 missing=[].
- Built: acwr, pb, benchmark, rtp, diff (2 required positionals + pp:happy-args), heatmap (renamed from periods to avoid generated-resource collision; registered via registerNovelCommand hook), report (markdown/CSV export, monotony+strain folded in)
- Shared helpers: novel_common.go (fetchStatsRows POST /stats, activity/athlete indexes, resolveActivityArg incl. last/prev/date forms, riskZone, median)
- Strategy: pp:data-source live on all 7 (stats engine data fetched per-invocation; local mirror not required)
- Deferred/accepted WARNs: dead generated helper collectionItemsForOutput (generator template issue — retro candidate); naming verb info→get on generated customer_info.go (kept: mirrors the API's /customer/info path)
