# YouTube Printing Press Brief

**Run:** 2026-05-17 18:50
**Target:** `youtube-pp-cli` — combined wrapper over YouTube Data API v3 + YouTube Analytics API v2
**End user:** Ale Gabayet (parenting / maternity creator)

## API Identity
Two Google APIs fused under one binary:
- **YouTube Data API v3** — channels, videos, playlists, comments, search, captions metadata. Public reads via `YOUTUBE_API_KEY`; owner-only mutations + private fields via OAuth2.
- **YouTube Analytics API v2** — `reports.query` for time-series and grouped metrics (views, watchTime, averageViewDuration, averageViewPercentage, cardClickRate, impressions, ctr, subscribersGained/Lost). OAuth2 required (read-only `yt-analytics.readonly`).
- **Transcripts:** scraped via the unofficial `/api/timedtext` endpoint (no auth, no quota); fallback to `yt-dlp --write-auto-sub` when blocked.

## Reachability Risk — PASS
Both YAML specs already downloaded (Data v3 + Analytics v2). Live spot-check:
- Data v3 reachable at `https://www.googleapis.com/youtube/v3/` (HTTPS, OpenAPI discovery doc public).
- Analytics v2 reachable at `https://youtubeanalytics.googleapis.com/v2/`.
- Transcript scrape path stable since 2020 (`youtube-transcript-api` still ships weekly).

## Top Workflows (Ale's actual asks)
1. Channel health snapshot (views, watch time, sub delta) for arbitrary windows.
2. Top videos by retention, CTR, average view duration.
3. Side-by-side performance comparison across N videos.
4. Decay detection — videos whose recent perf trails their own baseline.
5. Offline FTS5 search across titles, descriptions, tags, **and transcripts**.
6. Comment mining — recurring themes, FAQ surface, sentiment hot-spots.
7. Competitor parenting-niche scan — upload cadence, theme overlap.

## Table Stakes (must work day 1)
- `auth login` (OAuth installed-app, refresh token cached at `~/.config/youtube-pp-cli/token.json`).
- `channels get --mine`, `videos list --mine`, `videos get <id>`.
- `analytics report --metric ... --start ... --end ... --dimension ...`.
- `comments threads --video <id>`, `search list --q ...`.
- `sync videos|comments|analytics` → local SQLite.
- `sql`, `search` (FTS5), `--json --select`, `--agent` everywhere.

## Data Layer
- **SQLite** at `~/.local/share/youtube-pp-cli/store.db` (override via `YOUTUBE_PP_CLI_DB`).
- Tables: `channels`, `videos`, `video_stats_daily`, `analytics_rows`, `comments`, `transcripts`, `competitors`, `sync_runs`.
- FTS5 virtual tables: `videos_fts(title, description, tags)`, `transcripts_fts(text)`, `comments_fts(text)`.
- Daily snapshot tables enable decay/velocity math without re-querying Analytics.

## Source Priority
1. **YouTube Data API v3 OpenAPI** (Google Discovery doc → already converted to YAML). Authoritative for resource shapes, enums, required fields.
2. **YouTube Analytics API v2 OpenAPI** (Discovery doc). Authoritative for `reports.query` parameter matrix and metric/dimension validity.
3. **Transcript endpoint behavior** — empirical (no spec). Pinned via integration tests against 3 known-good public video IDs.

## Product Thesis
The market has dozens of YouTube "analytics dashboards" but no terminal-native, agent-first CLI that **(a)** unifies Data + Analytics + transcripts in one SQLite store, **(b)** exposes every endpoint as both CLI + MCP tool, and **(c)** computes derived metrics (decay, retention leaderboard, theme mining) locally so an agent can compound queries without burning the 10K quota. Existing tools (`ytapi`, `ytstudio-cli`, MCP servers, npm `youtube-transcript`) each cover one slice — none cover all three plus offline compounding.

## Build Priorities
1. Generator emits both APIs from the two specs with shared auth resolver (key-or-OAuth per endpoint).
2. `sync` covers videos (mine + competitor IDs), per-video daily stats (Analytics), comments (top-level + replies), transcripts (scrape).
3. Novel hand-coded: `decay`, `retention-leaderboard`, `theme-mine`, `competitor-diff`, `transcript-search`, `comment-faq`, `posting-cadence`, `sub-velocity`, `ctr-cohort`, `topic-cluster`.
4. Agent surface: `--json --select` works on every list/get; MCP walker exposes everything; novel reads carry `mcp:read-only=true`.
5. Doctor checks: API key present, OAuth token unexpired, quota headroom probe (`channels.list?part=id&mine=true` = 1 unit), transcript endpoint reachable.

## Quota Strategy
**Hard limit:** 10,000 units/day per project, non-negotiable without a Google form approval (weeks of latency, often denied for hobby projects).

**Cost cheatsheet:**
- `videos.list` (any parts), `channels.list`, `playlists.list`, `playlistItems.list`, `commentThreads.list`, `captions.list` → **1 unit** per page, up to 50 IDs per call.
- `search.list` → **100 units** per page. **This is the killer.**
- `videos.insert` → 1,600 units. Out of scope (no upload commands shipped).
- Analytics v2 `reports.query` → **does NOT count** against the Data v3 quota (separate Analytics quota, 720 queries/min/user; effectively unlimited for one creator).

**CLI rules baked in:**
1. **Never** call `search.list` when the user has a channel ID, video ID, or playlist ID — `videos.list?id=` and `channels.list?id=` are 100× cheaper.
2. `sync videos --mine` walks `playlistItems.list` of the uploads playlist (1 unit/page of 50), then hydrates with `videos.list?id=` batches of 50 (1 unit each). A 1,000-video channel costs ~40 units, not 2,000.
3. `search` (the CLI command) hits the **local FTS5 index**, not the API. The Data v3 `search.list` endpoint is exposed but flagged in help as "100 units/call — prefer offline `search` after `sync`".
4. Daily-snapshot tables mean repeated decay/velocity questions cost 0 quota after the morning sync.
5. `doctor` runs a 1-unit probe and warns if `<2,000` units of headroom remain (Google does not expose a quota-remaining header; we estimate from local request log).
6. Transcripts and competitor metadata sync respect the same `--limit` defaults; `--all` requires explicit opt-in.

Result: a parenting creator with ~300 own videos + 10 tracked competitors syncs in well under 500 units/day, leaving 9,500+ units for ad-hoc queries.
