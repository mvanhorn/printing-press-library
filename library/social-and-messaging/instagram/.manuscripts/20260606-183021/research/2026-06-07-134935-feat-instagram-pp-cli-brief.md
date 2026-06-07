# Instagram CLI Brief

## API Identity
- Domain: Instagram **Graph API** (Business/Creator analytics via linked Facebook Pages). Base `https://graph.facebook.com/v22.0/`. **Facebook-Login path** (NOT the Instagram-Login `graph.instagram.com` path) — chosen because it is the only path with `business_discovery`, hashtag search, system-user tokens, and central multi-account management.
- Users: Multi-brand social/marketing managers running several owned Business/Creator accounts (multiple owned brand accounts). Analytics-first, not publishing-first.
- Data profile: Per-account profile counts (followers/follows/media), account insights (reach, views, interactions, demographics), per-media insights (reach/views/saves/shares/watch-time), comments, hashtag performance, competitor public metrics. All keyed by an `{ig-user-id}` per brand.

## Reachability Risk
- **None.** `graph.facebook.com` is a standard token-authenticated REST/JSON API — no Cloudflare/bot challenge, no JS rendering, `standard_http`. The only gates are the OAuth token and Business-Use-Case (BUC) rate limits (handled via `x-business-use-case-usage` header). This is the *opposite* of scraping public profiles.
- Ecosystem-breakage risk: real but well-documented. The Jan + Apr 2025 deprecations (`impressions`, `video_views`, `plays`, legacy `audience_*`/`profile_views`/`follower_count` metrics) broke many wrappers. Mitigation baked into the spec: pin **v22.0**, use `views` everywhere (replaces impressions/plays/video_views), `metric_type=total_value` for account metrics, lifetime `follower_demographics`, and gracefully skip metrics that 400 for a given media type.

## Auth
- Type: **bearer_token** (Meta access token). Sent as `?access_token=<token>` query param (Bearer header also works).
- Env var: `INSTAGRAM_ACCESS_TOKEN` (canonical). No token in user env at run time → live Phase 5 deferred; CLI built + verified against mocks.
- Scopes: `instagram_basic`, `instagram_manage_insights`, `pages_show_list`, `pages_read_engagement`, `business_management`.
- Token types: short-lived (~1h), long-lived user (~60d), **system-user (non-expiring)** — recommended for unattended multi-brand CLI. Inspect via `GET /debug_token`.
- Getting the IG user id: `GET /me/accounts` → page → `GET /{page-id}?fields=instagram_business_account`.

## Top Workflows
1. **Cross-brand engagement comparison** — loop accounts, pull `reach`+`total_interactions` (period=day, total_value, since/until), compute ER per brand. (No native cross-account view exists anywhere — this is THE differentiator.)
2. **Top posts by reach/engagement (last 30d)** — page `/media`, fetch per-media `views,reach,total_interactions,saved,shares`, rank.
3. **Follower growth WoW** — snapshot `followers_count` + `follows_and_unfollows` per account on a schedule; diff over time (requires local store).
4. **Best-time-to-post** — bucket each post's engagement by weekday/hour of `timestamp` (Graph has no native best-time; derive locally).
5. **Hashtag performance** — `ig_hashtag_search` → `top_media`/`recent_media` (30 tags / 7d cap).
6. **Audience demographics** — `follower_demographics` (lifetime) with `breakdown=age|gender|city|country`.
7. **Reel/Story performance** — Reels watch-time (`ig_reels_avg_watch_time`, `ig_reels_video_view_total_time`); Stories within 24h.
8. **Competitor benchmarking** — `business_discovery.username(...)` for rivals' public followers/media/engagement.

## Table Stakes (from Iconosquare/Metricool/Later/Hootsuite/Sprout)
- Engagement-rate calculation (document denominator: followers vs reach), follower-growth tracking, hashtag-performance ranking, post-performance ranking, best-time-to-post, audience demographics, competitor benchmarking (≈5 competitors), story/reel completion + watch-time.

## Data Layer
- Primary entities: `accounts` (brands/ig-users), `media` (posts/reels/stories), `media_insights`, `account_insights_snapshots` (time-series for growth/WoW), `comments`, `hashtags` + `hashtag_media`, `competitors` (business_discovery snapshots).
- Sync cursor: media via cursor pagination (`paging.cursors.after`); account-insights snapshots are date-stamped rows (the local store IS the time-series, since Graph only returns short windows).
- FTS/search: captions, comment text, hashtag names.
- **Why a store matters here:** Graph API returns only short rolling windows and no cross-account view. Follower growth WoW, best-time-to-post, and historical trends are *only possible* with a local snapshot store. This is the moat.

## Codebase Intelligence
- Closest Go prior art: `qcserestipy/instagram-api-go-client` (library, not a CLI; covers media + account insights on v24). No mature Go analytics **CLI** exists — clear product gap.
- Analytics-focused MCP references: `BilalTariq01/instagram-analytics-mcp`, `jlbadano/ig-mcp` (uses `instagram_manage_insights`), `AleemHaider/instagram-mcp` (24 tools). Meta-Ads MCPs (`pipeboard-co/meta-ads-mcp`, `serkanhaslak/meta-mcp`) — good cursor-pagination + token-redaction patterns.
- Official SDKs: `facebook-nodejs-business-sdk` (JS), `python-facebook-api` / `facebook_business` (Python) — heavy, Marketing-API-centric; expose IGUser/IGMedia insights/media/comments/hashtag/business_discovery.

## User Vision
- User manages multiple owned business/creator IG accounts (multiple owned brand accounts) with admin access. Core need: **analytics across their own accounts**. Cross-account portfolio analysis (compare/rank brands, unified growth view) is the explicit differentiator opportunity — Meta Business Suite shows one account at a time with no comparison, no history, no CLI, no local store.

## Critical metric facts (these drive codegen correctness)
- **`views` replaces deprecated `impressions`/`plays`/`video_views` everywhere** — single biggest breaking change.
- Account insights now need `metric_type=total_value` → read `total_value.value` (+ `breakdowns[]`), NOT legacy `data[].values[0].value`.
- Demographics: lifetime `follower_demographics` / `engaged_audience_demographics` with `breakdown=age|gender|city|country`.
- Reels watch-time metrics valid: `ig_reels_avg_watch_time`, `ig_reels_video_view_total_time`.
- `business_discovery` + `ig_hashtag_search` are Facebook-Login-path only.

## Product Thesis
- Name: **igscope** (binary `instagram-pp-cli`). Tagline candidate: "Every Instagram Business metric across all your brands, with the cross-account history Meta Business Suite never gives you."
- Why it should exist: the only tool that puts *all your brand accounts* in one local, queryable, agent-native store — cross-account comparison, follower-growth history, best-time-to-post, hashtag + competitor benchmarking — none of which the official UI or any CLI offers today.

## Build Priorities
1. Data layer for accounts/media/insights/snapshots + sync + SQL + search (the moat).
2. Absorbed parity: account info, account insights, media list + media insights, comments, hashtag search, business_discovery, token doctor.
3. Transcendence: cross-account compare, follower-growth WoW (snapshot diff), best-time-to-post, top-posts ranking, hashtag-performance, competitor benchmark — the local-store-only features.
