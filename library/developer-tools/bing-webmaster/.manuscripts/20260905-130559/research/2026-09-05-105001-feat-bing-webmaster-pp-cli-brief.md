# Bing Webmaster API CLI Brief (reprint redo, run 20260905-104918-633b1430, 2026-09-05)

> Reprint of run 20260616-212356 (press 4.3.0) under press 4.31.1 with REDO research.
> Endpoint surface re-verified against current Microsoft Learn docs (see research verdict);
> the 8 prior novel commands (review, drift, publish, triage, quota, gap, feed-health, watch)
> are carried as Pass 2(d) reconciliation input — keep, reframe, or drop with reasons, never silent.

## API Identity
- **Domain:** SEO / site management for the Bing search index (Bing Webmaster Tools API). NOT the retired Bing Search API v7 — this service is live.
- **Users:** SEOs, agencies, publishers/e-commerce, and AI agents managing site indexing on Bing.
- **Data profile:** Per-site time-series (query/page/traffic/crawl stats over ~16 months), submission quotas, crawl issues, feeds, keywords, inbound links, normalization & geo settings.

## Auth & Transport (critical for spec authoring)
- **Base URL:** `https://ssl.bing.com/webmaster/api.svc/json/<Method>?apikey=<KEY>`
- **Verbs:** GET = `[WebGet]` reads (params in query string); POST = `[WebInvoke Method="POST"]` writes (flat JSON body, params as keys, `Content-Type: application/json`).
- **Auth:** `apikey` query parameter (per-user key, covers all verified sites). Env var: `BING_WEBMASTER_API_KEY`. OAuth2 bearer also supported (recommended by MS) — v1 uses api key.
- **Response envelope:** WCF/ASP.NET-AJAX `{"d": ...}` — object/array/null wrapped under `d`. Nested objects carry `__type` discriminators. **Client must unwrap `d`.**
- **Errors:** HTTP 400 with UNWRAPPED body `{"ErrorCode":<int>,"Message":"..."}` (e.g. `InvalidApiKey`). Client must detect this shape and surface actionable errors.
- **Dates:** Microsoft format `/Date(<ms-since-epoch><±offset>)/` (e.g. `/Date(1316156400000-0700)/`). Must be parsed/normalized to ISO.
- **Quotas/limits:** No documented RPS. URL submission quota dynamic per-site (headline 10k/day), `SubmitUrlBatch` caps at **500 URLs/request**. `GetUrlSubmissionQuota` → `{DailyQuota, MonthlyQuota}`. Resets midnight GMT.

## Reachability Risk
- **Low–Medium.** API is live and api-key reachable. Known quirks (not blockers): `GetQueryStats` returns sparse/empty arrays below an opaque data threshold (valid, not an error); recurring 403s on URL submission are usually upstream WAF/Bingbot-blocking, not the API; docs date from 2019/2022 and drift slightly. Treat empty arrays as valid; surface 403 with a hint.

## Top Workflows
1. **Weekly query-performance review** — query + rank/traffic stats for last 7 days vs prior 7 days; surface new/lost queries, CTR & avg-position deltas. *(No Bing tool computes deltas.)*
2. **Bulk URL submission on publish** — SubmitUrlBatch / IndexNow gated by GetUrlSubmissionQuota with quota-aware pacing (500/batch).
3. **Crawl-error triage** — GetCrawlIssues + GetCrawlStats, categorize by severity, diff vs last snapshot, map to affected URLs.
4. **Sitemap/feed health monitoring** — GetFeeds + GetFeedDetails, track submitted/discovered/indexed counts over time, alert on drops.
5. **Bing-vs-Google coverage gap** — join Bing query/page stats with GSC to find ranking/indexation gaps between engines. *(Wholly unserved — strongest moat.)*
6. **Export historical query data to a warehouse** — Bing has no native BigQuery export; the API + a local store is the only path.

## Table Stakes (must-have, from competitor catalog)
- API-key auth + multi-site (`GetUserSites`, default-site config), `doctor`.
- Query/page performance: GetQueryStats, GetPageStats, GetRankAndTrafficStats, GetQueryPageStats, date-range filtering.
- URL submission: SubmitUrl, SubmitUrlBatch, GetUrlSubmissionQuota (+ IndexNow path).
- URL/index inspection: GetUrlInfo, GetUrlTrafficInfo, GetChildrenUrlInfo.
- Crawl health: GetCrawlStats, GetCrawlIssues.
- Feeds/sitemaps: SubmitFeed/GetFeeds.
- Keyword research (Bing-unique vs GSC): GetKeyword, GetKeywordStats, GetRelatedKeywords.
- Structured output: json/csv/table; uniform JSON envelope; filter mini-language (steal from awkoy/gsc-cli).

## Complete Endpoint Surface (60 methods — all must be covered)
- **Sites (9):** GetUserSites, AddSite, VerifySite, RemoveSite, GetSiteRoles, AddSiteRoles, RemoveSiteRole, GetSiteMoves, SubmitSiteMove
- **Submission (8):** SubmitUrl, SubmitUrlBatch, GetUrlSubmissionQuota, SubmitContent, GetContentSubmissionQuota, FetchUrl, GetFetchedUrls, GetFetchedUrlDetails
- **Feeds (4):** SubmitFeed, GetFeeds, GetFeedDetails, RemoveFeed
- **Crawl (6):** GetCrawlStats, GetCrawlIssues, GetCrawlSettings, SaveCrawlSettings, GetUrlInfo, GetChildrenUrlInfo
- **Traffic/Query (9):** GetRankAndTrafficStats, GetQueryStats, GetQueryTrafficStats, GetQueryPageStats, GetQueryPageDetailStats, GetPageStats, GetPageQueryStats, GetUrlTrafficInfo, GetChildrenUrlTrafficInfo
- **Keywords (3):** GetKeyword, GetKeywordStats, GetRelatedKeywords  *(take q/country/language, NOT siteUrl)*
- **Links (4):** GetLinkCounts, GetUrlLinks, GetConnectedPages, AddConnectedPage
- **Deep links (6):** GetDeepLink*(obsolete)*, GetDeepLinkAlgoUrls*(obsolete)*, UpdateDeepLink*(obsolete)*, GetDeepLinkBlocks, AddDeepLinkBlock, RemoveDeepLinkBlock
- **Blocked/preview (6):** GetBlockedUrls, AddBlockedUrl, RemoveBlockedUrl, GetActivePagePreviewBlocks, AddPagePreviewBlock, RemovePagePreviewBlock
- **Query params (4):** GetQueryParameters, AddQueryParameter, EnableDisableQueryParameter, RemoveQueryParameter
- **Geo (3):** GetCountryRegionSettings, AddCountryRegionSettings, RemoveCountryRegionSettings

## Data Layer (local SQLite — enables transcendence)
- **Primary entities:** sites, rank_traffic_stats (date series), query_stats (query×date), page_stats, query_page_stats, crawl_stats (date series), crawl_issues, feeds, blocked_urls, link_counts, keywords, url_info.
- **Sync cursor:** date-stamped snapshots per site → enables period-over-period deltas, ranking drift, week-over-week query diffs.
- **FTS/search:** FTS5 over queries, pages, crawl-issue URLs.

## Existing Tooling (absorb targets)
- MCP servers: isiahw1/mcp-server-bing-webmaster (50 tools, active), zizzfizzix/mcp-server-bwt (70+ tools), saurabhsharma2u/search-console-mcp (multi-engine incl. Bing).
- Libs: merj/bing-webmaster-tools (Python, de-facto base), seo-meow Rust crate (full coverage), webjeyros PHP.
- CLI: NmadeleiDev/bwm — only ~5 of 60 methods (hollow). **No mature standalone CLI exists — the gap we fill.**
- GSC design references: awkoy/gsc-cli (JSON envelope, filter language, bulk sitemap-inspect), gsccli (period compare).

## User Vision
- User's words: "een complete CLI van alle mogelijke endpoints" + "out of the box commands" + "een perfecte tool voor SEO". → 100% endpoint coverage is mandatory scope, plus differentiating SEO intelligence commands. Final CLI to be placed in `C:\Users\pimme\Agents\Bing_cli_tool\`.

## Product Thesis
- **Name:** bingmaster (binary: `bing-webmaster-pp-cli`)
- **Why it should exist:** Every existing Bing tool stops at "expose raw endpoints." None compute deltas, ranking drift, quota pacing, bulk parallelism, or Bing↔Google reconciliation. This CLI matches the full 60-method surface AND layers the SEO intelligence nobody ships — with offline SQLite, `--json`/`--select`, typed exit codes, and MCP exposure for agents.

## Build Priorities
1. **P0:** SQLite data layer for all stat/issue/feed/site entities + sync + FTS + SQL path. `d`-envelope-unwrapping client + MS-date parser + error-shape handler.
2. **P1 (absorb):** All 60 endpoints as typed commands (obsolete deep-link methods shipped but flagged), quota-aware batch submit, full structured output.
3. **P2 (transcend):** Query-performance delta review, ranking-drift detection, crawl-error triage+diff, feed-health monitor, quota intelligence/pacing, IndexNow-unified publish pipeline, Bing-vs-Google gap (GSC join).
