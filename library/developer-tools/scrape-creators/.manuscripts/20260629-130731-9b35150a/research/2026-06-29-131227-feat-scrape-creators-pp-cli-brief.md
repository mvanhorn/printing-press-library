# Scrape Creators CLI Brief (Reprint, June 2026)

## API Identity
- Domain: Public social-media data extraction across **28 platforms** (TikTok, Facebook, YouTube, Instagram, GitHub, LinkedIn, Twitter/X, Spotify, Reddit, Threads, Rumble, Twitch, Pinterest, Google, TruthSocial, SoundCloud, Kwai, Bluesky, Snapchat, Kick, Amazon, plus link-in-bio resolvers Linktree/Komi/Pillar/Linkbio/Linkme).
- Users: growth marketers, influencer/agency analysts, brand competitive-intel teams, content researchers building RAG/LLM corpora, and AI agents that need on-demand social data.
- Data profile: creator profiles + follower/view stats, posts/videos/reels/shorts + engagement, comments + replies, transcripts (YouTube, TikTok, Instagram, Facebook, LinkedIn, Rumble), ad-library creatives (Facebook, TikTok, Google, LinkedIn), trending songs/hashtags/feeds, audience age-gender detection, account credit/usage.
- Auth: API key in `x-api-key` header. **Canonical env var: `SCRAPECREATORS_API_KEY`** (one word "scrapecreators", confirmed from the official CLI README). The prior CLI shipped the slug-derived `SCRAPE_CREATORS_API_KEY_AUTH`; the reprint sets the canonical name as primary (slug-derived retained as fallback).
- Base URL: `https://api.scrapecreators.com`.
- Spec: OpenAPI 3.1.0, freshly re-pulled from `https://docs.scrapecreators.com/openapi.json` — **164 endpoints, all GET (read-only)**. No writes.

## Reachability Risk
- **Low-to-moderate, recurring.** API itself is live ("all systems operational" 2026-06-29) and not Cloudflare-gated to callers — auth is one header, no caller rate limit (vendor advises < 500 concurrent). BUT the status history shows recurring endpoint-specific incidents driven by upstream platform changes and proxy-provider blocking: Jun 28 (IG post + TikTok reels search), Jun 21 (multi-endpoint proxy failure), May 18 (Reddit down). Reddit/YouTube/TikTok-Shop are the historically flaky surfaces. This is per-endpoint flakiness, not a build blocker.
- Infra: AWS Lambda (since Feb 2026) → **hard 29s per-request timeout**. CLI default request timeout must respect this. AI transcript processing capped at source videos ≤ 2 min.
- Pricing: credit-based pay-as-you-go (1 request = 1 credit; credits never expire; no subscription). **Depleted credits return HTTP 402** — needs credit-aware error handling. Free-credit count is disputed (1000 marketing vs 100 reported); do NOT hardcode a free-credit claim in user copy.

## Top Workflows
1. **Competitor ad monitoring** — track a brand's live ad creatives + spend signals across Facebook, TikTok (new Jun 2026), Google, and LinkedIn ad libraries; snapshot over time to diff new vs. disappeared creatives.
2. **Influencer discovery & vetting** — profile + follower/engagement pulls across IG/TikTok/YouTube; IG profile-discovery & hashtag search; age-gender detection for audience verification.
3. **Transcript mining for content research** — batch-fetch transcripts (YouTube, TikTok, Instagram, Facebook, LinkedIn, Rumble) for keyword search, topic analysis, and RAG ingestion. Highest compounding value: transcripts are 1 credit each and infinitely re-queryable once stored.
4. **Trend tracking** — TikTok search-suggestions/autocomplete, IG trending reels, YouTube paid-promotion filter, TikTok Ad Library top ads; follow a hashtag/song/topic as it moves across platforms.
5. **Follower / view growth tracking** — repeated profile pulls snapshotted over time (Facebook exact `view_count` + `talkingAboutCount`, Twitch `isLive`/viewers, follower counts) to compute growth deltas.
6. **Cross-platform creator footprint** — given one handle, find which of 28 platforms the creator is on with follower counts side-by-side.

## Table Stakes (competitor features to match)
- **Official `@scrapecreators/cli`** (JavaScript, 4★, github.com/ScrapeCreators/scrapecreators-cli) — **the bar to beat.** `scrapecreators <platform> <action>`, `list`, `auth login/status/logout`, `balance`, `config set/get/list`, `agent add <target>`, interactive mode, doubles as MCP server, compact-JSON default, 110+ endpoints. Auth precedence: `--api-key` → stored config → `SCRAPECREATORS_API_KEY` env. **Has no local persistence, no offline search, no snapshot/diff, no SQL.**
- **n8n community node** (TypeScript, 19★, adrianhorning08/n8n-nodes-scrape-creators) — n8n integration, API-token credential.
- **Official hosted MCP** (`https://api.scrapecreators.com/mcp`) — OAuth + API-key, 40+ agents. The de-facto agent integration today.
- No dedicated Python/PyPI SDK or standalone Claude plugin found.
- Positioning-only competitors (not wrappers): Apify actors, Bright Data, HikerAPI, Outscraper.

## Data Layer
- Primary entities: `creators` (cross-platform, keyed by handle + platform), `content` (videos/posts/reels/shorts + engagement), `comments`, `transcripts` (FTS5 full text), `ads` (Facebook/TikTok/Google/LinkedIn ad library), `trends` (songs/hashtags/feeds), `usage_log` (per-command credit accounting).
- Sync cursor: per-creator-per-platform timestamp; per-trend / per-ad snapshot timestamp.
- FTS/search: creators (handle, name, bio), content (title, description, captions), transcripts (full text), trends (name).
- **Credit-metered → no auto-refresh cache.** Every endpoint costs a credit; a pre-read auto-refresh would silently burn the user's balance. Sync is manual/explicit; `doctor` reports cache state. (This is the correct application of the accepted "sync enrichment": classify syncable resources well, do NOT enable pre-read cache refresh.)

## Source Priority
- Single source (Scrape Creators API). No combo ordering.

## User Vision
- Reprint motivation: lift the CLI from Printing Press 3.2.1 onto 4.27.0. Concrete improvement targets from the prior scorecard (A, 89%): MCP surface (surface_strategy 2/10, tool_design 5, remote_transport 5, token_efficiency 7) and data_pipeline_integrity (7). Both accepted as spec enrichments.
- MCP enrichment applied in-spec: `x-mcp` Cloudflare pattern (`transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden`) — the right shape for a 164-endpoint surface.
- Sync enrichment applied as correct syncable-resource classification, NOT cache auto-refresh (credit-metered API).

## Product Thesis
- Name: `scrape-creators-pp-cli` (display: **Scrape Creators**).
- Why it should exist: the official CLI is a thin per-request shell-out with no state. Our differentiator is **a local SQLite compounding store + cross-platform joins + agent-native dual surface**:
  - Local FTS5 store across creators/content/transcripts/comments/ads — turn 1-credit-per-call transient data into a queryable, diffable corpus without re-burning credits.
  - Cross-platform compound commands the API alone can't answer (presence matrix, trend triangulation, unified ad search).
  - Credit-burn instrumentation (`account budget` projects runway from local `usage_log` fused with the API's own usage endpoints), 402-aware errors, 29s-Lambda-aware timeout defaults.
  - Read-only across all 164 endpoints → safe agent-native default; every command an MCP tool with `mcp:read-only`.

## Build Priorities
1. Cross-platform sync + SQLite store (creators/content/transcripts/comments/ads/trends/usage_log).
2. All 164 endpoints as typed CLI commands (generator-emitted) + matched official-CLI framework commands (auth, balance, config-equivalent).
3. Cross-platform compound commands (presence, comparison, transcript FTS, unified ad search — now Facebook+TikTok+Google+LinkedIn, Reddit ad library dropped).
4. Credit-burn `account budget` projection; 402 handling.
5. Trend triangulation / delta across platforms.
6. Link-in-bio unified resolver (linktree/komi/pillar/linkbio/linkme).

## Reprint Deltas vs Prior CLI
- Spec grew 114 → 164 endpoints; 23 → 28 platforms. New: GitHub, Spotify, SoundCloud, Kwai, Rumble, FB Marketplace, TikTok Ad Library.
- **Reddit Ad Library deprecated/removed** — `ads search`/`ads monitor` must span Facebook + TikTok + Google + LinkedIn (drop Reddit).
- Env var corrected to canonical `SCRAPECREATORS_API_KEY`.
- 3 prior patches carried as watch-list: auth set-token credential field, store typed-table column dedupe, Go 1.26.4 vuln floor.
- Creator attribution permanent: Adrian Horning (@adrianhorning08).
