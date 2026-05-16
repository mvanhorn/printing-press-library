# YouTube CLI Absorb Manifest

## Source Tools Surveyed
- `jdwit/ytstudio-cli` (Java/Spring) — bulk metadata, channel management
- `danvega/youtube-cli` (Spring Shell) — stats, video listing, year filtering
- `HRSPROJECT/youtube-cli` (Python) — watch, comment, full stats
- `philbot9/youtube-comment-scraper-cli` (Node) — comment scrape JSON/CSV
- `mattwright324/youtube-comment-suite` (Java GUI) — comment archive across channels
- `anaisbetts/mcp-youtube` (Node MCP) — search/metadata/transcript
- `dannySubsense/youtube-mcp-server` (Python MCP) — 14 tools, content evaluation
- `stackone YouTube MCP` — 47 actions
- `apify/youtube-mcp-server` — search + channel stats

---

## Absorbed (match or beat everything that exists)

### YouTube Data API v3 (generator-emitted from OpenAPI)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | `videos list` / `get` / `update` / `delete` / `rate` | jdwit/ytstudio-cli | Auto-generated from spec | --json/--select/--csv/--compact, ETag caching, local store cache |
| 2 | `videos insert` (upload) | youtubeuploader, ytstudio-cli | Auto-generated + chunked multipart helper | --dry-run shows quota cost (1600 units), progress bar |
| 3 | `playlists` CRUD | All CLIs | Generator | Same agent-native flags |
| 4 | `playlistItems` CRUD + reorder | jdwit/ytstudio-cli | Generator | `bulk-position` helper for full-playlist reorder |
| 5 | `channels list/update` (brandingSettings) | jdwit/ytstudio-cli | Generator | Offline channel cache |
| 6 | `channelBanners insert` | none of the CLIs | Generator | Adds image-resize warning if dims wrong |
| 7 | `channelSections` CRUD | apify | Generator | — |
| 8 | `thumbnails set` | jdwit/ytstudio-cli, ThumbnailTest | Generator | Quota-cost printed, used by `ab thumbnails` |
| 9 | `watermarks set/unset` | none | Generator | — |
| 10 | `captions` insert/list/update/delete/download | mattwright324 | Generator | yt-dlp fallback for download |
| 11 | `commentThreads list/insert` | mattwright324, philbot9, anaisbetts | Generator | Streams via paginator with quota-aware backoff |
| 12 | `comments setModerationStatus` (batch IDs) | none | Generator | Accepts stdin-piped IDs, `--ban-author` flag |
| 13 | `comments` insert/update/delete | HRSPROJECT | Generator | — |
| 14 | `subscriptions` list/insert/delete | danvega | Generator | — |
| 15 | `search.list` | All CLIs | Generator | Warns at invocation: "100 quota units — consider `videos list --mine` (1 unit)" |
| 16 | `videoCategories`, `i18nLanguages`, `i18nRegions` | none | Generator | Local cache (rarely changes) |
| 17 | `activities list` | none (mostly deprecated) | Generator | Marked deprecated in help |
| 18 | `videoAbuseReportReasons list` | none | Generator | — |
| 19 | `members list` | none | Generator | (Requires Google access approval — surfaced in `doctor`) |
| 20 | `membershipsLevels list` | none | Generator | — |
| 21 | `superChatEvents list` (last 30d, isSuperStickerEvent flag) | none | Generator | — |
| 22 | `liveBroadcasts` CRUD (insert/bind/control/transition) | none | Generator | — |
| 23 | `liveStreams` CRUD | none | Generator | — |
| 24 | `liveChatMessages list/insert/delete` | none | Generator | Streaming poll command for long-running listen |
| 25 | `liveChatBans` / `liveChatModerators` | none | Generator | — |
| 26 | `videoSetRating` / `getRating` | HRSPROJECT | Generator | — |
| 27 | `abuseReports insert` | none | Generator | — |

### YouTube Analytics API v2 (generator-emitted)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 28 | `reports query` (raw) | danvega | Generator | Dimension/metric autocompletion in help |
| 29 | `groups list/insert/update/delete` | none | Generator | — |
| 30 | `groupItems list/insert/delete` | none | Generator | — |

### YouTube Reporting API v1 (generator-emitted)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 31 | `jobs create/list/delete` | none | Generator | — |
| 32 | `reportTypes list` | none | Generator | — |
| 33 | `jobs.reports list/get` | none | Generator | Auto-download via redirect helper |

### Framework (printing-press default)

| # | Feature | Our Implementation |
|---|---------|--------------------|
| 34 | `auth login` (OAuth2 device flow) | Persistent refresh token at `~/.config/youtube-pp-cli/auth.json` |
| 35 | `auth status` | Token validity + scopes granted |
| 36 | `doctor` | API reachability, OAuth status, members-list access flag |
| 37 | `sync` | Pull videos, playlists, comments, subscriptions into local SQLite |
| 38 | `search "term"` | FTS5 over synced data |
| 39 | `sql "SELECT …"` | SQL access to local store |
| 40 | `agent-context` / `agent-context json` | Agent-native command tree + flags |

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|-------------------------|-------|
| T1 | **Held-for-review moderation queue** | `mod queue --since 7d --json` | Combines `commentThreads.list?moderationStatus=heldForReview` + local store + batch-approve+ban-author in one verb. No CLI has this. | 9/10 |
| T2 | **Pattern-based comment auto-mod** | `mod auto --rules rules.yaml --since 1h --apply` | Local regex/keyword rules + LLM-classify hook + `setModerationStatus`. Replaces 3-step Studio dance. | 8/10 |
| T3 | **Daily/weekly analytics digest** | `digest analytics --since 7d --markdown` | Pre-baked queries (top videos, CTR, watch time, traffic sources, revenue, Shorts vs long-form). n8n→Email/Slack ready. | 9/10 |
| T4 | **Per-video deep-dive digest** | `digest video <id> --markdown` | Aggregates retention curve, traffic sources, demographics, devices for one video. | 7/10 |
| T5 | **Bulk metadata operations** | `bulk metadata --filter '<jq>' --set 'category=22' --append-description footer.md --dry-run` | Pipeline: enumerate uploads via `playlistItems.list` (1 unit, not 100), match filter, apply mutation, print diff. | 8/10 |
| T6 | **Playlist hygiene** | `playlist hygiene --auto-add 'tag:tutorial->PL_tutorial' --reorder-by views` | Tag/title pattern→playlist mapping + reorder by view count. Pure Data API loop with quota accounting. | 7/10 |
| T7 | **PubSubHubbub subscribe + verify** | `pubsub subscribe --topic-channel UCxxx --callback https://n8n/.../webhook` | Hand-built subscribe POST + verify-helper that prints the GET challenge n8n must echo. No CLI does this. | 9/10 |
| T8 | **Quota meter** | `quota meter` (and `--show-quota` global) | Per-call cost calc + remaining-estimate tracked in local SQLite across runs. Prevents the "10K-units-in-one-execute_command-call" disaster. | 9/10 |
| T9 | **Backup own channel** | `backup --since 30d --captions --thumbnails --info-json` | yt-dlp wrap with cookies-from-browser, writes to S3-compatible target or local. Indexes the backup in store. | 8/10 |
| T10 | **A/B thumbnail testing (DIY)** | `ab thumbnails start --video <id> --variants a.png,b.png --rotate 24h`, `ab thumbnails report` | Rotate via `thumbnails.set` on schedule, pull CTR via Analytics API, compute statistical significance locally. The only API-only A/B path. | 8/10 |
| T11 | **Reporting API warehouse sync** | `reporting sync --types content_owner_a1,channel_basic_a2 --since 30d --out ./reports/` | Async: enumerate `reportTypes`, ensure `jobs` exist, poll `reports`, download CSV via redirect. Drop-in for daily warehouse pulls. | 7/10 |
| T12 | **n8n recipe pack** | `recipes n8n` (prints) | Prints 6 ready-to-paste n8n workflow JSON snippets: held-comment-mod, daily-digest, upload-trigger via PubSub, bulk-metadata-monthly, ab-thumbnail-loop, backup-weekly. | 6/10 |
| T13 | **Auto-chapters from transcript** | `chapters auto <video-id> --apply` | yt-dlp pull captions → LLM (configurable provider) → write back to description via `videos.update`. | 6/10 |

---

## Build Status Plan
- **Shipping in this run (Priority 1)**: All 33 generator-emitted absorbed rows (#1-33). All 13 transcendence features (T1-T13) get full implementations — they are the differentiator.
- **Stubs**: T13 (`chapters auto`) ships with a configurable LLM provider hook but defaults to an instructional message if no provider configured. The transcript fetch + write-back is real.
- **Out of scope (deferred for separate TS sibling project)**: Community posts create/read, pin-comment, heart-comment, A/B test verdict reading from Studio, Studio Editor automation. Documented under `## Known Gaps` in README.
