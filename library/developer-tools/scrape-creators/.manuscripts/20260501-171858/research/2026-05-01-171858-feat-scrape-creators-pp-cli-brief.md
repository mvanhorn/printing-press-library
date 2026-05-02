# ScrapeCreators CLI Brief (Regeneration, May 2026)

## API Identity
- Domain: Public social-media data extraction across 23+ platforms (TikTok, Instagram, YouTube, Twitter/X, Facebook, LinkedIn, Reddit, Threads, Bluesky, Pinterest, Snapchat, Twitch, Kick, Truth Social, link-in-bio services, Amazon, Google Ads).
- Users: Growth marketers, influencer agencies, content analysts, brand-safety teams, competitive-intel teams, AI/LLM pipeline builders.
- Data profile: Creator profiles + stats, post/video/reel content + metadata, comments + replies, transcripts (TikTok, YouTube, IG, Facebook), trending songs/hashtags/videos, ad-library data (Facebook, LinkedIn, Google), audience demographics, account credits/usage.
- Auth: API key via `x-api-key` header. Env var: `SCRAPE_CREATORS_API_KEY`.
- Base URL: `https://api.scrapecreators.com`.
- Spec: OpenAPI 3.1.0 at `https://docs.scrapecreators.com/openapi.json` — **114 GET endpoints, all GETs (read-only)**. No writes, no auth-required user actions.

## Reachability Risk
- None. Confirmed live: `GET /v1/account/credit-balance` returns 200 with valid JSON. API runs on AWS Lambda; no rate-limit blockers reported. Auth is one header.

## Why we're regenerating
- Prior CLI (v1.3.2, Apr 2026): scoped to 13 TikTok-only endpoints. Spec has since expanded to 114 endpoints across 23 platforms (~9× growth).
- Printing Press itself jumped 1.3.2 → 3.2.1 (transport, scoring, MCP surface, auth modes all evolved). Prior brief's "TikTok-only product thesis" no longer matches the API.

## Top Workflows (informed by new platform breadth)
1. **Cross-platform creator intelligence** — fetch a single creator's footprint across TikTok + IG + YouTube + Twitter in one pipeline (most users live on 3+ platforms).
2. **Ad-library competitive research** — unified search across Facebook, Google, and LinkedIn ad libraries for a brand or competitor.
3. **Trend triangulation** — track a song/hashtag/topic as it moves across TikTok → IG Reels → YouTube Shorts.
4. **Transcript mining** — batch-fetch transcripts (TikTok, YouTube, IG, Facebook video) for keyword search, RAG, topic analysis.
5. **Influencer discovery** — keyword/hashtag/profile search, filter by engagement rate, build shortlists across platforms.
6. **Link-in-bio resolution** — given a Linktree/Komi/Pillar/Linkbio/Linkme URL, resolve all destinations (these 5 services are all in spec).

## Table Stakes (competitor features to match)
- **Phyllo** — 20+ network creator data API (we cover 23+).
- **Bright Data** — multi-platform structured data (Amazon, IG, TikTok, etc.).
- **Apify** — orchestration + actor marketplace; ScrapeCreators ships as Apify actors.
- **HypeAuditor / Modash / CreatorIQ** — creator analytics suites (web UI, not CLI).
- **adrianhorning08/n8n-nodes-scrape-creators** — official n8n integration by the same author (adrian-horning is ScrapeCreators CEO).
- **ScrapeCreators/scrapecreators-cli** — the official CLI (Node-based) with `list / auth / balance / config / agent add`, `--format`, `--clean`, `--output`, MCP integration. **This is the bar to beat.**

## Data Layer
- Primary entities: `creators` (cross-platform, keyed by handle + platform), `content` (videos/posts/reels/shorts), `comments` (with thread depth), `transcripts` (FTS-indexed full text), `ads` (Facebook/Google/LinkedIn ad library), `trends` (TikTok songs/hashtags), `usage_log` (per-command credit accounting).
- Sync cursor: per-creator-per-platform ISO timestamp; per-trend snapshot timestamp.
- FTS/search: creators (handle, name, bio), content (title, description, captions), transcripts (full text), trends (name).

## Product Thesis
- Name: `scrape-creators-pp-cli`.
- Why it should exist: There is already an official Node CLI. Our differentiator is **offline state + cross-platform joins + agent-native semantics**:
  - Local SQLite store with FTS5 across creators, content, transcripts, comments — no other ScrapeCreators tool persists data for re-querying without re-burning credits.
  - Cross-platform compound commands the API alone cannot answer ("which platforms is @mrbeast on", "what hashtag is rising fastest across TikTok+IG+YouTube").
  - Credit-burn instrumentation built-in (every command logs to `usage_log`; `account budget` projects runway).
  - Read-only across the board → safe agent-native default; every Cobra command becomes an MCP tool with `mcp:read-only: true`.

## Build Priorities
1. Cross-platform sync + SQLite store for creators/content/transcripts/comments/ads/trends.
2. All 114 endpoints exposed as typed CLI commands with consistent flag patterns.
3. Cross-platform compound commands (presence, comparison, transcript FTS, ad library unified search).
4. Credit-burn rate tracking and `account budget` projection.
5. Trend triangulation across TikTok / IG Reels / YouTube Shorts.
6. Link-in-bio unified resolver across linktree / komi / pillar / linkbio / linkme.

## User Vision (from invocation context)
- Full regeneration with current Printing Press (3.2.1).
- Validate prior 8 novel features against new multi-platform scope; rationalize keep/extend/drop and add new ideas unlocked by breadth.
- Preserve original copyright attribution: `Copyright 2026 adrian-horning` in NOTICE (adrian-horning is the ScrapeCreators CEO and original CLI author).
