# Traffic Intel Printing Press Skill

Use `traffic-intel-pp-cli` when an agent needs a private, local-first view of traffic, revenue, and refresh priorities across Search Console, Analytics, and backlink-style metrics.

## Rules

- Do not make external API calls from this MVP.
- Run `traffic-intel-pp-cli --agent agent-context` before automated workflows and honor schema `traffic-intel.agent-context/v1`.
- Run `traffic-intel-pp-cli --agent sources doctor` before live child CLI sync to check binary/env readiness.
- Run `traffic-intel-pp-cli --profile <name> sync` before analysis commands if local data is missing.
- Run `traffic-intel-pp-cli --profile <name> confidence` before trusting forecast or revenue-risk output.
- Run `traffic-intel-pp-cli --profile <name> movers` after at least two syncs to identify climbers, droppers, new Strike Zone entrants, and new revenue-at-risk pages.
- Prefer JSON via `--agent` for machine parsing.
- Treat env var values as secrets; doctor output reports presence only.

## Local-first source model

The MVP combines local data shaped like:

- GSC: clicks, impressions, CTR, average position, previous clicks, query sample.
- GA4: sessions, conversions, revenue, previous sessions/revenue.
- Ahrefs: backlinks, referring domains, top keyword.

Top-level `PageMetrics` fields remain available for simple ranking, while `sources.gsc`, `sources.ga4`, and `sources.ahrefs` preserve provenance for agents.

Every sync preserves the latest `<profile>-data.json` file and appends a dated snapshot under `snapshots/<profile>/` with schema version, source command versions, date range, and input hashes. Retention keeps daily snapshots for 30 days and weekly snapshots after that. Mover and outcome notes append to `learnings/<profile>.md`.

## Child CLI Sync

This scaffold does not import sibling `internal` packages and does not call APIs directly. `sync --source`, `--live`, and `--real` shell out to private child CLIs and ingest JSON:

- `google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions ["page"] --start-date <start> --end-date <end> --type WEB`
- `google-analytics-pp-cli top-pages --agent --property <property> --start <start> --end <end>`
- `ahrefs-pp-cli site-explorer top-pages --agent --target <target> --date <date> --select url,sum_traffic,keywords,referring_domains,top_keyword`

Use `sources doctor` to discover whether those binaries are on `PATH` and whether expected env vars (`GSC_SITE_URL`, `GA4_PROPERTY_ID`, `AHREFS_PROJECT` / `AHREFS_TARGET`) are present.

`sync --source all`, `--live`, and `--real` require all three source configs. Use `--source gsc`, `--source ga4`, or `--source ahrefs` for an intentional single-source sync.

Child CLI JSON must include a supported `schema_version`; unknown or missing child schemas fail closed before data feeds confidence, movers, or later apply surfaces.

## Commands

- `agent-context` — schema-versioned machine context with commands, env presence, and child source plan.
- `doctor` — local readiness, optional child CLI discovery, and env presence without secret values.
- `sources doctor` — source-specific child binary/env readiness table or JSON.
- `profile save/list/show/delete` — profile state under `~/.traffic-intel-pp-cli`.
- `sync` — embedded ecommerce fixture, `--import` local JSON, or opt-in child CLI sync.
- `movers` — snapshot diff for climbers, droppers, new Strike Zone entrants, and new revenue-at-risk pages.
- `confidence` — High/Medium/Low/Broken trust score with source coverage, freshness, tracking, and schema checks.
- `money-pages` — revenue-ranked URLs.
- `query-revenue` — revenue for matching URLs/titles.
- `explain-drop` — local before/after drop explanations.
- `refresh-queue` — page update priorities.
- `opportunity-gap` — high-impression, near-ranking pages scored by CTR gap and business value.
- `quick-wins` — near-page-one pages with weak CTR and conversion/revenue value.
- `revenue-at-risk` — pages where organic/session decline overlaps with commercial value.
- `refresh-brief <url-or-topic>` — agent-ready page refresh brief with likely issue and actions.
- `cannibalization` — same-topic competing pages and canonical consolidation suggestions.
- `topic-clusters` — traffic, revenue, backlink, and decay summaries by inferred topic.
- `source-coverage` — page-level audit of GSC, GA4, and Ahrefs evidence gaps.
- `internal-link-plan` — source and target page recommendations for internal links.
- `experiment-plan <url-or-topic>` — title, meta, content, and measurement tests for one page.
- `forecast-impact` — confidence-gated estimated click, conversion, and revenue upside from CTR-gap closure.
- `stale-winners` — high-value pages to refresh before visible decay.
- `digest weekly` — mover-led weekly summary with profile, date-range fallback, source coverage, confidence, and mode; safe for empty datasets.

`opportunity-gap` and `quick-wins` label every row with defend 1-4, move 5-20 Strike Zone, or ignore 21+ framing. This is presentation-only; it does not change the scoring math.
