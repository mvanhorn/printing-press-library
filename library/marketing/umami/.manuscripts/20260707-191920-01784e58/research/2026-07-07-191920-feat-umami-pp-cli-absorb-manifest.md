# Umami CLI — Ecosystem Absorb Manifest

Target: self-hosted **Umami v3** (instance verified live: umami.davidbarbier.com, 10 websites). Sources absorbed: 7 MCP servers (mikusnuz 66 tools, frontedu 61 incl. 16 composites, 0xtlt ~70, climactic 48, jakeyShakey, lukasschmit, Alurith), official @umami/api-client (all methods), umami-python, boly38 v3 client, umami-alerts, n8n template #2520, Grafana BI dashboards, web-analytics Claude plugin.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Websites list/get/create/update/delete/reset/transfer | mikusnuz umami-mcp (9 website tools) | (generated endpoint) websites list/get/create/update/delete/reset/transfer | --dry-run on mutations, --json/--select/--csv, typed exit codes |
| 2 | Website stats w/ comparison (prev/yoy) | all 7 MCPs get_stats | (generated endpoint) websites stats | human --period (7d/30d/today), growth % computed |
| 3 | Pageviews time series (unit/timezone) | all get_pageviews | (generated endpoint) websites pageviews | human periods, both backend x-formats parsed |
| 4 | Top metrics all v3 types incl. channel/entry/exit/utm* | all get_metrics | (generated endpoint) websites metrics | full v3 type enum, operator-prefix filters |
| 5 | Metrics expanded (per-item full stats) | frontedu get_metrics_expanded | (generated endpoint) websites metrics-expanded | tolerant string-number decode (live-verified quirk) |
| 6 | Active visitors now | all get_active | (generated endpoint) websites active | — |
| 7 | Collected-data date range | mikusnuz get_daterange | (generated endpoint) websites daterange | — |
| 8 | Realtime last-30-min | mikusnuz get_realtime | (generated endpoint) realtime get | --select deep filtering for agents |
| 9 | Sessions list/stats/weekly-heatmap/detail/activity/properties | mikusnuz + frontedu | (generated endpoint) sessions list/stats/weekly/get/activity/properties | offline mirror, --qualified filter |
| 10 | Events list + event series + event stats | mikusnuz | (generated endpoint) events list/series/stats | — |
| 11 | Event-data events/fields/properties/values/stats (+pivot) | mikusnuz + frontedu | (generated endpoint) event-data events/fields/properties/values/stats | v3 pivot + pf_* property filters |
| 12 | Session-data properties/values/stats | mikusnuz | (generated endpoint) session-data properties/values/stats | — |
| 13 | Saved reports CRUD | mikusnuz | (generated endpoint) reports list/get/create/update/delete | — |
| 14 | Report runners: funnel, retention, utm, goal, journey, revenue, attribution, breakdown, performance, heatmap | all MCPs (v2 names) | (generated endpoint) reports run-funnel/run-retention/run-utm/run-goal/run-journey/run-revenue/run-attribution/run-breakdown/run-performance/run-heatmap | **v3 body shapes** (filters top-level, ISO dates) — every MCP still ships v2 shapes |
| 15 | Revenue GET endpoints (chart/stats/metrics/sessions) | v3 API (no tool has them) | (generated endpoint) revenue chart/stats/metrics/sessions | first tool to expose them |
| 16 | Users CRUD + admin listings | mikusnuz 8 user tools | (generated endpoint) users create/get/update/delete + admin users/websites/teams | v3-correct (GET /users removed → /admin/users) |
| 17 | Teams full CRUD + members + join | mikusnuz 14 team tools | (generated endpoint) teams * | — |
| 18 | Me / my-websites / my-teams / password | mikusnuz account tools | (generated endpoint) me get/websites/teams/password | — |
| 19 | Shares create/update/delete/list (v3 entity shares) | 0xtlt umami-mcp (unique) | (generated endpoint) shares * + share resolve | v3 shareType enum (website/link/pixel/board) |
| 20 | Links + link shares (v3) | boly38 umami-api-client | (generated endpoint) links * | — |
| 21 | Pixels + pixel shares (v3) | boly38 | (generated endpoint) pixels * | — |
| 22 | Segments/cohorts CRUD (v3) | umami source (no tool has them) | (generated endpoint) segments * | first tool; segment/cohort usable as filter on all stats |
| 23 | Send event / pageview / identify / revenue event | umami-python new_event/new_revenue_event | (generated endpoint) send | tracker-testing ready, epoch-seconds handled |
| 24 | Batch event ingestion | mikusnuz batch_events | umami-pp-cli send batch | hand-built: /api/batch takes a top-level JSON array (not expressible in generated body schema); reads array from --file/stdin |
| 25 | Boards + dashboard (v3) | umami source (no tool has them) | (generated endpoint) boards * / dashboard get | first tool |
| 26 | Auth check / whoami / heartbeat | climactic umami_whoami | (behavior in umami-pp-cli doctor) auth + reachability + version detection | one command, agent-parseable |
| 27 | CSV export of website data | undocumented /export + pain issue #3208 | umami-pp-cli export | the missing self-hosted export button; unzips to CSVs |
| 28 | Filter-value autocomplete | @umami/api-client getWebsiteValues | (generated endpoint) websites values | — |
| 29 | JWT auto-refresh with expiry buffer | mikusnuz (best impl) | (behavior in umami-pp-cli auth login) decode exp, re-login 5-min buffer, cached token | no other CLI-shaped tool does this |
| 30 | Dual auth: self-hosted login + Cloud API key | all tools | (behavior in umami-pp-cli auth) canonical env vars UMAMI_URL/UMAMI_USERNAME/UMAMI_PASSWORD/UMAMI_API_KEY | unifies the fragmented env-var mess |
| 31 | Default website ID (implicit site) | climactic UMAMI_DEFAULT_WEBSITE_ID | (behavior in umami-pp-cli config) default-website config + domain→id resolution | address sites by domain, not uuid |
| 32 | Human period strings (24h/7d/this-month) | boly38 period API | (behavior in umami-pp-cli websites stats) --period on every stats command → epoch-ms | kills the #1 wrapper friction |
| 33 | Read-only / confirm safety modes | 0xtlt UMAMI_READONLY | (behavior in umami-pp-cli) --dry-run everywhere + typed exit codes | — |
| 34 | Site overview one-call (stats+top pages+referrers+devices+demographics) | frontedu get_site_overview + get_visitor_demographics | umami-pp-cli overview | parallel fan-out w/ partial-failure accounting |
| 35 | Period comparison with growth % | frontedu compare_periods | (behavior in umami-pp-cli websites stats) --compare prev/yoy output w/ growth % | native v3 compare param |
| 36 | Traffic sources / channel grouping | frontedu get_traffic_sources + Grafana channel grouping | (behavior in umami-pp-cli websites metrics) type=channel | native v3 channel metric |
| 37 | Engagement score 0-100 | frontedu get_engagement_score | umami-pp-cli engagement | reproducible formula documented |
| 38 | Growth trends (3-period acceleration) | frontedu get_growth_trends | umami-pp-cli trends | local snapshots → no triple API cost |
| 39 | Content performance: entry/exit/passthrough page roles | frontedu get_content_performance | umami-pp-cli pages | v3 entry/exit metric types native |
| 40 | Peak-hours publishing-window recommendation | frontedu get_peak_hours_analysis | umami-pp-cli peak-hours | sessions/weekly 7×24 native |
| 41 | Geo drill-down with share % | frontedu get_geo_insights | umami-pp-cli geo | country→region→city |
| 42 | Realtime-vs-baseline spike/drop detection | frontedu get_realtime_vs_baseline | umami-pp-cli pulse | baseline from local store, not extra API calls |
| 43 | UTM campaign performance (utm + attribution) | frontedu get_campaign_performance | umami-pp-cli campaigns | v3 attribution models first-click/last-click |
| 44 | Daily/weekly digest (markdown, Slack/email-ready) | Thunderbottom/umami-alerts + n8n template 2520 | umami-pp-cli digest | cron-friendly, --json/--plain/markdown |
| 45 | Multi-site portfolio sweep | web-analytics Claude plugin /analytics | umami-pp-cli portfolio | all 10 sites, one command, offline-capable |
| 46 | Qualified-sessions filter (≥2 pageviews, bot-strip) | Grafana BI dashboard | (behavior in umami-pp-cli sessions list) --qualified | — |
| 47 | Cloudflare Access service-token headers | lukasschmit umami-mcp | (behavior in umami-pp-cli config) extra-headers passthrough | protects CF-fronted instances |

Excluded from scope (with reason): jakeyShakey's docs-semantic-search / page-screenshot / HTML-fetch tools — not Umami-API features, out of CLI scope. AI narrative generation (n8n/rhelmer) — LLM dependency; our `digest --agent` output is the agent-native substitute.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Persona | Long Description |
|---|---------|---------|-------|--------------|------------------------|---------|------------------|
| 48 | Portfolio anomaly watch | `watch` | 10/10 | hand-code | Trailing 28-day per-weekday baseline per site requires local daily snapshots no API call provides; poll-once, cron-friendly. | David, Marc | Use this command for daily/weekly anomaly detection across synced local history for all sites. Do NOT use this command for last-30-minutes realtime spike checks; use 'pulse' instead. |
| 49 | SEO acquisition report | `seo` | 9/10 | hand-code | Joins local channel/referrer/entry-path metrics across two windows → organic & Google share with WoW delta; single API calls can't produce the delta view. | David | Use this command for a per-site organic/referral acquisition report (channel mix, Google share, entry pages). Do NOT use it for UTM campaign performance; use 'campaigns' instead. Do NOT use it for a full-site traffic summary; use 'overview' or 'digest' instead. |
| 50 | Tracker health check | `coverage` | 8/10 | hand-code | Detects zero-traffic gaps in local snapshot history, then confirms live (active/daterange) — dead trackers on a 10-site portfolio go unnoticed today. | David, Marc | Use this command to detect client sites whose tracking script stopped sending data. Do NOT use it to check CLI auth or API connectivity; use 'doctor' instead. |
| 51 | Movers report | `movers` | 8/10 | hand-code | Self-join of local metrics rows across adjacent windows ranks page/referrer risers & fallers; API returns one window at a time. | David, Inès | Use this command for page/referrer-level week-over-week risers and fallers on one site. Do NOT use it for site-level growth acceleration across periods; use 'trends' instead. |
| 52 | New-referrer discovery | `new-referrers` | 7/10 | hand-code | Anti-join of current referrers against full local referrer history = mechanical backlink discovery; no endpoint or wrapper retains history. | David | Use this command to list referrer domains first seen in the current window (new backlinks/mentions). Do NOT use it for overall referrer rankings; use 'seo' or the generated 'websites metrics' with type=referrer instead. |
| 53 | Month pacing | `pace` | 6/10 | hand-code | Live MTD projection vs prior full month summed from local snapshots — mixes live + local data no single call provides. | David, Inès | Use this command for a month-to-date projection against the prior full month. Do NOT use it for comparing two completed periods with growth %; use the generated 'websites stats' with --compare instead. |

Killed candidates (audit): decay (→movers), utm-audit (→seo), spam (unverifiable), compare-sites (→portfolio), funnel-diff (new table + per-experiment cadence), chart (no decision), records (trivia). Full audit trail: `2026-07-07-191920-novel-features-brainstorm.md`.
