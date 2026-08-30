# OpenAI Ads API CLI Brief

## API Identity
- Domain: Advertising / campaign management (ChatGPT Ads, Ads Manager Beta)
- Base URL: https://api.ads.openai.com/v1
- Spec: vendor OpenAPI 3.1.0, `info.version` 2.3.0, 41 endpoints, 75 schemas
- Source: https://developers.openai.com/ads/openapi.json
- Users: advertisers and agencies buying CPC placements inside ChatGPT; API partners configuring client accounts
- Data profile: campaign > ad_group > ad hierarchy; insights time series; custom audiences; conversion pixels/events; creative files

## Reachability Risk
- None for the API. `api.ads.openai.com` answers plain HTTPS with well-formed JSON errors.
- The *console* `ads.openai.com` is Cloudflare-managed-challenge gated (403, `cf-mitigated: challenge`).
  This is irrelevant to the CLI: the API host is separate and unprotected.
- NOTE: `probe-reachability` returned a false-negative `standard_http` for the console by treating a
  200-served Cloudflare interstitial as clean. Recorded as a machine-gap candidate for retro.

## Auth (TWO credential classes — a real user trap)
- **Ads API key** — Settings tab of Ads Manager, scoped to one ad account. Env: `OPENAI_ADS_API_KEY`.
  Bearer. Verified live: `GET /ad_account` -> 200.
- **Conversions API key** — Conversions tab, scoped to ONE conversion event in a data source.
  Minted via `POST /conversions/api_keys`. Verified live: returns
  `403 "Unauthorized to read ads data."` on `GET /ad_account`.
- Tier/permission hints from 4xx body: "403: Unauthorized to read ads data." (conversions key on ads read)
  and "Missing or invalid SDK key in Authorization header." (no key).
- Operator hit this exact confusion during this run. `doctor` must name WHICH key class is missing
  and WHICH Ads Manager tab issues it.

## Live account shape (verified this run)
- 1 campaign (CPC, daily cap), 1 ad group (fixed_bid), 1 ad (chat_card creative, review approved)
- 0 custom audiences, 0 conversion event settings, insights count 0 (campaign hours old)
- Account currency MXN, tz America/Monterrey, account review status `in_review`

## Spec fidelity
- No spec-vs-reality discrepancies found. `/geo_lookup/search` correctly declares required param `q`
  (an earlier probe using `query` was operator error, not a spec defect).
- `q` is a one-letter wire name and is a public-param-audit candidate: expose as `--query` with `q` alias.

## Top Workflows
1. Read the whole account tree and current status in one shot (campaigns -> ad groups -> ads)
2. Pull insights and understand pacing/spend against budget
3. Create/update campaign + ad group + ad, then activate or pause
4. Upload creatives and attach them to ads
5. Configure conversion measurement (pixels, event settings) and send events

## Table Stakes (from the only competitor)
- HYPD-AI/openai-ads-mcp: 11 read-only MCP tools over 5 endpoints. No writes, no audiences,
  no conversions, no upload, no geo lookup, no bulk, no product feeds, no persistence.

## Data Layer
- Primary entities: ad_account, campaigns, ad_groups, ads, custom_audiences,
  conversion_event_settings, insights (time series), geo locations (lookup cache)
- Sync cursor: cursor pagination via `limit` / `after` / `before` / `order`; `has_more`, `last_id`
- FTS/search: campaign+ad group+ad names, descriptions, creative title/body/target_url

## Why this CLI should exist
The API is per-entity and point-in-time. It cannot join across the hierarchy, cannot diff over
time, and returns every monetary value in micros. A local SQLite mirror makes pacing, drift,
fatigue, structural audit, and coherence checks possible — none of which any endpoint offers and
none of which the single existing tool attempts.

## Product Thesis
- Name: openai-ads-pp-cli
- Thesis: the first *writable* OpenAI Ads client, and the only one with local history —
  turning a point-in-time REST surface into an account you can audit, pace, and diff.

## Build Priorities
1. Data layer for full hierarchy + insights snapshots
2. All 41 endpoints as typed commands (the competitor has 5)
3. Transcendence commands built on the local store
