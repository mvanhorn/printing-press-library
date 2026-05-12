# AppsFlyer CLI Brief

## API Identity
- **Domain:** mobile measurement & attribution (MMP). The data layer behind paid-acquisition decisions on iOS/Android.
- **Users:** growth/UA analysts, mobile marketers, performance-marketing engineers, ad-spend reviewers, SKAN ops.
- **Data profile:** time-series KPIs (installs, revenue, ROAS, retention, CPM/eCPI) sliced by app × media-source × campaign × geo × event. Cohort retention/ROAS by aging window. SKAN postback windows (pb_0/pb_1/pb_2). Raw per-event CSV (installs, in-app events, uninstalls).

## Reachability Risk
- **Low.** AppsFlyer V2 API is documented, account-token authenticated, and has stable public hostname `hq1.appsflyer.com`. No bot-protection on programmatic endpoints — token auth gates everything.
- Rate limits are the real risk: 120 calls/day per account for ≥3-day windows; 24 calls/day per app. CLI must surface limiter state.
- China data residency uses a separate stack (`appsflyer-cn.com`) for SDK ingestion only; pull-API hostnames for EU/CN tenants are not surfaced in published docs (browser-sniff TODO for those tenants; default `hq1.appsflyer.com`).

## Top Workflows
1. **Daily ROAS roll-up across all apps** — D-1 master report joined with cost, ranked by media source. Morning briefing fuel.
2. **Partial-cohort LTV monitoring** — D1/D3/D7 cohort curves with `partial_data=true` to watch cohorts forming, flag divergence.
3. **SKAN reconciliation** — compare install-date vs postback-arrival-date SKAN reports against MMP attribution; detect null-CV spikes.
4. **Multi-app portfolio bird's-eye** — same KPI across N app_ids in one sweep (API limit: 10/call), rate-limit aware fanout.
5. **Raw-event pulls** — installs / in-app events CSV for retention modelling, fraud QA, ad-hoc analysis.

## Table Stakes (must absorb)
- Aggregate Pull API V2: partners_report, partners_by_date_report, daily_report, geo_report, geo_by_date_report
- Master API V2: combined dimensions
- Cohort API V1 (POST): D1/D3/D7/D30/D90 LTV, with partial-data handling
- Raw Data Pull API V2: installs_report, in_app_events_report, uninstall_events_report, organic_installs_report, organic_in_app_events_report
- SKAdNetwork API V1: aggregated SKAN, install-date and postback-arrival-date variants
- Cost integrations list (147 sources), ad-revenue integrations list (24 sources)
- Apps list, app settings, users, audiences, OneLink details
- Doctor: token validity, per-endpoint permission check (different scopes per subscription)

## Data Layer
- **Primary entities:** apps, media_sources (canonical `_int` ID), campaigns, daily_metrics (date × app × source × campaign × metric), cohort_rows (date × app × source × cohort_window × metric, with partial_data flag), skan_rows (date × app × postback × kpi), audiences, raw_events (per-install/per-event).
- **Sync cursor:** date-based per-report (last successful end_date by (app_id, report_type, region)).
- **FTS/search:** campaigns by name (filter strings), media-source friendly→canonical lookup, raw-event search by event name / appsflyer_id.

## Codebase Intelligence
- **Existing Go libs are sparse:** `Kachit/appsflyer-sdk-go` covers only raw-data reporting (v0.0.2, Sept 2020). No Go coverage of Master, Cohort, or SKAN. This CLI fills the gap.
- **Python:** no official AppsFlyer data-API lib; community ports are stale and partial.
- **MCP servers:** Official AppsFlyer BETA MCP (loaded in this session — strong ground truth); community `ysntony/appsflyer-mcp` is aggregate-only with 2 tools. Our CLI's MCP surface should match-and-beat the official BETA in coverage while remaining offline-friendly via local SQLite.
- **Singer/Airbyte/Fivetran:** all target nightly EL → warehouse. Our CLI's niche is **interactive, agent-native, ad-hoc** — analyzing one question across N apps without a warehouse round-trip.

## Auth
- **V2 token, account-level**, long-lived (JWT-shaped, >700 chars).
- Header: `Authorization: Bearer <token>`.
- Env var: `APPSFLYER_API_TOKEN`.
- **Hard requirement:** load from `~/.config/appsflyer-pp-cli/.env` via `joho/godotenv`. Process env wins over file values. Standard env-only loading is insufficient per catalog notes — this is a real-user UX requirement.
- Token scopes vary by subscription. `doctor` must probe each report family and report which return 403 (subscription/permission gap, not a bug).

## Rate Limits (documented)
- ≤2-day date ranges: 1 call/min per app per report
- ≥3-day date ranges: 120 calls/day per account, 24 calls/day per app
- Cohort cost refresh: ~every 4h (6×/day)
- Master API D-1 closes at ~3pm next day app-tz (freshness lag)

## Real Pain Points (cited)
- "Restricted" media-source rows return blank fields — silent attribution holes; doctor should surface.
- Partial-data dashes confuse re-queries — historical totals shift; CLI should flag rows with `partial_data=true`.
- Multi-app portfolios blow rate limits fast — limiter awareness is table stakes.
- Master API D-1 freshness lag — morning-pull commands need a `--wait-for-close` or `--accept-partial` option.

## Source Priority
- Single source — AppsFlyer V2 API. No combo CLI ordering required.

## Product Thesis
- **Name:** `appsflyer-pp-cli` (binary), `appsflyer` in the library.
- **Why it should exist:**
  > AppsFlyer isn't an attribution dashboard — it's a paid-channel control surface. The CLI turns it into a local-cached, agent-driven decision tool: every campaign-day-source row is a real-time bid, every cohort row is a payback forecast. Sync once, query offline, fan out across apps, surface partial data honestly, and beat the rate limiter.

## Build Priorities
1. **Foundation** — config (with dotenv via godotenv), region routing, Bearer auth, AdaptiveLimiter wired in client, doctor with per-endpoint permission probe.
2. **Absorb (Pull/Master/Cohort/SKAN/Raw)** — every report family, with friendly + canonical source IDs, partial-data flagging, fanout across app_ids.
3. **Transcend (local SQLite + multi-app intelligence)** — sync into local store, FTS over campaigns, multi-app SQL queries, partial-cohort drift detection, rate-limit budget planner, SKAN-vs-MMP reconciliation, "next-dollar" recommender.

## Browser-sniff TODO (not blocking)
- EU/CN pull-API hostnames (not in published docs)
- Exact request-body schema for Master API V2 `/v6/` (groupings, KPIs, filters)
- Cohort API allowlist for groupings, KPIs, filters
- Raw-data `additional_fields` allowlist (~70 optional fields)

These are deferrable — generation can proceed with documented endpoints and the MCP-derived enums; missing optional fields can be patched after sniffing live API responses during Phase 5 (with the user's token).
