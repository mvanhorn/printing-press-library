# YC Companies CLI Brief

## API Identity
- Domain: Y Combinator startup directory (`ycombinator.com/companies`) — every company funded by YC since 2005.
- Users: VCs/scouts hunting deal flow, founders researching competitors/peers, recruiters sourcing from YC alumni, journalists/analysts doing portfolio analysis, agents pulling structured startup data for downstream tasks.
- Data profile: 5,889 companies across 49 batches, 59 industries, 332 tags. Per-company fields include id, name, slug, batch, status (Active/Acquired/Public/Inactive), team_size, locations, regions, industry/subindustry, tags, one_liner, long_description, website, launched_at, top_company, isHiring, nonprofit, founder-demographics highlights, demo-day video URL, question/answers.

## Reachability Risk
- **None.** Primary source is `https://yc-oss.github.io/api/` — a static JSON dump on GitHub Pages refreshed daily via GitHub Actions. No rate limit, no auth, no bot detection. Live Algolia fallback (`45bwzj1sgc-dsn.algolia.net`) is also unauthenticated for the public index. Browser-sniff is for spec enrichment and validating live paths, not for circumventing protection.

## Top Workflows
1. **Filter the directory** — "AI companies from W24 in SF, hiring, top company status" → table or JSON output for downstream tooling.
2. **Get fresh signal on a batch** — "Show everything from S25, sorted by launch date, with one-liners."
3. **Watch a portfolio** — Sync locally, mark a list of company slugs as "watched", surface status/team-size/hiring changes over time.
4. **Cross-batch industry analysis** — "Count AI companies per batch since W20", "Average team_size for fintech by batch."
5. **Export for outreach/CRM** — Filtered CSV/JSON exports with founder slugs, websites, one-liners for cold outreach or spreadsheet workflows.
6. **Discover similar companies** — Given a YC slug, find peers by overlapping tags + industry.
7. **Detect new launches since last sync** — "What companies appeared since I last ran sync?"

## Data Layer
- Primary entities: `companies` (5,889 rows, full field set), `batches` (49 rows, with batch metadata), `industries` (59), `tags` (332), `regions`.
- Sync cursor: `meta.json` exposes `last_updated`; per-company `launched_at` is a Unix timestamp. We can do full-replace daily (data is small, ~3-5 MB JSON) and compute deltas via a `companies_history` snapshot table.
- FTS/search: FTS5 over `name`, `one_liner`, `long_description`, `tags`, `industry`, `founder_names` (from question_answers when available).

## Codebase Intelligence
- Source: yc-oss/api repo + the public reverse-engineering writeup (rayhanadev 2025) plus existing Python scrapers (Nneji123, corralm, akshaybhalotia, dirkjbreeuwer, goofygary).
- Auth: None for the static API or the Algolia public index. Algolia uses a restricted public key embedded in the page bundle.
- Data model: Algolia index `WaaSPublicCompanyJob_created_at_desc_production`. The `workatastartup.com/companies/fetch` endpoint can hydrate a company by ID for fields not in the search index (job listings, deeper founder bios).
- Rate limiting: yc-oss API is GitHub Pages — effectively unlimited GET. Algolia public endpoint has generous limits for the YC index.
- Architecture: Static JSON files (`all.json`, `batches/<slug>.json`, `industries/<slug>.json`, `tags/<slug>.json`, `companies/top.json`, `companies/hiring.json`, etc.) regenerated daily from the Algolia index by GitHub Actions.

## Product Thesis
- **Name:** `yc-companies-pp-cli` (binary `yc-companies-pp-cli`, library slug `yc-companies`).
- **Why it should exist:** Existing tools are Python scrapers that re-fetch from scratch on every run, output flat CSV/JSON, and have no concept of a local store, history, or composable filters. None expose a clean agent-native interface. With a local SQLite mirror + FTS, we get sub-second filtering across 5,889 companies, multi-tag intersections, batch-over-batch deltas, and queries no static endpoint supports ("AI companies hiring in SF/NYC, team_size 11-50, that appeared since W23"). All while staying offline-first and replayable.

## Build Priorities
1. **Data layer first** — `sync` pulls `meta.json` + `companies/all.json` into SQLite; normalizes industries/tags/regions; snapshots into `companies_history`.
2. **Absorb every filter dimension** that any static endpoint exposes (batch, industry, tag, top, hiring, nonprofit, demographic highlights) plus full-text search.
3. **Transcend with local-only queries** — multi-axis filters, history/deltas, similar-companies, watched-portfolio status diffs, cross-batch aggregations.
4. **Agent-native plumbing** — `--json`, `--select`, `--csv`, `--dry-run`, typed exit codes, `--agent` shortcut, MCP exposure for read-only queries.

## Reach Out To Source Files Of Interest
- `https://yc-oss.github.io/api/meta.json` — sync cursor + counts.
- `https://yc-oss.github.io/api/companies/all.json` — full corpus.
- `https://yc-oss.github.io/api/batches/<batch-slug>.json` — per-batch slice.
- `https://yc-oss.github.io/api/industries/<industry-slug>.json` — per-industry slice.
- `https://yc-oss.github.io/api/companies/top.json`, `companies/hiring.json` — curated views.
- `https://45bwzj1sgc-dsn.algolia.net/...` — live fallback (verified during browser-sniff).
