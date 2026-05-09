# YouTube Absorb Manifest

## Source tools surveyed

| Tool | URL | Role |
|------|-----|------|
| YouTube Data API v3 (OpenAPI) | https://api.apis.guru/v2/specs/googleapis.com/youtube/v3/openapi.yaml | Canonical 23-resource / 72-method spec |
| dannySubsense/youtube-mcp-server | https://github.com/dannySubsense/youtube-mcp-server | Most comprehensive MCP (14 read functions) |
| anaisbetts/mcp-youtube | https://github.com/anaisbetts/mcp-youtube | yt-dlp-based transcript MCP |
| nattyraz/youtube-mcp | https://github.com/nattyraz/youtube-mcp | Captions + markdown conversion MCP |
| mourad-ghafiri/youtube-mcp-server | https://github.com/mourad-ghafiri/youtube-mcp-server | Transcription + metadata MCP |
| yt-mcp / space-cadet | https://space-cadet.github.io/yt-mcp/ | Data API MCP |
| yt-dlp | https://github.com/yt-dlp/yt-dlp | Reference for transcript path & format listing |
| danvega/youtube-cli | https://github.com/danvega/youtube-cli | Spring Shell + Data API channel stats CLI |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Get video details (snippet+stats+contentDetails) | dannySubsense `get_video_details`, yt-mcp | `videos get` → videos.list batched, cached to SQLite | --json --select, offline re-query |
| 2 | Get playlist details | dannySubsense `get_playlist_details` | `playlists get` → playlists.list, cached | offline, --json |
| 3 | Playlist items full crawl | dannySubsense `get_playlist_items` | `playlist-items list --all` → playlistItems.list paginated | full crawl, ETag cache |
| 4 | Get channel details | dannySubsense `get_channel_details` | `channels get` → channels.list, cached | offline, --select |
| 5 | List video categories | dannySubsense `get_video_categories` | `categories list --region US` → videoCategories.list | offline cache |
| 6 | Channel videos | dannySubsense `get_channel_videos` | `channels videos --all` → uploads playlist crawl | full crawl, offline |
| 7 | Search videos | dannySubsense `search_videos`, yt-mcp | `search videos "<q>" --type video` → search.list | quota-cost preview before run |
| 8 | Trending videos by region | dannySubsense `get_trending_videos` | `trending --region US` → videos.list?chart=mostPopular | snapshotted to SQLite |
| 9 | Video comments (sorted, paginated) | dannySubsense `get_video_comments` | `comments list <video> --order time/relevance` | offline FTS over comment text |
| 10 | Channel playlists | dannySubsense `get_channel_playlists` | `channels playlists <id>` → playlists.list?channelId | offline, --json |
| 11 | Caption metadata | dannySubsense `get_video_caption_info` | `captions list <video>` → captions.list | --json, --select |
| 12 | Video transcript (no quota) | anaisbetts mcp-youtube, nattyraz | `transcripts get <video> --lang en` → yt-dlp shell-out for VTT/SRT | local cache, FTS, multi-language |
| 13 | Activities feed | YouTube Data API | `activities list --channel <id>` → activities.list | full pagination |
| 14 | Subscriptions list | YouTube Data API (OAuth) | `subscriptions list --mine` → subscriptions.list | OAuth-gated, --dry-run |
| 15 | Members list | YouTube Data API (OAuth) | `members list` → members.list | OAuth-gated |
| 16 | Comment thread insert/reply | API | `comments reply <thread> --text "..."` → commentThreads.insert/comments.insert | --dry-run by default |
| 17 | Playlist CRUD | YouTube Data API | `playlists {create,update,delete}`, `playlist-items {add,remove,update}` | --dry-run, --stdin batch |
| 18 | Comment moderation | YouTube Data API | `comments moderate <id> --status held/published/rejected` | --dry-run |
| 19 | Video metadata update | YouTube Data API | `videos update <id> --title --desc --tags --category` | --dry-run, --stdin batch |
| 20 | Video rate (like/dislike/none) | YouTube Data API | `videos rate <id> --rating like/dislike/none` | OAuth-gated |
| 21 | Get rating | YouTube Data API | `videos get-rating <id>` | OAuth-gated |
| 22 | Captions upload/update/delete | YouTube Data API | `captions {upload,update,delete}` | OAuth-gated, --dry-run |
| 23 | Thumbnails set | YouTube Data API | `thumbnails set <video> --file path.jpg` | OAuth-gated, --dry-run |
| 24 | Channel sections CRUD | YouTube Data API | `channel-sections {list,create,update,delete}` | OAuth-gated |
| 25 | Subscriptions CRUD | YouTube Data API | `subscriptions {add,remove}` | OAuth-gated |
| 26 | Report abuse / abuse-report-reasons | YouTube Data API | `videos report-abuse <id>`, `abuse-reasons list` | OAuth-gated |
| 27 | i18n languages / regions | YouTube Data API | `i18n languages`, `i18n regions` | offline cache |
| 28 | Video format/quality info | yt-dlp -F | `formats list <video>` → yt-dlp shell-out for `formats` JSON | offline cache, --json |
| 29 | Engagement metrics analyzer | dannySubsense `analyze_video_engagement` | `videos engagement <id>` → local computation | runs offline, no quota |
| 30 | Live broadcasts/chat metadata | YouTube Data API | `live broadcasts list`, `live chat messages` | OAuth-gated |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Quota cost planner | `quota plan <cmd...>` | 10/10 | Parses planned command tree, sums per-endpoint unit cost, compares against local 24h ledger — no MCP exposes this; nothing dry-runs cost |
| 2 | Channel digest with FTS | `digest <channel> --since 7d --keywords k1,k2` | 9/10 | SQLite join of videos × transcripts × channels filtered by publishedAt and FTS match in one command |
| 3 | Cross-corpus FTS search | `corpus search "<query>"` | 10/10 | FTS5 union across videos+transcripts+comments; no MCP server in the field does this |
| 4 | Trending list diff | `trending diff <region> --since YYYY-MM-DD` | 8/10 | Diff against `trending_snapshots` table populated by sync — absorb only snapshots, this diffs |
| 5 | Per-video velocity | `velocity <channel> --window 30d` | 8/10 | Δviews/Δlikes/Δcomments per day from repeated `videos.list` snapshots |
| 6 | Topic-trending crossover | `topic crossover "<keyword>"` | 7/10 | Three-table join trending × videos × transcripts ranked by trending position |
| 7 | Subscription feed rebuild | `subscriptions sweep --since 7d` | 8/10 | OAuth subs → uploads-playlist join → chronological feed agent-side |
| 8 | Quota ledger audit | `cost ledger [--last 24h]` | 8/10 | Local sidecar table populated by every command; aggregates by command/endpoint/day |

## Stubs

None. All transcendence features are shipping-scope. Live broadcast/chat (Absorb #30) and OAuth-gated mutations (Absorb #14-26) require OAuth — they ship with the OAuth code path; if no OAuth token is present, they print an honest `oauth required: run \`youtube-pp-cli auth login\`` message rather than failing silently.

## Build totals

- Absorbed: 30
- Transcendence: 8
- Total commands targeted: ~38 user-facing
