# Acceptance Report — alphavantage-pp-cli

**Level:** Quick Check
**Tests:** 10/10 passed
**Live API calls burned:** ~4 (quote get, news sweep ×2 for FTS5 backfill, movers brief). Local-only commands and dry-runs add 0.

## Matrix results

| # | Test | Result | Notes |
|---|------|--------|-------|
| 1 | doctor --json | PASS | env var resolved, MARKET_STATUS reachable, schema_version=2 |
| 2 | quota status --json | PASS | returns used/max/remaining/recent_calls (used=43 — includes Phase 4 verify mock runs; not a bug) |
| 3 | quote get AAPL --json | PASS | live: AAPL $294.80, change +0.72% |
| 4 | news sweep --tickers AAPL --json | PASS | 50 articles, 156 ticker_sentiment rows persisted (the killer feature: per-ticker sentiment array preserved) |
| 5 | news timeline AAPL --days 30 --json | PASS | 2-day window, mean_sentiment 0.11 with 12 articles |
| 6 | news search "apple" --json | PASS after fix | initially 0 matches (FTS5 schema bug); fixed inline → 23 matches |
| 7 | news search "AAPL" --json | PASS | 12 matches |
| 8 | movers brief --side gainers --enrich sentiment --json | PASS | live TOP_GAINERS_LOSERS, top gainer TDIC +126.92% |
| 9 | watchlist add + sentiment --name us-core --json | PASS | AAPL added, mean_sentiment_7d 0.157 from 50 articles |
| 10 | screen --watchlist us-core --sentiment-min 0.0 --json | PASS | matched AAPL with sentiment_7d 0.157 |

## Fixes applied during dogfood (1)

### FTS5 contentless schema bug → fixed
- **Symptom:** `news search "apple"` returned 0 matches despite 50 articles persisted.
- **Cause:** `internal/store/store.go` declared `av_news_fts` with `content=''` which makes columns indexed-but-not-stored. The news_search.go query JOINs on `av_news_articles.url = fts.url` but fts.url was unreadable. FTS5 was indexing tokens correctly but the JOIN returned no rows.
- **Fix:** removed `content=''` from the FTS5 schema. URL stays UNINDEXED; title/summary/topics are both indexed and stored. Storage ~3x larger but the news corpus is small.
- **Verification:** dropped existing FTS5 table, ran sweep, manual backfill from av_news_articles → "apple" returns 23 matches, "AAPL" returns 12.

## Printing Press issues for retro

1. **Spec → root.go Short truncation at 150 chars.** Headlines longer than ~150 chars get truncated mid-sentence with `…` in root.go Short. The printing-press generator should either truncate at a word boundary or warn the author when the headline exceeds the limit.
2. **`apikey=` query-param auth detection.** The auth field uses `header: apikey` + `in: query` — the spec semantics make `header` overloaded (it's the query param name when in:query). Documenting this in spec-format.md would prevent confusion.
3. **CSV-shaped 200-with-error not detected by client middleware.** The client middleware catches JSON-shaped `{"Note":..., "Information":..., "Error Message":...}` soft-failures, but Alpha Vantage's CSV endpoints (EARNINGS_CALENDAR, IPO_CALENDAR, LISTING_STATUS) can return error messages as raw CSV text that the JSON-shape detector misses. Added a guard in `extractFirstEarningsRow` to catch this — but the generator could do better by routing CSV endpoints through a CSV-aware detector.
4. **`content=''` FTS5 schema pattern is a footgun for hand-authored novel commands.** The subagent's first cut declared contentless FTS5 to save storage. Joined queries silently return zero. The Printing Press scorecard's "Local Cache" dimension passes regardless of whether FTS5 actually works at query time. A live-search smoke test in the binary-owned matrix would catch this.

## Gate: PASS

All 10 matrix tests passed (one needed a hot fix during dogfood). The CLI is shippable:
- Auth flow works end-to-end with the Alpha Vantage free tier
- News sentiment data persists correctly with per-ticker_sentiment preserved (the killer feature)
- FTS5 search works against the persisted corpus
- Local-only aggregation (news timeline, watchlist sentiment, screen) reads correctly
- Live API endpoints (quote, movers brief, news sweep) return valid data
- Multi-call orchestration (briefing earnings) has improved error guards against CSV-shaped soft-failures
