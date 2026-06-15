# Traffic Intel Printing Press Skill

Use `traffic-intel-pp-cli` when an agent needs a private, local-first view of traffic, revenue, and refresh priorities across Search Console, Analytics, and backlink-style metrics.

## Rules

- Do not make external API calls from this MVP.
- Run `traffic-intel-pp-cli --agent agent-context` before automated workflows and honor schema `traffic-intel.agent-context/v1`.
- Run `traffic-intel-pp-cli --agent sources doctor` when deciding whether future child CLI sync is available.
- Run `traffic-intel-pp-cli --profile <name> sync` before analysis commands if local data is missing.
- Prefer JSON via `--agent` for machine parsing.
- Treat env var values as secrets; doctor output reports presence only.

## Local-first source model

The MVP combines local data shaped like:

- GSC: clicks, impressions, CTR, average position, previous clicks, query sample.
- GA4: sessions, conversions, revenue, previous sessions/revenue.
- Ahrefs: backlinks, referring domains, top keyword.

Top-level `PageMetrics` fields remain available for simple ranking, while `sources.gsc`, `sources.ga4`, and `sources.ahrefs` preserve provenance for agents.

## Future child CLI sync plan

This scaffold does not import sibling `internal` packages and does not call APIs. Future adapters should shell out to private child CLIs and ingest JSON:

- `google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions ["page"] --start-date <start> --end-date <end> --type WEB`
- `google-analytics-pp-cli top-pages --agent --property <property> --start <start> --end <end>`
- `ahrefs-pp-cli site-explorer top-pages --agent --target <target> --date <date> --select url,sum_traffic,keywords,referring_domains,top_keyword`

Use `sources doctor` to discover whether those binaries are on `PATH` and whether expected env vars (`GSC_SITE_URL`, `GA4_PROPERTY_ID`, `AHREFS_PROJECT` / `AHREFS_TARGET`) are present.

## Commands

- `agent-context` — schema-versioned machine context with commands, env presence, and child source plan.
- `doctor` — local readiness, optional child CLI discovery, and env presence without secret values.
- `sources doctor` — source-specific child binary/env readiness table or JSON.
- `profile save/list/show/delete` — profile state under `~/.traffic-intel-pp-cli`.
- `sync` — embedded ecommerce fixture or `--import` local JSON.
- `money-pages` — revenue-ranked URLs.
- `query-revenue` — revenue for matching URLs/titles.
- `explain-drop` — local before/after drop explanations.
- `refresh-queue` — page update priorities.
- `digest weekly` — weekly summary; safe for empty datasets.
