# Google Search Console CLI — Absorb Manifest

## Source tools surveyed

| Source | Type | Stars | Contributed |
|--------|------|-------|-------------|
| Bin-Huang/google-search-console-cli | npm CLI | — | full command surface (query/sites/sitemaps/inspect + batch) |
| ahonn/mcp-server-gsc | MCP | — | regex filter operators, `detectQuickWins` + thresholds |
| AminForou/mcp-gsc | MCP | — | `compare_search_periods`, `get_search_by_page_query`, `check_indexing_issues`, `batch_url_inspection` |
| ncosentino/google-search-console-mcp | MCP (zero-dep) | — | binary distribution pattern (we're Go, get this for free) |
| saurabhsharma2u/search-console-mcp | MCP | — | multi-source idea (we ship GSC only in v1) |
| joshcarty/google-searchconsole | Python SDK | — | `.range()` date helpers, `.to_dataframe()` shape (we ship `--csv` instead) |
| TechWithTy/google-search-console-sdk | Python SDK | — | typed models pattern (generator handles for free) |
| ivankristianto/google-search-console-cli | CLI | — | Indexing-API trigger (different API, out of scope for v1) |
| NmadeleiDev/google-search-console-cli | CLI | — | pipx install pattern (Go binary makes this trivial) |
| kasdimg/analytics-cli | CLI | — | GA4 + GSC unified service-account auth (we mirror with gcloud ADC) |
| ComposioHQ/awesome-claude-skills/google-search-console-automation | SKILL | — | skill format reference (generator emits SKILL.md natively) |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Query search analytics by dimensions | Bin-Huang `query`, ahonn `search_analytics` | Spec-derived `searchanalytics query` | `--json`, `--select`, `--csv`, typed exit codes, `--paginate-all` |
| 2 | Regex / contains / equals / not* filters | ahonn `search_analytics` (filterOperator) | Spec-derived; filter shape passed through | Composable with local store post-fetch |
| 3 | Quick wins detection (today's snapshot) | ahonn `detectQuickWins` (positionRange / minImpressions / minCtr) | Hand-built `quickwins` over local store | Reproducible thresholds in config; SQL-composable; works offline |
| 4 | Period-over-period comparison | AminForou `compare_search_periods` | Hand-built `compare` over local store | No re-querying API; supports any two windows |
| 5 | List sites | All competitors | Spec-derived `sites list` | — |
| 6 | Get / add / remove site | Bin-Huang `site`, `site-add`, `site-remove` | Spec-derived; add/remove gated to `webmasters` scope | — |
| 7 | List / get / submit / delete sitemap | Bin-Huang `sitemap*` | Spec-derived | — |
| 8 | URL inspection | All competitors `inspect` | Spec-derived `urlinspection inspect` | — |
| 9 | Batch URL inspection | Bin-Huang `inspect-batch`, AminForou `batch_url_inspection` | Hand-built `inspect-batch` (file/stdin) | NDJSON streaming; rate-limit-aware retry; persists to `url_inspections` |
| 10 | Cross-property aggregation (single-window) | analytics-cli (GA4 + GSC) | `--all-sites` flag on query | Per-site rollup with totals row |
| 11 | Pretty + compact JSON output | All competitors | Generated `--json` / `--csv` / `--select` / `--compact` | Standard across every command |
| 12 | Search by page (which queries drive a URL) | AminForou `get_search_by_page_query` | Spec-derived (filter on page) | — |
| 13 | Indexing-issues check (current state) | AminForou `check_indexing_issues` | Hand-built over local `url_inspections` table | Persistent — finds *new* issues since last sync |

All 13 absorbed rows are shipping scope. No stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|-------------------------|---------|
| 1 | Query decay watch | `decay --window 12w --min-loss 100` | 8/10 | Linear-fit slope per `(site, query)` over 16 months of daily rows; live API has no slope endpoint. | Maya, Aki |
| 2 | Keyword cannibalization | `cannibalize --top 20 --min-impressions 100` | 9/10 | `GROUP BY (site, query) HAVING COUNT(DISTINCT page) > 1` — single API call cannot express it. | Maya, Priya |
| 3 | Page momentum | `momentum --window 7d --vs 28d` | 7/10 | Windowed aggregation across daily rows; both directions (rising / collapsing). | Maya, Devin, Priya |
| 4 | Coverage drift | `coverage-drift --since last-sync` | 8/10 | Inter-snapshot diff on `url_inspections` — `coverageState` flips, canonical changes, stale crawls. Requires history. | Priya, Devin, Aki |
| 5 | Opportunity with baseline | `opportunity --new-since 14d` | 8/10 | Today's quick-wins JOIN prior daily snapshots → "this page entered the zone N days ago." | Maya, Priya, Aki |
| 6 | Territorial shift | `territory --by country,device` | 6/10 | Cross-time pivot on `(query × country × device)` showing share deltas. | Devin, Maya |
| 7 | Search-appearance breakdown | `appearance --window 28d --vs prior` | 6/10 | The `searchAppearance` dimension is mutually exclusive with others; nobody else stores it across time. | Maya, Devin |
| 8 | New (and lost) queries | `new-queries --window 7d --min-impressions 50 [--lost]` | 8/10 | `LEFT JOIN prior_window WHERE prior IS NULL`; impossible without persistence. | Aki, Maya |
| 9 | Sitemap health (regressions) | `sitemap-health --regressed` | 6/10 | Cross-snapshot diff on sitemap state; flags new errors/warnings/indexed-ratio drops. | Devin, Priya |
| 10 | Cross-property mover board | `book --window 7d --top 25` | 9/10 | UNION across all verified properties of (page, query) click deltas — Devin's whole Friday workflow in one command. | Devin |
| 11 | Indexing-issue triage | `triage --by impact` | 8/10 | Join non-INDEXED rows from `url_inspections` × prior-month impressions; rank broken pages by traffic lost. | Priya, Aki |

All 11 transcendence rows are shipping scope. No stubs. No hidden dependencies on external services.

## Things explicitly NOT in scope for v1

- **Indexing API trigger** (URL submission for re-crawl) — separate Google API with separate scope; ivankristianto's CLI does this; hold for v2.
- **HTML/title scraping** for content suggestions — out of scope; brief excludes scrapers.
- **GA4 join** — saurabhsharma2u multi-source pattern; hold for v2.
- **Bing Webmaster** — same.
