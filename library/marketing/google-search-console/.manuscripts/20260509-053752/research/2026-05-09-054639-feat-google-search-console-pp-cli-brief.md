# Google Search Console CLI Brief

## API Identity

- **Domain:** SEO performance data, indexing diagnostics, site/sitemap management
- **Users:** SEOs, content marketers, SEO automation engineers, AI agents doing research/audits
- **Data profile:** Time-series + dimensioned. Performance rows (clicks/imps/ctr/position keyed by query, page, country, device, search appearance, date) + per-URL index state + sitemap submission state + per-site permission state
- **API surface:** 10 endpoints across 4 resources (sites, sitemaps, searchanalytics, urlInspection) -- `mobileFriendlyTest` was retired Dec 4, 2023 and excluded from spec
- **Spec source:** Hand-written OpenAPI 3.0.3 derived from Google's Discovery doc at `https://searchconsole.googleapis.com/$discovery/rest?version=v1`
- **Auth:** OAuth 2.0 bearer access token (env: `GSC_ACCESS_TOKEN`). Pre-fetched token mirrors the google-ads CLI pattern in this library -- no browser flow code in the printed CLI.
- **Server:** `https://searchconsole.googleapis.com`

## Reachability Risk

- **None.** Probed live with provided OAuth token, returned HTTP 200 + 1 site (siteOwner). Spec is canonical. No bot-protection, no Cloudflare, no rate-limit issues from research. Per-day quotas exist (URL Inspection ~2,000/day per property; Search Analytics request quota in tens of thousands) but those are quota mechanics, not reachability risk.

## Top Workflows

1. **Pull search analytics for a date range, grouped by query/page/country/device.** The dominant workhorse endpoint. Often paginated past the 25k row cap.
2. **Period-over-period comparison.** "What changed week-over-week / MoM / YoY?" -- clicks, impressions, CTR, average position deltas. The single most-requested aggregation that the API doesn't natively provide.
3. **Find quick wins.** Queries currently ranking 11-20 with high impressions and low CTR -- page-2 to page-1 opportunities. Surfaced as a discrete workflow by ahonn/mcp-server-gsc and AminForou/mcp-gsc.
4. **Detect keyword cannibalization.** Multiple pages ranking for the same query. AminForou's MCP exposes this as a first-class tool.
5. **Inspect URL index status (single + batch).** Debug "why isn't this page indexed?" -- verdict, coverage state, robots.txt state, indexing state, canonical mismatch, last crawl. Single-URL only at the API level; batch is N parallel calls bounded by the 2k/day quota.
6. **Submit / list / monitor sitemaps.** Automate "submitted vs. indexed" tracking, auto-resubmit on failure, alert on warnings/errors.
7. **Cross-property analytics.** Run the same analytics query across N verified properties at once -- the API requires one call per site, but the workflow is "show me top queries across all my sites."

## Table Stakes (must match every existing tool)

From Bin-Huang/google-search-console-cli (closest agent-native CLI competitor, 11 commands):
- `query` (search analytics) with `--start-date`, `--end-date`, `--dimensions`, `--row-limit`, `--start-row`, `--data-state`, `--aggregation-type`, `--type`, `--all` (auto-paginate), `--dimension-filter`
- `sites`, `site`, `site-add`, `site-remove`
- `sitemaps`, `sitemap`, `sitemap-submit`, `sitemap-delete`
- `inspect`, `inspect-batch` (NDJSON streaming, `--file` input)
- JSON default + `--format compact` for single-line / NDJSON

From AminForou/mcp-gsc (20 MCP tools, the "feature ceiling" reference):
- `list_properties`, `get_site_details`, `add_site`, `delete_site`
- `get_search_analytics`, `get_performance_overview`, `compare_search_periods`, `get_search_by_page_query`, `get_advanced_search_analytics`
- `inspect_url_enhanced`, `batch_url_inspection`, `check_indexing_issues`
- `get_sitemaps`, `list_sitemaps_enhanced`, `manage_sitemaps`
- `get_capabilities`, `reauthenticate`
- **Cannibalization detection** (multiple pages ranking for same query)
- **Position-11-20 opportunity surfacing** (high imps, low CTR)
- **Period-over-period comparison** as a first-class workflow

From ahonn/mcp-server-gsc (most-cited MCP):
- **Quick Wins detection** with configurable thresholds (position range, min impressions, min CTR)
- Regex filtering across query/page dimensions
- Up to 25k rows per call

From Josh Carty's `searchconsole` Python lib (the recommended community library):
- Fluent query builder (`.range`, `.dimension`, `.filter`, `.search_type`)
- `range(date, days=N)` ergonomic date helper
- `.to_dataframe()` (pandas integration -- CLI analog: CSV/TSV out)

## Codebase Intelligence

Skipped DeepWiki / MCP-source-code deep-read. Auth pattern is well-known (OAuth2 bearer access token in `Authorization: Bearer <token>` header), Discovery doc is canonical, and the spec was hand-derived from it directly. No additional ground truth needed.

## Data Layer

The transcendence features depend on the local store. Primary entities to persist:

- **Sites** -- verified properties + permission level. Cheap, listed once, refreshed on demand.
- **Sitemaps** -- per-site sitemap state (path, status, errors, warnings, content counts, last-submitted/downloaded). Time-series -- track over time to alert on regressions.
- **SearchAnalyticsRows** -- the workhorse table. One row per (siteUrl, date, query, page, country, device, search-appearance, search-type) tuple, with clicks/imps/ctr/position. Sync cursor: `(siteUrl, date)`. This table grows fast -- partition by site, retain forever (transcendence over the 16-month API window).
- **UrlInspections** -- per-URL inspection snapshots, time-series. Track index-state changes over time.

**Sync strategy:**
- `sync` pulls last 90 days by default (configurable). Idempotent on `(siteUrl, date, query, page, country, device, type)` primary key.
- Incremental: store last-fetched-date per site per dimension-set, only pull deltas.
- FTS5 over query text + page URL for offline `search` command.

## User Vision

User asked for a generic public CLI to contribute to the printing-press-library -- explicitly not tailored to the user's own GSC use cases. Scope is "the canonical GSC CLI everyone in SEO would reach for": agent-native, offline-cacheable, beats every existing tool on feature count, and has transcendence features that only a local-SQLite CLI can deliver.

## Source Priority

Single source -- skipping multi-source priority gate.

## Product Thesis

- **Name:** `google-search-console-pp-cli` (binary), `google-search-console` (slug). Display name in narrative: **Google Search Console**.
- **Headline thesis:** "Every Google Search Console feature you'd reach for, plus offline SQLite, period comparison, quick wins, and keyword cannibalization detection -- none of which the API or web UI offers natively."
- **Why it should exist:** The 5 existing CLIs/MCPs each pick a slice. None of them cover the full landscape and none cache locally. The press's offline-SQLite + agent-native pattern fits GSC perfectly because the most valuable SEO workflows (period comparison, cannibalization, time-series tracking past 16 months, cross-property roll-ups) all require persistence outside the API's native window.

## Build Priorities

**Priority 0 (foundation):**
- Bearer-token auth (`GSC_ACCESS_TOKEN`)
- SQLite store with all primary entities + FTS5 over queries+pages
- `sync` command (pull search analytics, sites, sitemaps incrementally)
- `search` command (FTS5 query)
- `sql` (raw SELECT-only)
- Generated endpoint-mirror commands (sites/sitemaps/searchanalytics/url-inspection)

**Priority 1 (absorb every feature from every competitor):**
- All 11 Bin-Huang commands matched (with our agent-native polish)
- All 20 AminForou MCP tools matched as commands
- ahonn-style configurable quick-wins thresholds
- Josh Carty-style date range presets (`--last 7d`, `--last 28d`, `--last 3mo`)
- Auto-pagination via `--all` on `query`
- Batch URL inspection via `--file` (NDJSON streaming output)
- CSV/TSV output toggle

**Priority 2 (transcendence -- only possible with our approach):**
- `quick-wins` -- page-2→page-1 candidates with configurable thresholds, queries surfaced from the local store (no live API call needed once synced)
- `cannibalization` -- pages competing for the same query, with severity ranked by combined impressions
- `compare` -- period-over-period delta on any dimension, with significance hints (clicks Δ, impressions Δ, CTR Δ pp, position Δ)
- `cliff` -- detect day-over-day click/impression cliffs from local snapshots (sitemap-wipe, ranking loss, indexing drop signatures)
- `roll-up` -- cross-property aggregation in one command (sum / avg / top-N across N sites from local cache)
- `coverage-drift` -- track URL inspection state changes over time from snapshot history
- `historical` -- query data older than the API's 16-month window (from local cache)
- `outliers` -- queries/pages with anomalous CTR for their position (using observed CTR-by-position curves from your own corpus)

**Stretch:**
- `--compare prev-period` shorthand on `query`
- `--dim` shorthand vs. `--dimensions`

## Build Sequence Hints

- The endpoint-mirror commands the press generates from the spec cover most of Priority 1 mechanically
- Only manual Priority 1 work: `--all` auto-paginate flag, `--last Nd` date presets, `inspect-batch --file` NDJSON streaming
- Priority 2 is all hand-built novel-feature work -- aim for at least 5 transcendence features that score ≥5/10
