# Umami CLI Brief

## API Identity
- Domain: Web analytics (open-source, privacy-first Google Analytics alternative). Self-hosted + Cloud.
- Users: developers/agencies self-hosting analytics for client sites; indie hackers; privacy-conscious teams. The requesting user runs a self-hosted **v3** instance (umami.davidbarbier.com) with **10 websites** (client sites: restodom.fr, davidbarbier.com, cabinet-echinard.fr, aurarios.fr, dindonstudio.fr, …) and 5+ teams — an agency/portfolio profile.
- Data profile: time-series (pageviews/sessions series), top-N metrics (path, referrer, browser, os, device, country, event, channel…), sessions with activity trails, custom events + event-data properties, saved reports (funnel, retention, utm, goal, journey, revenue, attribution, breakdown), realtime (last 30 min), websites/teams/users admin.

## Reachability Risk
- **None.** Live instance probed with the user's admin credentials: `POST /api/auth/login` → 200 + JWT; `/api/websites` → 10 sites; `/stats`, `/metrics?type=path|referrer`, `/metrics/expanded`, `/active`, `/api/config`, `/api/links`, `/api/heartbeat` all 2xx.
- Probe-safe endpoint used: `GET /api/heartbeat` (public) and `GET /api/websites` (authed, read-only).
- **Version pin risk (important):** the instance is **v3** (confirmed: `type=path` works, `type=url` → 400; `/api/config`, `/metrics/expanded`, `/api/links` exist). Community wrappers mostly target v2; v2→v3 renamed filters (`url`→`path`, `host`→`hostname`) and report types (`insights`→`breakdown`, `goals`→`goal`). The CLI targets **v3 self-hosted**, with Cloud (`x-umami-api-key`, base `https://api.umami.is/v1`) as a secondary auth mode.
- Quirk observed live: `/metrics/expanded` returns `pageviews`/`totaltime` as JSON **strings**, `visitors`/`visits`/`bounces` as numbers → decode with tolerant number extraction.

## Top Workflows
1. **Scheduled/scripted traffic reports** — daily/weekly digest per site: stats + top pages + top referrers + comparison vs previous period (official guide: automate-reporting-with-api; n8n template #2520; Thunderbottom/umami-alerts).
2. **Multi-site portfolio sweep** — "how are ALL my sites doing?" No rollup exists in Umami UI (issues #3362, #3174, #3193); users with 10+ sites are explicitly underserved.
3. **SEO / acquisition analysis** — top referrers, channel breakdown (organic search vs direct vs social), UTM campaign report, entry pages, Google share. (User's own context: SEO reporting for client sites; restodom.fr gets 2161/2811 visits from google.com.)
4. **Custom event & funnel analysis** — event series, event-data properties, funnel/goal/journey/retention reports.
5. **Data export/backup** — CSV export missing in self-hosted UI (issues #946, #2456, #3208); API `/export` + sessions/events pagination is the escape hatch.

## Table Stakes
- Full v3 endpoint coverage: websites CRUD, stats/pageviews/metrics(+expanded), sessions (+stats/weekly/detail/activity/properties), events (+series), event-data (events/fields/properties/values/stats), session-data, reports CRUD + 8+ runners, realtime, users/teams/me/admin, links/pixels/segments (v3), send/batch ingestion, share tokens.
- Both auth modes: self-hosted login (username/password → JWT, auto-refresh before expiry — only mikusnuz MCP does this today) and Cloud API key.
- Rich filter flags on every stats command (path, referrer, country, browser, os, device, event, tag, hostname, utm*…).
- Human date ranges ("7d", "30d", "today", "this-month") → epoch-ms conversion (every wrapper makes users compute epoch-ms by hand).
- `--json`, `--select`, `--csv`, typed exit codes, `--dry-run` (no wrapper has any of this).

## Data Layer
- Primary entities: websites, stats snapshots (per site per day — enables local history/trends), metrics rows (per site/type/period), sessions, events, reports (saved definitions), teams/users.
- Sync cursor: time-windowed (startAt/endAt) per website; daily granularity snapshots.
- FTS/search: websites by name/domain; events by name; sessions by country/browser.
- Local store unlocks: **cross-site rollups** (the #1 community pain), week-over-week deltas without recomputing, offline report history, traffic-drop detection against local baselines.

## Codebase Intelligence
- Ground truth from source (v2.20.2 tag + master/v3 routes): zod-validated query params; startAt/endAt epoch-ms everywhere except `/api/send` payload.timestamp (epoch-seconds); paged envelope `{data, count, page, pageSize}`; filter operator prefixes (`!` neq, `~` contains, `!~` not-contains); special "all time" = `startAt=0&endAt=1`.
- Auth: `Authorization: Bearer <jwt>` from `POST /api/auth/login`; share-token header `x-umami-share-token`; Cloud `x-umami-api-key`. JWT decode-exp + re-login with buffer = best-practice refresh (mikusnuz).
- Rate limiting: Cloud = 50 calls/15s; self-hosted = none built in. **No wrapper handles 429 anywhere** — our AdaptiveLimiter is a differentiator.
- Architecture: Next.js app-router API; relational (Postgres) or ClickHouse backends; series x-values differ by backend ("YYYY-MM-DD HH:mm:ss" vs ISO) → parse both.
- DeepWiki skipped: we already have route-level source ground truth + live-instance probes, which is strictly deeper than wiki summaries.

## User Vision
- (No explicit briefing given — user chose "Let's go".) Context signals: the user is a web developer running client sites; invoked from a project on branch `feat/seo-reporting`; the URL pointed at a client site (restodom.fr). SEO/acquisition reporting and multi-client digests are the likely headline uses.

## Product Thesis
- Name: **umami-pp-cli**
- Why it should exist: Umami has **no functional CLI at all** (official CLI = abandoned 16-line stub since 2022; Plausible/Matomo have none either; only GA4 has a young CLI). Seven MCP servers exist but all are thin API mirrors with no local state, no rollups, no offline story, no rate-limit handling, and mostly stale v2 targeting. A CLI with a SQLite mirror turns the top community pain points (multi-site rollup, CSV export, scheduled digests, period comparison) into single commands — and is agent-native from day one.

## Build Priorities
1. Full v3 typed endpoint surface + dual auth (self-hosted login w/ JWT refresh, Cloud key) + human date-range parsing.
2. SQLite mirror (websites, daily stat snapshots, metrics, events, sessions) with sync → enables portfolio rollup, trends, baselines.
3. Transcendence: portfolio sweep/rollup, SEO digest (channels/referrers/entry pages/Google share), period comparison, traffic-drop watch vs baseline, CSV export, markdown digest for scheduled reports.
4. Report runners (funnel/retention/utm/goal/journey/revenue/attribution/breakdown) as first-class commands.
5. Ingestion (`send`/`batch`) for testing trackers and backfilling.
