# Shipcheck — scrape-creators-pp-cli
Date: 2026-04-22

## Build
- `go build ./cmd/scrape-creators-pp-cli/` → **BUILD OK**

## Dogfood (printing-press dogfood)
- Path Validity: 5/6 valid (PASS) — `/` root not in spec, expected
- Auth Protocol: MATCH
- Dead Flags: 0 (PASS)
- Dead Functions: 0 (PASS) — removed `usageErr`
- Data Pipeline: PARTIAL (generic search, 1 domain table)
- Examples: 8/10 (PASS)
- Novel Features: **8/8 survived (PASS)**
- **Verdict: PASS**

## Verify (printing-press verify)
- Help: 100% (33/33)
- Dry-run: 100% (33/33)
- Exec: partial (platform cmds need required params — expected in mock mode)
- Data Pipeline: **PASS** — sync crash fixed (duplicate sort_by + user_id columns)
- **Verdict: PASS**

## Scorecard
- **89/100 — Grade A**

## Bugs Fixed (fix-before-ship)
1. Duplicate `sort_by` column in tiktok table schema + INSERT → removed
2. Duplicate `user_id` column in instagram table schema + INSERT → removed
3. Sync migration crash → resolved by both fixes
4. Dead function `usageErr` → removed

## Live Dogfood (API key: env:SCRAPECREATORS_API_KEY)
All 8 transcendence commands tested live:
- `tiktok analyze --handle charlidamelio --limit 5` → ✅ ranked by ER
- `tiktok spikes --handle charlidamelio --threshold 1.5` → ✅ correct 0 spikes (all below threshold)
- `tiktok cadence --handle charlidamelio` → ✅ by-day + by-hour breakdown
- `tiktok compare --handle charlidamelio --handle addisonre` → ✅ side-by-side
- `tiktok track --handle charlidamelio` → ✅ snapshot saved
- `tiktok track --handle charlidamelio --history` → ✅ history returned
- `account budget` → ✅ 10094 credits remaining
- `search trends --hashtag dance` → ✅ 18 videos, top by play_count
