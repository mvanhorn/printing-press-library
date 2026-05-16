# YouTube CLI Brief

## API Identity
- **Domain**: Video platform — content management, moderation, analytics, monetization
- **Users**: Creators running channels (solo + small teams), agencies managing multiple channels, internal automation in n8n/Zapier-style stacks
- **Data profile**: Three official Google APIs sharing OAuth2 + the same data lake
  - **Data API v3** (`youtube/v3`, 39 OpenAPI paths) — CRUD on videos, playlists, channels, comments, captions, live broadcasts, subscriptions, members
  - **Analytics API** (`youtubeAnalytics/v2`, 3 paths) — real-time `reports.query` with ~70 dimensions/metrics + `groupItems` for cohorts
  - **Reporting API** (`youtubereporting/v1`, 6 paths) — async bulk daily reports for warehouse-style ingestion
  - **PubSubHubbub** (`pubsubhubbub.appspot.com/subscribe`) — WebSub feed for upload-pings

## Reachability Risk
- **None** — all three APIs are first-party Google endpoints with stable OpenAPI specs from apis-guru (auto-converted from Google Discovery). 100K+ users daily. Not blocked.

## Top Workflows
1. **heldForReview moderation** — pull `commentThreads.list?moderationStatus=heldForReview`, batch-approve/reject + optional `banAuthor` via `comments.setModerationStatus`. Highest-frequency creator pain.
2. **Daily/weekly analytics digest** — quota-cheap Analytics queries (views, watch-time, CTR, top traffic sources, revenue, Shorts vs long-form via `creatorContentType`) — formatted as Markdown for n8n→Email/Slack.
3. **Bulk metadata ops** — re-tag, re-categorize, append description footer, fix typos across back catalog. Pure Data API loop. Use `playlistItems.list` (1 unit) over `search.list` (100 units) to enumerate uploads.
4. **Playlist hygiene** — auto-add new uploads to playlists by tag/title pattern; reorder by view count.
5. **Upload-ping trigger** — PubSubHubbub subscribe so n8n receives webhook on new upload (own channel or competitor) instead of polling.

## Table Stakes (from absorbed competitors)
- All Data API resource CRUD (videos, playlists, channels, comments, captions, channelBanners, watermarks, channelSections, subscriptions, search, liveBroadcasts, liveChatMessages, members)
- Analytics queries with full dimension/metric coverage
- Reporting job create/list/download
- Auth via OAuth2 device flow OR API key (where read-only suffices)
- `--json`, `--select`, `--csv`, `--quiet`, `--dry-run`, typed exit codes
- Offline FTS5 search over synced videos/comments/playlists
- Quota awareness (per-call cost calculation + remaining estimate)

## Data Layer
- **Primary entities**: videos, playlists, playlistItems, comments, commentThreads, channels, captions, subscriptions, liveBroadcasts, members, superChatEvents, analyticsSnapshots, reportingJobs
- **Sync cursor**: per-resource `etag` + `updatedAt`; analytics: per-day snapshot rows
- **FTS5/search**: full-text on video titles + descriptions, comment text, playlist titles

## Codebase Intelligence
- **anaisbetts/mcp-youtube** (popular MCP, ~1K+ stars) — Node, video metadata + transcript pull, yt-dlp wrap. Auth: API key.
- **dannySubsense/youtube-mcp-server** — Python, 14 tools, "technology freshness scoring" content eval.
- **jdwit/ytstudio-cli** — Java/Spring, bulk metadata, Data API only.
- **danvega/youtube-cli** — Java/Spring Shell, channel stats, video listing.
- **mattwright324/youtube-comment-suite** — Java GUI, comment archive across many channels.
- **Auth pattern across all**: OAuth2 refresh token in JSON file at `~/.config/<tool>/auth.json`, scopes vary (`youtube`, `youtube.readonly`, `youtube.force-ssl`, `yt-analytics.readonly`, `yt-analytics-monetary.readonly`, `youtube.channel-memberships.creator`).
- **Quota-conscious patterns**: Use `channels.list?part=contentDetails` to get uploads playlist ID (1 unit), then `playlistItems.list` (1 unit/page) instead of `search.list?forMine=true` (100 units/page). ETags on `If-None-Match` get `304 Not Modified` for 0 units.

## User Vision
> "alles würde ich sagen aber fokus auch klar auf das was nicht so einfach von der kostenlosen api abgedeckt wird.. und irgendwie auch via n8n und so funktioniert"

- Cover all three official APIs (Data + Analytics + Reporting) as a single CLI with sub-modules
- Especially address the gaps where official APIs are clumsy or quota-expensive (moderation queue UX, analytics digests, bulk ops, upload triggers)
- n8n-integration: `Execute Command` with JSON-out contract, typed exit codes, OAuth refresh-token persistence in `~/.config/youtube-pp-cli/`
- Future: TypeScript-helper for InnerTube-gaps (community posts, pin-comment) — out of scope for this Go CLI; planned as separate sibling project

## Source Priority (combo CLI)
- **Primary**: youtube-data-v3 — official OpenAPI, biggest surface (39 paths, ~100 ops)
- **Secondary**: youtube-analytics-v2 — official OpenAPI, 3 paths, all `POST /reports.query`-style
- **Tertiary**: youtube-reporting-v1 — official OpenAPI, 6 paths (jobs + report fetch)
- **Auxiliary**: PubSubHubbub WebSub — hand-built subscribe command, not in any OpenAPI
- **Economics**: All three APIs free under the same OAuth2 / API key (10K default daily quota across Data v3; Analytics + Reporting share their own quota pools). No paid tier.
- **Inversion risk**: None — Data v3 is correctly primary by surface size, command count, and user-stated priority.

## Product Thesis
- **Name**: youtube-pp-cli
- **Why it should exist**: YouTube Studio is web-only; the official Data API covers ~70-80% of Studio surface but is brutally quota-expensive when used naively. The competing CLIs (`ytstudio-cli`, `danvega/youtube-cli`) each cover one slice and ignore Analytics + Reporting + n8n integration. This CLI absorbs every Data/Analytics/Reporting operation, adds **agent-native output** (`--json`/`--select`/`--csv`), a **local SQLite store with FTS5** (offline search + incremental sync via ETags), a **quota meter** (per-call estimates + remaining), a **moderation queue UX** (held-for-review → batch-approve + author-ban as a single command), and a **PubSubHubbub subscribe helper** for n8n-trigger workflows.

## Build Priorities
1. Generator emits all 48 paths × auth/error wiring + local store schema
2. Hand-built P2 transcendence features: `mod queue`, `mod approve`, `mod auto`, `digest analytics`, `digest video`, `bulk metadata`, `playlist hygiene`, `pubsub subscribe/verify`, `quota meter`, `backup` (yt-dlp wrap), `ab thumbnails` (rotate + CTR), `reporting sync`
3. n8n recipes section in README + SKILL.md showing the `Execute Command` pattern
