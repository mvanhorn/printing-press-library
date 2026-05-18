# YouTube CLI Brief

## API Identity
- **Domain**: Video discovery, channel data, transcripts, playlists, comments
- **Users**: Researchers, content creators, app developers, agents doing video search-and-enrich
- **Data profile**: 32 resources, 38 endpoints in the YouTube Data API v3 OpenAPI spec (apis-guru, 422 KB). Heavy hitters: `search`, `videos`, `channels`, `playlists`, `playlistItems`, `comments`, `commentThreads`, `captions`, `activities`, `subscriptions`. Live-streaming and members-only resources are out of scope for a power-user-with-api-key CLI.

## Reachability Risk
- **None for the official API.** Probe returned HTTP 200 with real video data. Quota is the constraint, not reachability: 10,000 units/day default. `search.list` costs 100 units/call (so ~100 searches/day); `videos.list` costs 1 unit/call.
- **Low for transcripts (timedtext endpoint).** YouTube has aggressively blocked cloud-provider IPs (AWS/GCP/Azure) from the undocumented timedtext endpoint since 2024. From a residential IP (user's home/work machine, target use case), transcripts work reliably. From a server, plan for `RequestBlocked` failures and document the proxy workaround.
- **High for `relatedToVideoId`.** Deprecated June 12, 2023, returns 400. **All "related videos" features in the CLI MUST be rebuilt on topicId/channelId/tag overlap heuristics**, not the deprecated parameter. arXiv paper 2506.04422 documents the research-community workarounds.

## Top Workflows
1. **Photo-keywords → ranked videos** (user's primary). Take search terms (CLI args or stdin), search YouTube, return top N per term with embed URLs and transcripts.
2. **Transcript extraction** for arbitrary public videos (auto-generated or manual captions).
3. **Related-video discovery** despite `relatedToVideoId` deprecation — using topic/channel/tag similarity.
4. **Channel growth tracking** — sync a channel's uploads, diff over time.
5. **Comment thread digest** — top N comments + thread structure for sentiment/research workflows.

## Table Stakes
- Search (with all filter dimensions: type, region, language, duration, definition, publishedAfter/Before, category)
- Videos: list, get details (snippet, statistics, contentDetails, topicDetails, status)
- Channels: list, get details, list uploads via uploads playlist
- Playlists + playlistItems
- Comments + commentThreads
- Captions metadata (`captions.list`)
- Transcripts (via timedtext, not Data API — captions.download requires OAuth-as-owner)
- Activities, subscriptions, video categories, i18n regions/languages

## Data Layer
- **Primary entities** worth a local SQLite table: `videos`, `channels`, `playlists`, `playlist_items`, `comments`, `comment_threads`, `search_results` (with query as the cursor), `transcripts` (separate table keyed by `video_id` + `lang`).
- **Sync cursor**: `publishedAfter` for channel uploads; `pageToken` for resumability. Quota-aware: warn when sync would exceed remaining daily quota.
- **FTS/search**: FTS5 over `videos.title|description`, `channels.title|description`, `transcripts.text`, `comments.text`. Local search is the killer for "find videos I've already seen about X" and "find transcripts mentioning Y across N videos."

## User Vision
- **Workflow**: external image-processing produces search terms → CLI takes those terms (stdin or args) → returns relevant YouTube videos with transcripts, related videos, embed URLs → consumed by a personal webapp the user is building.
- **Auth**: `YOUTUBE_API_KEY` only. No OAuth client ID configured. All read-only public-data operations work; write operations (upload, comment, rate, playlist mutations) are out of scope.
- **Reach**: power-user CLI for personal use AND library for webapp development. Output must be webapp-ready (clean JSON with embed URLs, thumbnail URLs, ISO timestamps).

## Source Priority
- Single source: YouTube Data API v3 (official spec from apis-guru, derived from Google Discovery doc). Transcripts via the unofficial timedtext endpoint as a secondary HTTP surface.

## Product Thesis
- **Name**: `youtube-pp-cli` (binary), brand "YouTube"
- **Why it should exist**: every existing CLI/MCP either (a) requires OAuth for table-stakes operations, (b) has no transcript support, (c) hand-rolls related-video logic poorly, or (d) doesn't compose for the bulk-search-with-transcript-and-embed workflow. None offer a local SQLite store with FTS over transcripts.

## Build Priorities
1. **Generator-emitted resources** (32 resources, 38 endpoints) — sync, search, sql, doctor, and per-endpoint commands for the read-only surface. API-key auth.
2. **Transcripts** (`videos transcript <id>` + `videos transcripts <id1> <id2> ...`) via timedtext, with the IP-blocking caveat documented.
3. **Related-videos rebuild** (`videos related <id>`) using topicDetails + same-channel + tag overlap.
4. **Embed URL helper** baked into every video output (`--embed-url` flag, or always-on field in `--webapp` output mode).
5. **Bulk search from stdin** (`search bulk <stdin>` or `search "term1" "term2" --bulk`).
6. **Webapp output mode** (`--webapp` emits clean JSON: videoId, title, channel, embedUrl, thumbnailUrl, duration_ms, publishedAt ISO, transcript_excerpt).
7. **Transcript FTS** across synced transcripts (`transcripts search "<query>"`).
8. **Search result diff** (`search diff "<term>"` — compare against last cached result).
