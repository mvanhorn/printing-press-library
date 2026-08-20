# StackAdapt CLI Brief

## API Identity
- **Domain:** StackAdapt programmatic advertising (DSP) — GraphQL API at `https://api.stackadapt.com/graphql`. Manages advertisers, campaigns, campaign groups, ads, audiences/segments, and reporting (delivery/insights), plus brand-lift, footfall, CTV/DOOH inventory.
- **Users:** Programmatic media buyers, ad-ops teams, and agencies running StackAdapt campaigns; their analysts and AI agents who need spend/pacing/performance answers without the dashboard.
- **Data profile:** Campaigns + campaign groups + advertisers (hierarchy), ads, audiences/segments, and time-series delivery/insight reporting (spend, impressions, clicks, CTR, conversions, ROAS, CPA, CPC, CPM). Reporting time-series is the high-gravity data.

## Reachability Risk
- **None.** Live-introspected with the user's GraphQL token: `POST /graphql` → HTTP 200, 64 read queries visible. Bearer auth. Rate-limited (429). Endpoint stable (`api.stackadapt.com`; sandbox `sandbox.stackadapt.com/public`).
- REST API is deprecated (sunset end-2025); this CLI targets **GraphQL** only.

## Top Workflows
1. **"Are my campaigns pacing on budget?"** — budget vs actual spend per campaign/group, flag under/over-pacing. (NOI)
2. **Inventory + status** — list advertisers, campaigns, campaign groups, ads; get one with its config.
3. **Performance reporting** — campaignDelivery / campaignInsight (spend, impressions, CTR, ROAS, CPA) over a window.
4. **Drift detection** — CTR/CPM/spend drift week-over-week (needs local history).
5. **Audience analysis** — segment list + audienceInsights; overlap/cannibalization.

## Table Stakes (absorb from the official TS SDK + reporting connectors)
- `@stackadapt/pa-typescript-sdk` (v0.73.0): campaigns, advertisers, Insights Reporting (getDomainReport, getGeographicReport).
- Supermetrics (45 metrics / 38 dims), Improvado, Fivetran connectors pull: spend, impressions, clicks, CTR, conversions, ROAS, CPA, CPC, CPM, video completion, creative + audience-segment performance, **pacing** — all read. Our CLI matches these as read commands + adds local analysis.

## Data Layer
- **Primary entities:** advertisers, campaigns, campaign_groups, ads, audiences, custom_segments; delivery_daily / insight_daily (the reporting time-series); conversion_paths.
- **Sync cursor:** GraphQL cursor pagination per resource; reporting windowed by date.
- **FTS/search:** across advertisers + campaigns + campaign_groups + ads names.
- Hot tables: `delivery_daily` / `insight_daily` (pacing, drift, reconcile). Index `(campaign_id, date)`.

## Codebase Intelligence
- **Auth:** `Authorization: Bearer <token>`. Env var `STACKADAPT_API_TOKEN`. Separate GraphQL token (REST key won't work). Rate-limited 429.
- **Transport:** GraphQL POST. CLI generated as scaffold (Phase 2) + hand-built commands via a GraphQL client wrapper (Phase 3) — heavy hand-code ratio, like Azure.
- **Surface:** 64 read queries + 98 mutations. READ-ONLY → exclude all 98 mutations.

## User Vision (USER_BRIEFING_CONTEXT)
- Read-only GraphQL CLI, category marketing. Live-test EVERY command with the user's working token throughout (the bar that closed HubSpot #923). Token is a CLIENT account → all read-only, SCRUB all client identifiers (advertiser/campaign/account names + IDs + token) before PR, same as Azure. Beginner-first docs. NOI = pacing & delivery observatory. PR title `feat(stackadapt): add stackadapt` (slug scope per AGENTS.md).

## Product Thesis
- **Name:** `stackadapt` (binary `stackadapt-pp-cli`), category `marketing`.
- **Why it should exist:** Connectors dump StackAdapt data to a warehouse; the dashboard shows it one screen at a time. Nobody answers "which campaigns are under-pacing right now?" or "whose CTR decayed 20% this week?" in one call. Those need local time-series + cross-joins — native to a SQLite-backed agent CLI, impossible stateless. Plus the first agent-native StackAdapt CLI (terminal + skill + MCP).

## Build Priorities
1. **P0:** SQLite data layer (advertisers/campaigns/groups/ads/audiences + delivery/insight time-series); GraphQL client wrapper; sync; search/SQL.
2. **P1 absorb:** read list/get for advertisers, campaigns, campaign-groups, ads, audiences, custom-segments; reporting (campaign-delivery, campaign-insight, advertiser-delivery, audience-insights, conversion-path).
3. **P2 transcend (NOI):** `pacing`, `delivery-drift`, `reconcile`, `audience-overlap`, `bottleneck`, `stale-campaigns`.
4. **Docs:** beginner-first narrative (auth setup, pacing-first quickstart, teaching recipes).

## Source Priority
- Single source (StackAdapt GraphQL). No combo. Multi-source gate skipped.
