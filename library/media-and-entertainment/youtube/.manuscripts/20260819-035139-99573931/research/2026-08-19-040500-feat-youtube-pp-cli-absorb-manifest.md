# youtube-pp-cli Absorb Manifest (reprint 2026-08-19)

Scope: read-only, api-key. OUT OF SCOPE by user ruling (19/08): OAuth writes, own-channel Analytics/Reporting APIs, Google Cloud project management. Target: market + competitor analysis.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search with full filter set | Bin-Huang/youtube-data-cli `search` | (generated endpoint) youtube search-list | Complete param wiring incl. maxResults (prior print dropped params); results cached to store |
| 2 | Bulk search from stdin/args | prior novel search-bulk | youtube-pp-cli youtube search-bulk | Top-N per term in one JSON document |
| 3 | Video details batch by ids | youtube-data-ss MCP getVideoDetails | (generated endpoint) youtube videos-list | --id CSV batching up to 50; statistics snapshot stored with date |
| 4 | Channel metadata + stats by id/@handle/username | Bin-Huang `channels` | (generated endpoint) youtube channels-list | forHandle resolution; stats snapshots stored |
| 5 | Trending by region/category | pauling-ai youtube-mcp-server trending | (behavior in (generated endpoint) youtube videos-list) chart=mostPopular + regionCode/videoCategoryId | cached locally for later offline comparison |
| 6 | Playlists + playlist items | Bin-Huang | (generated endpoint) youtube playlists-list / youtube playlist-items-list | paged, cached |
| 7 | Comment threads | Bin-Huang comment-threads | (generated endpoint) youtube comment-threads-list | cached |
| 8 | Top comments by like count | prior novel videos-comments | youtube-pp-cli youtube videos-comments | like-ranked across up to 5 pages regardless of API order |
| 9 | Video categories + i18n languages/regions | Bin-Huang | (generated endpoint) youtube video-categories-list / youtube i18n-languages-list / youtube i18n-regions-list | NEW coverage vs prior print |
| 10 | Channel sections | Bin-Huang | (generated endpoint) youtube channel-sections-list | NEW |
| 11 | Channel activities | Bin-Huang activities | (generated endpoint) youtube activities-list | NEW |
| 12 | Caption track listing | Bin-Huang captions | (generated endpoint) youtube captions-list | NEW |
| 13 | Transcript fetch without OAuth | prior novel videos-transcript + pauling-ai dual strategy | youtube-pp-cli youtube videos-transcript | timedtext, sole-track language fallback, --format markdown/text (restored from salvage), cached |
| 14 | Embed snippet generation | prior novel videos-embed | youtube-pp-cli youtube videos-embed | html/iframe/markdown |
| 15 | Related videos, topic-weighted | prior novel videos-related | youtube-pp-cli youtube videos-related | shared-topic outranks same-channel; HTML-unescaped; same-channel-only warning |
| 16 | Channel uploads in one call | prior novel channel-uploads | youtube-pp-cli youtube channel-uploads | @handle → uploads playlist → recent uploads |
| 17 | Playlist enrichment | prior novel playlist-enrich | youtube-pp-cli youtube playlist-enrich | concurrent, paged, per-row error isolation |
| 18 | Single-video enrichment | prior novel videos-enrich | youtube-pp-cli youtube videos-enrich | same enrichedVideo shape |
| 19 | Description link extraction | prior novel videos-links | youtube-pp-cli youtube videos-links | redirect expansion, social-noise skip |
| 20 | URL-tolerant video arguments | prior patch fix-videoarg-commands-parse-urls | (behavior in youtube-pp-cli youtube videos-transcript) shared parseVideoID on every video-arg command | youtu.be / watch?v= / scheme-less accepted |
| 21 | Local store, sync, offline search, SQL, doctor | Printing Press framework | youtube-pp-cli sync | plus search / sql / doctor; write-through caching; `backfill` (transcendence row 1) is the real data path |
| 22 | Learning loop | prior tree | youtube-pp-cli teach | plus recall / learnings / playbook; operator vocabulary persists |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|-------------------------|------------------|
| 1 | Competitor watchlist | watch | 10/10 | hand-code | Registers ~15 competitor channels in the local databank (typed watchlist table); the roster the monitoring machine runs on. USER ADDITION at gate (19/08). | Use this command to manage the set of channels the monitoring machine tracks. Do NOT use it to fetch channel data; use 'youtube channels-list' instead. |
| 2 | Watchlist monitor | monitor | 10/10 | hand-code | One command refreshes the whole watchlist: channel stats snapshot, new uploads since last run, re-snapshot of recent video stats, optional new comments. ~20-40 quota units per run for 15 channels; cron-able. Repeated dated snapshots create the time series the API itself never offers. USER ADDITION. | Use this command to refresh all watched channels' data in one run. Do NOT use it for a single ad-hoc channel pull; use 'backfill' instead. |
| 3 | Video velocity | velocity | 10/10 | hand-code | Views-per-day per video from real between-snapshot deltas in the local databank - current market movement, days fresher than NexLev's 1-2 week lag. USER ADDITION. | Use this command to see which videos are gaining views fastest right now. Do NOT use it for lifetime channel deltas; use 'growth' instead. |
| 4 | Growth between snapshots | growth | 8/10 | hand-code | Channel-level subs/views/upload-count deltas between dated snapshots (>=2 guard); the API exposes no history at all. | Use this command for channel-level change over time between snapshots. Do NOT use it for per-video movement; use 'velocity' instead. |
| 5 | Breakout finder | breakouts | 9/10 | hand-code | Chained search-filter matrix (terms x upload-window x duration x region/category), local dedupe, batched stats, join to channel size in the databank, ranked by views-per-subscriber and views-per-day - fresh niche breakouts weeks before curated indexes. USER ADDITION (chained filters). | Use this command to discover fresh high-momentum videos across the market. Do NOT use it for tracked-competitor movement; use 'velocity' instead. |
| 6 | Comment intelligence | comments-mine | 9/10 | hand-code | Syncs comments into a typed FTS table and reports top-liked, keyword frequencies, and question extraction - fast audience signal from data you own. USER ADDITION. | Use this command for aggregate comment analysis from the local store. Do NOT use it to fetch one video's raw top comments live; use 'youtube videos-comments' instead. |
| 7 | Channel history backfill | backfill | 10/10 | hand-code | Uploads-playlist walk + batched videos.list (~2 units per 100 videos) into the databank with dated snapshots; seeds every new watchlist entry with full history. | Use this command to pull a channel's complete upload history into the local store. Do NOT use it for a quick recent-uploads peek; use 'youtube channel-uploads' instead. |
| 8 | Packaging collector | packaging | 9/10 | hand-code | Per video or per watched channel: title, thumbnail downloaded as a local image file, hook text (transcript opening), duration, and stats into a typed packaging table - so the strong agent driving this CLI can do thumbnail/hook/packaging analysis directly from the databank. USER ADDITION (replaces chapters). | Use this command to collect titles, thumbnails, and hook text for packaging analysis. Do NOT use it for full transcripts; use 'youtube videos-transcript' instead. |

| 9 | Workspaces | workspace | 10/10 | hand-code | Named databank profiles mapped to separate SQLite files: the competitor machine stays untouched while a new niche is explored in its own workspace; instant switching. USER ADDITION (19/08, isolation requirement). | Use this command to create, list, and switch named databanks (e.g. an explore workspace for a new niche). Do NOT use it to manage tracked channels inside a workspace; use 'watch' instead. |
| 10 | API key ring | auth keys | 9/10 | hand-code | Multiple named YouTube API keys stored locally: instant switching plus optional automatic failover to the next key when one exhausts its quota mid-run. USER ADDITION (19/08). | Use this command to add, list, and switch between stored API keys. Do NOT use it for one-off key entry; use 'auth set-token' instead. |

Reprint verdicts: all 9 prior novels KEEP (see brainstorm audit trail). No stubs anywhere in this manifest.
Hand-code count: 10 transcendence rows + 9 prior novel command files to carry/re-land + rich-transcript restore. Killed at gate by user: outliers, cadence, titles, compare-channels (NexLev owns computed wide-market judgments), videos-chapters (not interesting). Databank schema is a first-class deliverable: typed tables channels, channel_snapshots, videos, video_snapshots, comments (FTS), watchlist, monitor_runs, packaging — per workspace (named DB files).
