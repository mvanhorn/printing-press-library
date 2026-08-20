# Google Search Console CLI -- Absorb Manifest

> Phase 1.5 manifest. 29 absorbed features (match every existing tool) + 11 transcendence features (only possible with the press's offline-SQLite + agent-native pattern). All transcendence features scored ≥6/10 by the novel-features subagent.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List verified properties | Bin-Huang `sites`, AminForou `list_properties` | Generated `sites list` from spec | Cached locally, FTS-searchable, --json/--csv/--md |
| 2 | Get property detail + permission level | Bin-Huang `site`, AminForou `get_site_details` | Generated `sites get` | Cached, doesn't re-hit API |
| 3 | Add a property | Bin-Huang `site-add`, AminForou `add_site` | Generated `sites add` | --dry-run |
| 4 | Remove a property | Bin-Huang `site-remove`, AminForou `delete_site` | Generated `sites delete` | --dry-run |
| 5 | List sitemaps | Bin-Huang `sitemaps`, AminForou `get_sitemaps` | Generated `sitemaps list` | Cached, time-series tracking |
| 6 | Enhanced sitemap details (errors/warnings/contents) | AminForou `list_sitemaps_enhanced` | Same `sitemaps list` enriched with full WmxSitemap | One command, no separate "enhanced" variant |
| 7 | Get sitemap detail | Bin-Huang `sitemap`, AminForou via manage | Generated `sitemaps get` | Snapshot history |
| 8 | Submit sitemap | Bin-Huang `sitemap-submit`, AminForou `manage_sitemaps` | Generated `sitemaps submit` | --dry-run, idempotent |
| 9 | Delete sitemap | Bin-Huang `sitemap-delete`, AminForou `manage_sitemaps` | Generated `sitemaps delete` | --dry-run |
| 10 | Search analytics query (clicks/imps/ctr/position) | Bin-Huang `query`, AminForou `get_search_analytics`, ahonn `search_analytics` | Generated `searchanalytics query` from spec | Local persistence, cross-property roll-up, FTS over query text |
| 11 | All dimensions (query/page/country/device/searchAppearance/date) | Bin-Huang/ahonn dimensions | Spec dimensions enum | --dim shorthand |
| 12 | Multi-dim group filter (operators) | Bin-Huang `--dimension-filter`, ahonn dimensionFilterGroups | dimensionFilterGroups passthrough + --filter shorthand | Friendly `--filter country=USA,device=MOBILE` syntax |
| 13 | Regex dimension filter | ahonn `regex:` prefix, AminForou regex filtering | --filter operator includingRegex/excludingRegex | Exposed as --filter ~ syntax |
| 14 | Auto-paginate over 25k row limit | Bin-Huang `--all` | --all flag walks startRow until empty | Streams NDJSON when --all + --json |
| 15 | searchType / type (web/image/video/news/discover/googleNews) | Bin-Huang `--type`, ahonn search type | Spec --type | Default web |
| 16 | dataState (final/all) | Bin-Huang `--data-state` | Spec --data-state | Final by default for repeatable reports |
| 17 | aggregationType (auto/byPage/byProperty) | Bin-Huang `--aggregation-type` | Spec --aggregation-type | -- |
| 18 | Inspect single URL index status | Bin-Huang `inspect`, AminForou `inspect_url_enhanced` | Generated `url-inspection inspect` | Local snapshot history (track changes over time) |
| 19 | Batch URL inspection (NDJSON) | Bin-Huang `inspect-batch --file`, AminForou `batch_url_inspection` | Hand-built `url-inspection inspect-batch --file` | Streams NDJSON, --max-per-day quota guard |
| 20 | Search terms by page (per-URL slice) | AminForou `get_search_by_page_query` | Built into `searchanalytics query` w/ filter | Single command, --filter page=URL |
| 21 | Indexing audit batching with prioritized fixes | AminForou `check_indexing_issues` | Hand-built `index-audit` | -- |
| 22 | Date range presets (--last 7d / 28d / 3mo) | Josh Carty `.range(date, days=N)` | Hand-built date helpers in query/compare/roll-up | --prev-period for built-in delta |
| 23 | CSV/TSV output | Josh Carty pandas | Press --csv flag | Standard framework |
| 24 | Markdown output (for AI agents) | kasdimg `--output md` | Press --md flag | Standard framework |
| 25 | JSON default with compact NDJSON | Bin-Huang `--format compact` | Press --json + --compact | Standard framework |
| 26 | Capabilities/auth status check | AminForou `get_capabilities` | Press `doctor` command | Adds API reachability + scope verification |
| 27 | Local SQLite cache + sync | (transcendence base -- no competitor) | Press `sync` + store | Time-series persistence |
| 28 | FTS5 search across cached queries+pages | (transcendence base) | Press `search` | Offline, regex-capable |
| 29 | Raw SQL access (SELECT only) | (transcendence base) | Press `sql` | Joins, window functions |

**No stubs planned.** Every absorbed row is shipping-scope.

## Transcendence (only possible with our approach)

11 hand-built novel features, all scoring ≥6/10 in the rubric.

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| T1 | Quick Wins | `quick-wins [site] [--position 8-20] [--min-imps 100] [--min-ctr 0.05]` | 9/10 | Local SQLite SELECT on search_analytics_rows WHERE position BETWEEN 8 AND 20 AND impressions >= min, ordered by impressions*(target_ctr - actual_ctr) | ahonn/mcp-server-gsc flagship; AminForou exposes it; Mira's persona pain explicit |
| T2 | Cannibalization | `cannibalization [site] [--min-imps 50] [--top N] [--include-singletons]` | 9/10 | GROUP BY query in local store, HAVING COUNT(DISTINCT page) > 1, severity = SUM(impressions) | AminForou first-class MCP tool; brief workflow #4 |
| T3 | Period compare | `compare [site] --period 28d --vs prev-period [--dim query] [--top 50]` | 9/10 | Two date-window aggregates from local store joined on dimension; emits Δclicks, Δimps, ΔCTR pp, Δposition | Brief workflow #2 ("most-requested aggregation API doesn't provide"); AminForou compare_search_periods |
| T4 | Cliff detector | `cliff [site] [--metric clicks] [--threshold -25%] [--window 7d]` | 7/10 | Day-over-day delta from snapshots; flag drops below threshold; signature hints (sitemap-wipe, indexing drop) | Brief Priority 2; Devon cliff-investigation workflow 2-3x/week |
| T5 | Cross-property roll-up | `roll-up [--metric clicks] [--top 50] [--group-by query] [--last 28d]` | 8/10 | Single SQL aggregate across all (siteUrl, ...) partitions in local store | Brief workflow #7 cross-property; Devon agency persona; no competitor offers it |
| T6 | Coverage drift | `coverage-drift [site] [--field indexingState] [--days 30]` | 7/10 | Diff url_inspections snapshots over time on chosen field, surface URLs that flipped | Brief Priority 2; absorbs index-diff via `--field indexingState`; Devon-Riya pain |
| T7 | Historical (>16mo) | `historical [site] --start <date> --end <date> [--dim query]` | 8/10 | SELECT from local store on date range that predates the 16-month API window | Brief Priority 2 explicitly; only feature truly impossible via the API |
| T8 | CTR outliers | `outliers [site] [--metric ctr] [--top 50] [--sigma 2]` | 6/10 | Bucket rows by integer position, compute mean+stddev CTR per bucket from local corpus, flag rows where \|actual_ctr - bucket_mean\| > sigma * bucket_stddev | Brief Priority 2; no competitor has it; needs corpus → genuine transcendence |
| T9 | Sitemap regression watcher | `sitemap-watch [site] [--since 7d]` | 7/10 | Diff sitemaps table snapshots over window; surface new errors/warnings, content-count drops, last-downloaded staleness | Brief workflow #6 monitor sitemaps; Riya Friday ritual |
| T10 | Decaying pages | `decaying [site] [--window 90d] [--min-imps 500] [--top 50]` | 7/10 | Linear regression slope on per-page weekly clicks over window; rank by negative slope * total impressions | Mira persona pain explicit ("which old posts are dying"); no competitor |
| T11 | New queries arriving | `new-queries [site] [--since 28d] [--min-imps 50] [--top 100]` | 7/10 | Local set difference: queries with impressions in last N days NOT in corpus before that window | Mira/Riya content-ideas workflow; impossible without retained history |

## Customer model summary (4 personas)

- **Riya** -- in-house SEO lead, 200-SKU ecom, lives in GSC web UI + Sheets, frustrated by 16-month window and missing period compare
- **Devon** -- agency SEO automation engineer with 30+ client properties, frustrated by per-client script duplication and no shared cache
- **Mira** -- content marketer, no SQL, runs single commands on demand, frustrated by buried Quick Wins and decaying pages
- **AI agent** -- needs deterministic JSON, full API surface in one binary, persistence across calls

Full personas in `2026-05-09-054639-novel-features-brainstorm.md`.

## Killed candidates (5)

Documented in audit-trail file. Summary: brand-split (wrapper-thin), index-diff (special case of coverage-drift), query-intent (LLM-dependent), competitor-overlay (external service required), query→page map (folded into cannibalization).
