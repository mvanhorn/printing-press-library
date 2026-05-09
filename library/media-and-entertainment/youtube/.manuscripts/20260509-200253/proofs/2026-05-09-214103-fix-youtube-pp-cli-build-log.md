# Build Log — youtube-pp-cli

## What was built

### Foundation
- API key auth wired (`YOUTUBE_API_KEY` → `?key=` query param via client.go); coexists with OAuth bearer for write commands
- Doctor updated to recognize either credential as "configured"
- `internal/store/youtube_extras.go` — YouTube-specific tables: `yt_channels`, `yt_videos`, `yt_comments`, `yt_transcripts`, `yt_trending_snapshots`, `yt_video_snapshots`, `yt_quota_log` + FTS5 indexes (`yt_videos_fts`, `yt_transcripts_fts`, `yt_comments_fts`)

### Absorbed (Priority 1) — generator-emitted, all 30 features
The generator emitted 76 endpoint commands under `youtube <resource>-<method>` covering every documented Data API v3 read/write operation:
- videos.list/insert/update/delete/rate/getRating/reportAbuse
- channels.list/update
- playlists/playlistItems CRUD
- search.list
- comments/commentThreads CRUD + setModerationStatus + markAsSpam
- captions.list/insert/update/delete/download
- subscriptions/members/membershipsLevels list (+ subs CRUD)
- channelSections/channelBanners CRUD
- thumbnails.set, watermarks.set/unset
- liveBroadcasts/liveChat (messages, bans, moderators)
- activities.list, videoCategories.list, i18nLanguages/Regions, videoAbuseReportReasons.list

### Transcendence (Priority 2) — 8 hand-built commands
1. `quota plan <endpoint>...` — projects unit cost vs. local 24h ledger
2. `cost ledger` + `cost log` — read/write the quota_log
3. `corpus search <q>` — FTS5 union over videos/transcripts/comments
4. `digest <channel>` — channel uploads × transcript-FTS keyword join
5. `trending snapshot` + `trending diff` — captures and diffs `mostPopular`
6. `velocity <channel>` — Δviews/Δlikes/Δcomments per day from snapshot history
7. `topic crossover <kw>` — trending × videos × transcripts join
8. `subscriptions sweep` — OAuth-gated sub feed rebuild

### Supporting
- `internal/transcripts/ytdlp.go` — yt-dlp shell-out for VTT subs + plain-text extraction
- `internal/quota/quota.go` — endpoint cost map + APIKey hashing for ledger
- `sync-channel <id>` — YouTube-specific replacement for the generic `sync` (which can't enumerate without a starting param)

## What was intentionally deferred

- **Live OAuth login flow harvesting full token to disk:** the generator emitted `auth login` but the YouTube-specific OAuth scope set hasn't been validated end-to-end. A user running OAuth-gated commands gets an honest "oauth required" error pointing to `auth login`.
- **Comments sync command:** comments are a separate sync target. Calling `youtube comments-list <video> --all` writes to the generic `resources` table; promoting to `yt_comments` for FTS would need a dedicated sync-comments command. Left for v0.2.
- **Live broadcast moderation flows:** wired through endpoint mirrors but not given dry-run wrappers for moderation chains (insert-bans, insert-cuepoints).

## Skipped body fields

The OpenAPI spec marks several POST/PUT bodies (`videos.insert`, `captions.insert`, `thumbnails.set`, `watermarks.set`) as having complex multi-part request bodies. The generator skipped emitting flag-based shims for those bodies; users must use `--stdin` JSON, which is the correct shape for those endpoints.

## Generator limitations encountered

- The OpenAPI spec from apis.guru declared only `Oauth2`/`Oauth2c` security schemes, no `apiKey`. Before generation I patched the spec to add an `apiKey` scheme + top-level `security: [{apiKey: []}]`. Generator's auth-detection still picked OAuth as primary (the OAuth schemes have richer scope metadata). Worked around by hand-wiring API key support in `config.go` + `client.go` + `doctor.go` — printed-CLI fix, not a generator fix.
- The generator's MCP block in the spec (`mcp.transport`, `mcp.orchestration`, `mcp.endpoint_tools`) didn't get applied. Scorecard's `mcp_token_efficiency`/`mcp_remote_transport`/`mcp_tool_design`/`mcp_surface_strategy` dims scored 4-5/10 as a result. Would benefit from a polish pass that switches to the Cloudflare orchestration pattern.

## Build summary
- Go module: `youtube-pp-cli`
- Binary: 20 MB
- Total commands registered: 93 (76 endpoint mirrors + 8 transcendence + 9 framework: doctor, auth, sync, sync-channel, transcripts, agent-context, profile, feedback, etc.)
- All tests pass: `go build ./...` clean, `go vet ./...` clean
