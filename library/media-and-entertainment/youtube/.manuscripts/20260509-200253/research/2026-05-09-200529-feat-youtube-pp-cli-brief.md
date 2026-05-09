# YouTube CLI Brief

## API Identity
- Domain: video platform — videos, channels, playlists, comments, captions, livestreams, memberships, trending, categories
- Users: creators (analyze own channel, manage playlists), researchers/analysts (track topics, channels, regional trends), agent builders (summarize/transcribe/search), tooling teams (cache + cost-plan against the 10K-unit/day quota)
- Data profile: high-volume metadata, paginated lists, deep nesting under `snippet`/`statistics`/`contentDetails`, transcripts available out-of-band via yt-dlp

## Spec Source
- OpenAPI 3.0.0 at `https://api.apis.guru/v2/specs/googleapis.com/youtube/v3/openapi.yaml` (~422 KB, 23 resources, ~72 methods)
- Server: `https://youtube.googleapis.com/`
- Auth: `apiKey` (query `key=`) for reads; OAuth2 for writes/private — we will scope MVP to apiKey + OAuth as a planned secondary path (token already harvested or env-supplied)

## Reachability Risk
- **None.** Public, well-documented Google API. Standard 401/403/429 paths. The only friction is the 10,000-unit/day quota and the fact that `search.list` costs 100 units each — we treat that as a feature opportunity, not a risk.

## Top Workflows
1. **Find videos by topic** — search.list → videos.list (stats batch) — quota-expensive without batching/caching
2. **Mirror a channel locally** — channels.list → playlists.list / playlistItems.list → videos.list batched; tracked over time
3. **Pull a transcript for a video** — yt-dlp subtitles path (free of quota); fall back to captions.list/download for owned content
4. **Track trending in a region/category** — videos.list?chart=mostPopular, snapshotted daily for trend analysis
5. **Comment thread analysis** — commentThreads.list paginated, FTS across replies
6. **Plan quota before running** — cost-preview every command; abort if today's budget would be blown

## Table Stakes (must match every competitor)
- All 14 functions from `dannySubsense/youtube-mcp-server` (most comprehensive MCP)
- yt-dlp-style transcript extraction (no quota, captions or auto-captions)
- Trending fetch (any region)
- Comment fetch (any sort, paginated)
- Channel video crawl with pagination
- Video category lookup
- All read endpoints in the OpenAPI spec (videos, channels, playlists, search, comments, commentThreads, captions, activities, subscriptions, members, etc.)

## Data Layer
- Primary entities: `videos`, `channels`, `playlists`, `playlist_items`, `comments`, `comment_threads`, `captions`, `categories`, `regions`, `trending_snapshots` (region+date keyed), `transcripts` (video_id keyed, content)
- Sync cursor: `publishedAfter` per channel; ETag on per-resource list calls; `pageToken` chained
- FTS5 indexes: video title+description+tags+transcript; channel name+description; comment text
- Trending: snapshot table keyed `(region, category, captured_at)` so we can diff trending lists across days/weeks without re-querying

## Codebase Intelligence
- Source: `dannySubsense/youtube-mcp-server` README — 14 tools cover the read surface comprehensively (videos, channels, playlists, search, trending, comments, captions, transcripts, evaluation)
- Source: `anaisbetts/mcp-youtube` — transcript path uses `yt-dlp` shelled out; no API key needed for transcripts
- Auth pattern: `key=` query param OR `Authorization: Bearer <oauth-token>` header; env vars `YOUTUBE_API_KEY` and `YOUTUBE_OAUTH_TOKEN` are conventional

## Product Thesis
- **Name:** `youtube-pp-cli` (slug `youtube`)
- **Headline:** Every YouTube Data API v3 endpoint, plus quota-aware planning, plus offline transcripts (no quota), plus an SQLite-backed store so channel/trending analysis costs zero quota after sync.
- **Why it should exist:** Existing tools split the surface — yt-dlp owns downloads, MCP servers own metadata read, no one owns the operator who wants to do all three plus track quota plus query their own cached corpus with SQL/FTS. The Printing Press's local-store + agent-native + dry-run/JSON pattern was made for this.

## Build Priorities
1. **Foundation (Priority 0):** SQLite store for videos/channels/playlists/playlist_items/comments/transcripts/trending_snapshots; FTS5 search; sync workflow with ETag
2. **Absorb (Priority 1):** All read endpoints from the spec + transcript shell-out via yt-dlp; OAuth-gated writes (insert/update/delete) for playlists/comments/videos with `--dry-run` first
3. **Transcend (Priority 2):** quota cost preview, trending diff, channel time-series, transcript FTS across many videos, comment-sentiment surfacing, "what should I watch from channel X today" digest, watch-later ingest
