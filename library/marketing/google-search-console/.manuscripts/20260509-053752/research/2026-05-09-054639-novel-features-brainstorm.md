# Novel Features Brainstorm — Google Search Console (audit trail)

> Subagent output from Phase 1.5 Step 1.5c.5. Persisted for retro/dogfood debugging per the skill's audit-trail rule. The Customer model and Killed candidates are not rendered into the manifest but are preserved here.

## Customer model

**Persona 1 — Riya, in-house SEO lead at a 200-SKU ecommerce brand.**

*Today (without this CLI):* She lives in the GSC web UI and a tab full of Google Sheets. Every Monday morning she filters Performance to last-7-days, exports CSV, vlookups it against last week's CSV, and pastes the deltas into a Slack post for the merch team. When something tanks she pivots to URL Inspection and clicks each problem URL one at a time.

*Weekly ritual:* Monday WoW report, mid-week "what queries are on page 2" pass, Friday sitemap-warnings sweep, end-of-month YoY deck for her director. Anything older than 16 months is just gone — she has screenshots.

*Frustration:* The 16-month window, the 1,000-row UI cap, the lack of period comparison anywhere except by hand, and the fact that "which pages cannibalize each other" requires a pivot table she rebuilds from scratch every time.

**Persona 2 — Devon, SEO automation engineer at an agency managing 30+ client properties.**

*Today (without this CLI):* He has a Python script using Josh Carty's `searchconsole` lib that pulls per-client CSVs into S3 nightly, and a second script that emails clients when impressions drop >20% DoD. Cross-property "top queries across all 30 sites" requires a loop he wrote, with hand-rolled rate-limit backoff.

*Weekly ritual:* Friday deck-prep — runs 30 queries, normalizes, ranks, screenshots into client decks. Investigates "why is property X's traffic off a cliff" 2-3 times a week.

*Frustration:* Every client onboarding means re-pointing his scripts, no shared cache means duplicate API spend, and the 16-month window means he can't answer "is this March-2024 normal or not?" without his own backups.

**Persona 3 — Mira, content marketer who owns publishing but not infrastructure.**

*Today (without this CLI):* She uses the GSC UI for one property. She does NOT write SQL, does NOT run cron jobs. She wants "what should I update this week" answered in a single command. Her dev gave her the OAuth token and told her "run this when you want the report."

*Weekly ritual:* Picks 2-3 articles to refresh based on whatever GSC's Performance tab makes obvious. Tuesday is publish day, Thursday is refresh day.

*Frustration:* Quick Wins (page-2 with high impressions) is buried — she has to filter position 11-20 + sort by impressions + eyeball CTR every single time. Same for "which of my old posts are dying."

**Persona 4 — an AI agent doing an SEO audit on demand.**

*Today (without this CLI):* Calls one of three competing MCPs. Each covers a different slice. None persist data, so a follow-up question ("compare to last month") means re-fetching everything. NDJSON output is inconsistent across tools.

*Weekly ritual:* Spawned ad-hoc by a human ("audit example.com"). Needs to chain commands, read JSON, decide next call. Wants determinism and explicit JSON shapes.

*Frustration:* No single binary covers the full surface. Has to fall back to "shell out and parse" because the MCPs each lock features behind their own opinions.

## Candidates (pre-cut)

| # | Name | Command | One-line | Persona | Source |
|---|------|---------|----------|---------|--------|
| C1 | Quick Wins | `quick-wins [site] [--position 8-20] [--min-imps 100] [--min-ctr 0]` | Surface page-2 queries with high impressions and low CTR from local cache | Riya, Mira | (b) ahonn, AminForou competitor gap |
| C2 | Cannibalization | `cannibalization [site] [--min-imps 50] [--top N]` | Pages competing for the same query, ranked by combined impressions | Riya, Devon | (b) AminForou first-class tool |
| C3 | Period compare | `compare [site] --period 28d --vs prev-period [--dim query]` | Period-over-period delta on clicks/imps/CTR/position with significance | Riya, Devon | (b) AminForou competitor gap |
| C4 | Cliff detector | `cliff [site] [--metric clicks] [--threshold -25%]` | Day-over-day cliffs in clicks/imps from local snapshots, with signature hints | Devon, Riya | (b) brief Priority 2 |
| C5 | Cross-property roll-up | `roll-up [--metric clicks] [--top 50] [--group-by query]` | Aggregate top queries/pages across all verified properties | Devon | (c) cross-entity local query |
| C6 | Coverage drift | `coverage-drift [--site X] [--days 30]` | URL inspection state changes over time from snapshot history | Riya, Devon | (c) cross-entity local query |
| C7 | Historical (>16mo) | `historical [site] --start 2023-01-01 --end 2024-01-01 --dim query` | Query data older than the API's 16-month window from local cache | Devon, Riya | (b) brief Priority 2, transcendence base |
| C8 | CTR outliers | `outliers [site] [--metric ctr] [--top 50]` | Queries/pages with CTR anomalous for their position vs your own observed curve | Riya, Devon | (b) brief Priority 2 |
| C9 | Sitemap regression watcher | `sitemap-watch [site] [--since 7d]` | Diff sitemap state across snapshots, surface new errors/warnings/last-downloaded changes | Riya | (c) cross-entity local query |
| C10 | Decaying pages | `decaying [site] [--window 90d] [--min-imps 500]` | Pages with monotonic decline in clicks over a rolling window | Mira, Riya | (a) Mira's Thursday refresh ritual |
| C11 | Brand vs non-brand split | `brand-split [site] --brand-regex 'ozark|...'` | Slice clicks/imps into branded vs non-branded buckets via regex | Devon, Riya | (a) Devon agency-deck need |
| C12 | Indexing diff | `index-diff [site] --from 2026-04-01 --to 2026-05-01` | URLs that flipped indexed↔not-indexed between two snapshot dates | Riya, Devon | (c) cross-entity local query |
| C13 | Query intent buckets | `query-intent [site]` | Bucket queries into informational/commercial/branded heuristically | Mira | (b) content-marketer pattern |
| C14 | Competitor SERP overlay | `competitor-compare [site] --vs example.com` | Side-by-side rankings against a competitor domain | Devon | external |
| C15 | New queries arriving | `new-queries [site] [--since 28d] [--min-imps 50]` | Queries that started showing up in the last N days but didn't exist prior | Mira, Riya | (c) cross-entity local query |
| C16 | Query→Page map | `top-pages-for "query" [site]` | All pages that have ranked for a query, ordered by clicks | Devon, Riya | (a) Devon's "what page should I link to" workflow |

## Survivors and kills

### Forced answers on every survivor (Pass 3)

See SKILL output above — each survivor has weekly-use, wrapper-vs-leverage, transcendence-proof, and sibling-kill answers documented.

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Quick Wins | `quick-wins [site] [--position 8-20] [--min-imps 100] [--min-ctr 0.05]` | 9/10 | Local SQLite SELECT on search_analytics_rows WHERE position BETWEEN 8 AND 20 AND impressions >= min, ordered by impressions*(target_ctr - actual_ctr) | ahonn/mcp-server-gsc has it as flagship; AminForou exposes it; Mira's persona pain is explicit |
| 2 | Cannibalization | `cannibalization [site] [--min-imps 50] [--top N] [--include-singletons]` | 9/10 | GROUP BY query in local store, HAVING COUNT(DISTINCT page) > 1, severity = SUM(impressions) | AminForou first-class MCP tool; brief workflow #4; absorbed C16 query→page map via `--include-singletons` |
| 3 | Period compare | `compare [site] --period 28d --vs prev-period [--dim query] [--top 50]` | 9/10 | Two date-window aggregates from local store joined on dimension; emits Δclicks, Δimps, ΔCTR pp, Δposition | brief workflow #2 ("most-requested aggregation API doesn't provide"); AminForou compare_search_periods; Riya weekly ritual |
| 4 | Cliff detector | `cliff [site] [--metric clicks] [--threshold -25%] [--window 7d]` | 7/10 | Day-over-day delta from snapshots; flag drops below threshold; signature hints (sitemap-wipe, indexing drop) match against url_inspections + sitemaps deltas same day | brief Priority 2; Matt's 2026-03 sitemap-wipe diagnosis; Devon "cliff investigation" 2-3x/week |
| 5 | Cross-property roll-up | `roll-up [--metric clicks] [--top 50] [--group-by query] [--last 28d]` | 8/10 | Single SQL aggregate across all (siteUrl, ...) partitions in local store | brief workflow #7 cross-property; Devon agency persona; no competitor tool offers it |
| 6 | Coverage drift | `coverage-drift [site] [--field indexingState] [--days 30]` | 7/10 | Diff url_inspections snapshots over time on chosen field, surface URLs that flipped | brief Priority 2; absorbed C12 (index-diff) as `--field indexingState` flag; Devon-Riya pain |
| 7 | Historical (>16mo) | `historical [site] --start <date> --end <date> [--dim query]` | 8/10 | SELECT directly from local store on date range that predates the 16-month API window | brief Priority 2 explicitly; only feature category truly impossible via the API |
| 8 | CTR outliers | `outliers [site] [--metric ctr] [--top 50] [--sigma 2]` | 6/10 | Bucket rows by integer position, compute mean+stddev CTR per bucket from local corpus, flag rows where \|actual_ctr - bucket_mean\| > sigma * bucket_stddev | brief Priority 2; no competitor has it; needs corpus → genuine transcendence |
| 9 | Sitemap regression watcher | `sitemap-watch [site] [--since 7d]` | 7/10 | Diff sitemaps table snapshots over window; surface new errors/warnings, content-count drops, last-downloaded staleness | brief workflow #6 monitor sitemaps; Riya Friday ritual |
| 10 | Decaying pages | `decaying [site] [--window 90d] [--min-imps 500] [--top 50]` | 7/10 | Linear regression slope on per-page weekly clicks over window; rank by negative slope * total impressions | Mira persona pain explicit ("which old posts are dying"); content-refresh workflow universal; no competitor |
| 11 | New queries arriving | `new-queries [site] [--since 28d] [--min-imps 50] [--top 100]` | 7/10 | Local set difference: queries with impressions in last N days NOT in corpus before that window | Mira/Riya content-ideas workflow; impossible without retained history; brief Priority 2 implicit |

### Killed candidates

| Feature | Kill reason | Closest-surviving sibling |
|---------|-------------|---------------------------|
| C11 Brand vs non-brand split | Wrapper-thin: `query --filter query~regex` + sum already does it; not worth a dedicated command | `searchanalytics query --filter` (absorb manifest #13) |
| C12 Indexing diff (binary flip) | Special case of C6; C6 with `--field indexingState` covers it without a second command | C6 `coverage-drift --field indexingState` |
| C13 Query intent buckets | Either LLM-dependent or brittle regex pretending to be classification; no trustworthy mechanical version | Pipe `searchanalytics query --json` into `claude` |
| C14 Competitor SERP overlay | Requires external SERP API not in spec; external-service kill | None — out of scope |
| C16 Query→Page map | Heavy overlap with C2; absorbed into `cannibalization --include-singletons` | C2 `cannibalization --include-singletons` |
