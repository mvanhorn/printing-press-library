# Google Search Console CLI Brief

## API Identity
- **Domain:** SEO observability — search performance analytics, site verification, sitemap submission, URL index inspection.
- **Users:** SEO practitioners, growth engineers, content ops, technical SEO consultants. Increasingly: AI agents driving SEO workflows.
- **Data profile:** Time-series search analytics rows (date × query × page × country × device × searchAppearance) with high cardinality (queries are essentially unbounded), plus low-cardinality reference data (sites, sitemaps) and per-URL inspection state.

## Reachability Risk
- **None.** Spec is on apis.guru, generated from Google Discovery. The API has been stable since 2014. Auth is OAuth2 with the `webmasters` (or `webmasters.readonly`) scope; `gcloud auth print-access-token` is the easiest local-dev path. Live smoke testing was declined by the user, so reachability is not gated on credential setup for this run.

## Top Workflows
1. **Daily/weekly performance pull** — query search analytics for last N days, dimension by query and page, filter by country/device, export to JSON/CSV for spreadsheets or downstream automation.
2. **Quick wins / opportunity hunting** — find pages ranking on positions 4-20 with high impressions but low CTR; surface them as "low-hanging fruit" for title/meta optimization.
3. **Sitemap lifecycle** — list, submit, and monitor sitemap status across multiple verified properties.
4. **URL index inspection** — check whether a specific URL is indexed, what crawl errors exist, what the last crawl date was; useful when shipping new pages or debugging deindexing.
5. **Period-over-period comparison** — compare a recent window (last 7 days) against a baseline (same window 30 days prior) to spot rising/falling queries and pages.
6. **Cross-property aggregation** — for agencies and orgs with many verified properties, pull and roll up analytics across N sites in one command.

## Table Stakes (every competing tool has these)
- **`searchanalytics.query`** with all dimensions (query, page, country, device, searchAppearance, date, hour) and all filter operators (equals, contains, notEquals, notContains, includingRegex, excludingRegex).
- **`sites.list/get/add/delete`** — verified property management (add/delete require Full permission).
- **`sitemaps.list/get/submit/delete`** — sitemap lifecycle (submit/delete require Full permission).
- **`urlInspection.index.inspect`** with optional `--language` for translated diagnostic messages.
- **Batch URL inspection** — Bin-Huang and AminForou both expose a batch wrapper (limited to ~10 URLs concurrently because the API is single-URL).
- **OAuth2 auth** with three credential paths: explicit JSON key file (service account), `GOOGLE_APPLICATION_CREDENTIALS` env var, gcloud Application Default Credentials.
- **Pretty + compact JSON output**; the better tools also stream NDJSON for batch operations.

## Data Layer (what deserves SQLite)
- **Primary entities:**
  - `sites` — small, slow-changing.
  - `sitemaps` — small, per-site.
  - `search_analytics_rows` — large, append-mostly. Schema: `(site_url, type, date, query, page, country, device, search_appearance, clicks, impressions, ctr, position, ingested_at)`. PK on the dimension tuple + date.
  - `url_inspections` — per-URL inspection snapshots; storing history enables coverage-drift detection.
- **Sync cursor:** date-based (`MAX(date) WHERE site_url = ?`); each sync window covers `[last_synced_date+1, today-3]` because GSC data finalizes ~3 days late.
- **FTS/search:** FTS5 on `(query, page)` so users can `search "checkout"` and find every analytics row plus the page URL hits.
- **Backfill window:** GSC retains ~16 months of data; new properties can be backfilled in chunks.

## Codebase Intelligence
*DeepWiki not queried — research signals from competitor MCP source were sufficient.*

- **Auth:** OAuth2 user/installed-app flow (preferred for local dev) or service account (preferred for CI). Token via `Authorization: Bearer <access_token>`. Easiest local source: `gcloud auth print-access-token`. Scope: `https://www.googleapis.com/auth/webmasters` for write, `.readonly` for query-only.
- **Data model:** Resources encoded in `operationId` (e.g., `searchanalytics.query`, `sites.list`, `sitemaps.submit`, `urlInspection.index.inspect`) — same convention as `google-cloud-run.yaml`. Generator's operationId-derivation path handles this.
- **Rate limiting:** Default quotas: 1,200 QPM per project per user, 30,000 QPD per project. Soft per-property limits exist for query/inspect endpoints. 429s should be retried with exponential backoff.
- **Architecture:** REST/JSON over HTTPS, hostname `searchconsole.googleapis.com`. URL inspection has its own root (`urlInspection.index.inspect`) but uses the same OAuth scope.
- **Surprising bits:** `searchAppearance` dimension is mutually exclusive with all other dimensions in a single query — must be requested alone. `data-state` toggles freshness (`all` includes preliminary data, `final` excludes; `hourly_all` is opt-in). `aggregationType` matters for property-level vs page-level row math.

## User Vision
*User selected "Let's go" without offering a vision — proceeding with my judgment.*

## Product Thesis
- **Name:** `google-search-console-pp-cli` (binary), `google-search-console` (slug). Display name: **Google Search Console**.
- **Why it should exist:** Every competing tool today either (a) wraps the API one call at a time with no persistence, or (b) offers a single in-memory analytic (quick wins, period comparison) that re-queries the API every time. None of them sync GSC data into a local store, which means none can answer questions that require joins across queries × pages × time × dimensions. The compound-insight angle — query-intent observatory: decay, cannibalization, opportunity, coverage-drift — is structurally impossible without local persistence. This CLI absorbs every feature the npm/MCP ecosystem ships, then adds the workflows that only a SQLite-backed agent-native CLI can express.

## Build Priorities
1. **Sync + store + search** (foundation) — a real SQLite backing for `search_analytics_rows`, `sites`, `sitemaps`, `url_inspections`. FTS5 over `(query, page)`. SQL-composable.
2. **Match the surface** — every endpoint Bin-Huang/AminForou/ahonn expose, with `--json`, `--select`, `--csv`, `--dry-run`, typed exit codes, agent-friendly errors. Includes batch URL inspection and regex filtering.
3. **Quick wins / opportunity** — match ahonn's `detectQuickWins` shape but back it with persistent baselines (so we can say "this page MOVED into the opportunity zone in the last 14 days" rather than just "this page is in the opportunity zone today").
4. **Compound insights (transcendence)** — decay, cannibalization, opportunity-with-baseline, coverage-drift, page-momentum, territorial-shift, device-spread. All SQL-backed; all impossible in any competing tool.
5. **Cross-property roll-up** — for agencies/orgs: one command, many sites, aggregated output.
