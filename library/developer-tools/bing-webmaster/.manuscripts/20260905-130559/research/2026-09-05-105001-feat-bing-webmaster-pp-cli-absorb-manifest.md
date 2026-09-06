# Bing Webmaster API CLI — Absorb Manifest

Strategy: **match the full 60-method API surface** (more than any existing CLI — the best one, `bwm`, covers ~5), beat every tool with offline SQLite + `--json`/`--select`/`--csv` + typed exit codes + MCP exposure, then layer the SEO intelligence (deltas, drift, quota pacing, bulk parallelism, Bing↔Google reconciliation) that **no existing tool ships**.

## Absorbed — match or beat everything that exists (all 60 IWebmasterApi methods)

| Group | Endpoints (commands) | Best existing source | Our added value |
|---|---|---|---|
| **Sites (9)** | GetUserSites, AddSite, VerifySite, RemoveSite, GetSiteRoles, AddSiteRoles, RemoveSiteRole, GetSiteMoves, SubmitSiteMove | isiahw1/zizzfizzix MCP | Multi-site default config, `--json`, store-cached site list |
| **Submission (8)** | SubmitUrl, SubmitUrlBatch, GetUrlSubmissionQuota, SubmitContent, GetContentSubmissionQuota, FetchUrl, GetFetchedUrls, GetFetchedUrlDetails | bwm (partial) | Quota-aware batching (500/req), `--dry-run`, idempotent |
| **Feeds/sitemaps (4)** | SubmitFeed, GetFeeds, GetFeedDetails, RemoveFeed | merj lib | Stored history → health diffs |
| **Crawl (6)** | GetCrawlStats, GetCrawlIssues, GetCrawlSettings, SaveCrawlSettings, GetUrlInfo, GetChildrenUrlInfo | MCP servers | Issue categorization + snapshots |
| **Traffic/Query (9)** | GetRankAndTrafficStats, GetQueryStats, GetQueryTrafficStats, GetQueryPageStats, GetQueryPageDetailStats, GetPageStats, GetPageQueryStats, GetUrlTrafficInfo, GetChildrenUrlTrafficInfo | MCP servers | Date-series store → deltas/drift |
| **Keywords (3)** | GetKeyword, GetKeywordStats, GetRelatedKeywords *(q/country/language, not siteUrl)* | merj lib | Bing-unique vs GSC; cached |
| **Links (4)** | GetLinkCounts, GetUrlLinks, GetConnectedPages, AddConnectedPage | zizzfizzix MCP | Paged fetch, FTS over link targets |
| **Deep links (6)** | GetDeepLink*(obsolete)*, GetDeepLinkAlgoUrls*(obsolete)*, UpdateDeepLink*(obsolete)*, GetDeepLinkBlocks, AddDeepLinkBlock, RemoveDeepLinkBlock | zizzfizzix MCP | Shipped + obsolete flagged in help |
| **Blocked/preview (6)** | GetBlockedUrls, AddBlockedUrl, RemoveBlockedUrl, GetActivePagePreviewBlocks, AddPagePreviewBlock, RemovePagePreviewBlock | isiahw1 MCP | `--dry-run` on every mutation |
| **Query params (4)** | GetQueryParameters, AddQueryParameter, EnableDisableQueryParameter, RemoveQueryParameter | zizzfizzix MCP | GSC-killed feature, full coverage |
| **Geo (3)** | GetCountryRegionSettings, AddCountryRegionSettings, RemoveCountryRegionSettings | isiahw1 MCP | GSC-killed feature, full coverage |

**Plus framework (auto-generated):** `doctor` (auth + reachability), `sync`, `search` (FTS5), `sql` (read-only), `context` (agent), `--json/--select/--compact/--csv`, MCP server (every command mirrored as a tool).

## Transcendence — what nobody else ships (P2, hand-built on the SQLite store)

| # | Feature | Command | Why only we can do this | Score |
|---|---------|---------|------------------------|-------|
| 1 | Query-performance delta review | `review --days 7` | New/lost queries + CTR & avg-position deltas vs prior period — needs date-stamped local snapshots no API call returns | 9 |
| 2 | Ranking-drift detection | `drift --query "..." ` / `drift --top 20` | Position change per query×page over time — requires historical snapshots joined locally | 9 |
| 3 | Quota-aware bulk submit + publish pipeline | `publish --from-sitemap <url>` / `submit bulk --file urls.txt` | Chunks to 500/batch, paces against live `GetUrlSubmissionQuota`, dedupes against already-submitted (store) | 9 |
| 4 | Crawl-error triage + diff | `triage` | Categorizes GetCrawlIssues by severity, diffs vs last snapshot, maps issues→affected child URLs in one local join | 8 |
| 5 | Submission quota intelligence | `quota` | Unifies URL + content quota, shows remaining daily/monthly + pacing recommendation; nobody surfaces "you have N left, pace at X/hr" | 8 |
| 6 | Bing↔Google coverage gap | `gap --gsc <export.csv>` | Joins Bing query/page stats with a GSC performance export to find queries/pages you rank on Bing but not Google (and vice-versa) — wholly unserved | 8 |
| 7 | Feed/sitemap health monitor | `feed-health` | Tracks submitted/discovered/indexed counts over time, alerts on drops — needs stored feed snapshots | 7 |
| 8 | Indexation watch (snapshot diff) | `watch` | Diffs latest sync vs prior: surfaces indexation/crawl/impression regressions per site | 7 |

All transcendence features score ≥5/10. None are stubs. Dependency note: `gap` reads a GSC performance export file (CSV/JSON) — no extra OAuth required; the user supplies the export (from GSC UI or the gsc tooling).

## Reachability / risk notes (carried to README)
- `GetQueryStats` can return empty arrays below an opaque data threshold — valid, not an error (handled as such).
- URL-submission 403s are usually upstream WAF/Bingbot-blocking — surfaced with a hint.
- MS `/Date(ms±offset)/` dates and the `{"d":...}` envelope are normalized in the HTTP client layer.
