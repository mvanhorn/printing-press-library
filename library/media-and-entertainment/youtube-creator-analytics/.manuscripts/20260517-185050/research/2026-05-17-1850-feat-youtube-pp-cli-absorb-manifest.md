# YouTube Printing Press — Absorb Manifest

## Absorbed (match competitor surface)

| # | Capability | Source | youtube-pp-cli equivalent |
|---|---|---|---|
| 1 | List/search/get videos, channels, playlists (Data v3) | `djthorpe/ytapi`, `googleapis/google-api-go-client`, `nerveband/youtube-api-cli` | `videos list/get`, `channels list/get`, `playlists list/get` — generator-emitted from Data v3 spec |
| 2 | Bulk metadata edits, comment moderation | `jdwit/ytstudio-cli` | `videos update`, `comments setModerationStatus` (OAuth) — generator-emitted |
| 3 | Comment thread retrieval + replies | `ZubeidHendricks/youtube-mcp-server`, `ytapi` | `comments threads`, `comments replies` — generator-emitted |
| 4 | Channel KPIs (views, sub count, video count) | `DannyIbo/youtube-data-analytics-tools`, `HarshaAbeyvickrama/YouTube-Statistics` | `channels get --part statistics` plus `analytics report` |
| 5 | Transcript fetch (no auth) | `youtube-transcript-api` (PyPI), `youtube-transcript` (npm), `yt-dlp` | `transcripts get <video>` — hand-coded scraper with yt-dlp fallback |
| 6 | OAuth installed-app flow | `ytapi`, `ytstudio-cli`, `googleapis` Go client | `auth login` / `auth status` / `auth logout` — generator-emitted OAuth helper |
| 7 | Output formatting (JSON/CSV/text) | `ytapi` (CSV mode), Composio | `--json --select`, `--csv`, default human table |
| 8 | MCP tool exposure of YouTube resources | `ZubeidHendricks/youtube-mcp-server`, `NastyRunner13/youtube-content-management-mcp`, `CDataSoftware/youtube-analytics-mcp-server` | Every Cobra command auto-exposed via `cobratree` walker; typed endpoint tools for `pp:endpoint` set |
| 9 | Trending video discovery | `NastyRunner13/youtube-content-management-mcp` | `videos list --chart mostPopular --region <CC>` — generator-emitted |
| 10 | Captions metadata (list available languages) | `ytapi` | `captions list --video <id>` — generator-emitted |

## Transcendence (only-we-can-do)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---|---|---|---|
| 1 | Decay detector — flag videos whose 7-day rolling views fell ≥N% below their own 30-day baseline | `youtube-pp-cli decay --threshold 0.3 --window 7` | hand-code | Requires daily snapshots in local SQLite + per-video baseline math. Web dashboards show absolute trend lines; none compute "below own baseline" alerts. Quota-free after morning sync. |
| 2 | Retention leaderboard — top videos by averageViewPercentage with title/length context | `youtube-pp-cli retention-leaderboard --top 20 --min-length 60` | hand-code | Joins Analytics `reports.query?metrics=averageViewPercentage&dimensions=video` with cached Data v3 video metadata in one SQL query. MCP servers expose raw Analytics rows but don't enrich. |
| 3 | Theme mining — extract recurring n-grams (2-4) from comments across the channel | `youtube-pp-cli theme-mine --videos mine --min-count 5 --json` | hand-code | Needs full comment corpus locally (thousands of rows) + tokenizer + stopword list. API can't aggregate; doing this online would burn quota and be slow. |
| 4 | Competitor diff — sync N rival channels and compare upload cadence, length, theme overlap | `youtube-pp-cli competitor-diff --me UCxxx --vs UCaaa,UCbbb --since 90d` | hand-code | Requires a `competitors` table, normalized stats, and Jaccard-style overlap on tag/title tokens. No competitor tool does this offline. |
| 5 | Transcript FTS5 search — full-text search across all scraped transcripts | `youtube-pp-cli transcript-search "sleep regression" --limit 10` | hand-code | Unique combination: scraped transcripts + FTS5 + snippet highlighting + timestamp jump links. `youtube-transcript-api` fetches; nobody indexes. |
| 6 | Comment FAQ — surface most-upvoted question-shaped comments per video | `youtube-pp-cli comment-faq --video <id> --top 10` | hand-code | Filters local `comments` by `?`/interrogatives, ranks by `likeCount`, dedupes near-duplicates. Live API call would cost 1 unit/page; offline it's free and fast. |
| 7 | Posting cadence vs performance — correlate day-of-week + hour-of-day with normalized views | `youtube-pp-cli posting-cadence --me --metric views --normalize` | hand-code | SQL group-by `strftime` on `publishedAt` joined to 28-day-views from snapshots. Web dashboards show a calendar; none correlate. |
| 8 | Sub velocity — per-day subscriber delta, attributable per-video where Analytics supports it | `youtube-pp-cli sub-velocity --since 30d --by video` | hand-code | Stitches Analytics `subscribersGained/Lost` by `day` + by `video` dimensions; computes running 7-day average locally. |
| 9 | CTR cohort — group videos by title pattern / length bucket and compare impressions→CTR | `youtube-pp-cli ctr-cohort --bucket title-prefix --top 5` | hand-code | Regex-bucket titles in SQL, join to Analytics `impressions,ctr` rows. Not exposed anywhere as a derived view. |
| 10 | Topic cluster — k-means-lite group of videos by title+tag+transcript token overlap | `youtube-pp-cli topic-cluster --k 6 --json` | hand-code | Needs local transcripts + simple TF-IDF + small-k clustering. Pure local compute, zero quota. |
| 11 | Quota-aware planner — estimate cost of a planned `sync` before running | `youtube-pp-cli sync plan --videos --comments --analytics` | hand-code | Reads the local request log, counts known costs per planned call, prints unit totals. No competitor surfaces this. |

All transcendence features are `hand-code` and depend on three foundations the generator gives us for free: typed endpoint clients, the SQLite store, and dual-auth (key for public hydration of competitor channels, OAuth for own-channel Analytics).
