## Customer model

### Persona 1: Maya, in-house SEO lead at a mid-size SaaS

**Today (without this CLI):** Every Monday morning Maya opens the GSC web UI in three tabs (one per major property), changes the date range to "last 28 days," exports CSVs of "Queries" and "Pages," and pastes them into a Google Sheet that has VLOOKUPs against last week's pull. She also runs a Python notebook a contractor wrote two years ago that pulls the Search Analytics API for "queries that moved >5 positions." She cannot answer "which queries did this page used to rank for that it lost?" without writing fresh SQL against a one-off BigQuery export.

**Weekly ritual:** Pull last 7 + last 28 days of search analytics by query and by page for ~6 properties; flag movers; identify pages that need title/meta refreshes; produce a Slack post for the content team listing 5-10 specific URLs to edit.

**Frustration:** The web UI's "Compare" mode is two columns of numbers with no way to slice. The API gives her rows but no memory — every "what changed since last week" question requires keeping her own historical copy. Decay (a query slowly losing impressions) is invisible until it's already gone.

### Persona 2: Devin, technical SEO consultant juggling 18 client properties

**Today (without this CLI):** Devin runs a homegrown Node script nightly that hits `searchanalytics.query` for each client and dumps JSON to S3. For URL inspection he uses Bin-Huang's `inspect-batch` against a list of changed URLs from the client's sitemap diff. When a client emails "did our redesign hurt traffic?" he has to manually align his S3 dumps to the launch date and eyeball it. Cross-client rollups (e.g., "which clients had a Core Update hit?") require an ad-hoc Pandas notebook.

**Weekly ritual:** Per-client status email every Friday with traffic delta, top movers, indexing issues introduced this week, and any new sitemaps that failed to process. Cross-property roll-up internally to spot patterns across the book.

**Frustration:** Maintaining the homegrown sync. It breaks when Google rotates a token, the schema drift between his JSON dumps and the actual API isn't enforced, and he can't query across clients without another notebook. He'd pay to delete all of it.

### Persona 3: Priya, content-ops manager driving a 12,000-page editorial site

**Today (without this CLI):** Priya owns the content refresh queue. She lives in a spreadsheet exported from Screaming Frog joined by hand to a monthly GSC export. She wants to know: which pages were ranking on page 1 a month ago and have since slipped to page 2? Which pages have a query their title doesn't mention? Which pages were indexed in the last sync but aren't anymore? Today she answers none of these — she just refreshes "the oldest 20 pages" each week.

**Weekly ritual:** Pick the next 20 pages for the editorial refresh queue based on opportunity signal; verify last week's refreshes are re-indexed; report deindexings.

**Frustration:** No memory of yesterday's index state. When a page silently drops out of the index she finds out weeks later from a sales rep, not from her own tooling.

### Persona 4: Aki, AI agent operating SEO autonomously for a small e-com brand

**Today (without this CLI):** Aki is a Claude/agent runtime instructed to "improve organic traffic." It currently has a generic HTTP MCP that wraps `searchanalytics.query` and `urlInspection.inspect`. Every reasoning loop re-fetches the same data from the API because there's no persistent surface; quotas burn fast. The agent can't reliably answer "is this query new this week?" without rebuilding state in its context window each turn.

**Weekly ritual:** Nightly: pull yesterday's data, decide which product pages need title rewrites or new internal links, propose patches via a content-management MCP. Weekly: report what changed.

**Frustration:** No durable state between runs. Forced re-queries waste quota and hit rate limits. Wants a single tool that returns "what's new since my last visit" deterministically.

## Candidates (pre-cut)

(Full list of C1-C16 with persona, source label, and inline kill/keep verdicts; see Pass 3 force-answers for the cuts.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Query decay watch | `decay --window 12w --min-loss 100` | 8/10 | Linear-fit slope per `(site_url, query)` over `search_analytics_rows` daily history; ranks by absolute click loss. | Maya, Aki |
| 2 | Keyword cannibalization | `cannibalize --top 20 --min-impressions 100` | 9/10 | `GROUP BY (site_url, query) HAVING COUNT(DISTINCT page) > 1`; reports impression split and CTR drag. | Maya, Priya |
| 3 | Page momentum | `momentum --window 7d --vs 28d` | 7/10 | Compares 7-day rolling click sum per page against trailing 28-day baseline; emits rising and collapsing pages. | Maya, Devin, Priya |
| 4 | Coverage drift | `coverage-drift --since last-sync` | 8/10 | Diffs successive `url_inspections` snapshots on `coverageState`, `lastCrawlTime`, `googleCanonical`; flags state flips. | Priya, Devin, Aki |
| 5 | Opportunity with baseline | `opportunity --new-since 14d` | 8/10 | Today's quick-wins joined to prior daily snapshots to compute the date each page entered the zone. | Maya, Priya, Aki |
| 6 | Territorial shift | `territory --by country,device` | 6/10 | Cross-time pivot on `(query, country, device)` showing share-of-impression deltas. | Devin, Maya |
| 7 | Search-appearance breakdown | `appearance --window 28d --vs prior` | 6/10 | Stored snapshots of the searchAppearance dimension; reports gained/lost rich-result types per page. | Maya, Devin |
| 8 | New (and lost) queries | `new-queries --window 7d --min-impressions 50 [--lost]` | 8/10 | Anti-join: `LEFT JOIN prior_window WHERE prior IS NULL`; `--lost` inverts. | Aki, Maya |
| 9 | Sitemap health | `sitemap-health --regressed` | 6/10 | Joins current `sitemaps` snapshot against prior snapshot; reports deltas in errors/warnings/indexed-vs-submitted. | Devin, Priya |
| 10 | Cross-property mover board | `book --window 7d --top 25` | 9/10 | UNION across all verified `sites` of (page, query) click deltas; sorts by absolute change with per-site rollup row. | Devin |
| 11 | Indexing-issue triage | `triage --by impact` | 8/10 | Joins non-INDEXED rows from `url_inspections` against last-30-day impressions; ranks by impressions lost. | Priya, Aki |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C5 title-gap | Requires fetching page HTML — external service not in spec. | C2 cannibalize |
| C10 lost-queries | Same machinery as new-queries with inverted predicate. | C8 `new-queries --lost` (folded in) |
| C13 sync | Foundation, already in build priorities. | n/a |
| C14 graph export | Not weekly; emits raw artifact; user can already SQL it. | C2 cannibalize |
| C15 buckets | Histogram is wrapper-shaped; novel piece covered by momentum + opportunity. | C3 momentum |
