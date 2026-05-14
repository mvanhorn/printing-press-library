# yt-studio Absorb Manifest

Locked scope per HANDOFF. Combined absorb (match every feature any existing
YouTube CLI/MCP exposes) + transcend (novel features only this CLI can deliver
because of its local SQLite + multi-channel schema + script binding).

## Tools surveyed

| Tool | Type | Stars | Features | Lang |
|---|---|---|---|---|
| dannySubsense/youtube-mcp-server | MCP | ~50 | 14 tools (channel/video/playlist/search/comments/transcript) | Python |
| anaisbetts/mcp-youtube | MCP | ~300 | subtitle download via yt-dlp | TS |
| adhikasp/mcp-youtube | MCP | ~100 | transcript fetch | Py |
| nattyraz/youtube-mcp | MCP | ~30 | metadata + captions → markdown | TS |
| mourad-ghafiri/youtube-mcp-server | MCP | ~20 | Whisper transcription + metadata | Py |
| space-cadet/yt-mcp | MCP | ~15 | YouTube Data API tools | TS |
| danvega/youtube-cli | CLI | ~10 | Spring Shell channel stats | Java |
| googleapis/google-api-go-client | SDK | 4K+ | full Data API + Analytics API typed Go bindings | Go |

None of these touch creator analytics (retention, demographics, CTR). None
have a local store. None bind to scripts.

## Absorbed — every feature anyone else has, matched and beaten

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|--------------|
| 1 | Get video details | dannySubsense.get_video_details | `video get <video_id>` (resource endpoint mirror) | Cached in store + `--json --compact` + `--select` |
| 2 | Get playlist details | dannySubsense.get_playlist_details | `playlist get <playlist_id>` | Cached, multi-channel scoped |
| 3 | Get playlist items | dannySubsense.get_playlist_items | `playlist items <playlist_id>` | Paginated, FTS-searchable |
| 4 | Get channel details | dannySubsense.get_channel_details | `channel get <channel_id>` and `channel info --self` | Stored, joined to videos |
| 5 | Get video categories | dannySubsense.get_video_categories | `categories list --region <code>` | Read once, cached |
| 6 | Get channel videos | dannySubsense.get_channel_videos | `videos list --channel <id> --limit <n>` | FTS, --since cursor |
| 7 | Search videos | dannySubsense.search_videos | `search "<q>" --channel <id>` (Data API) + `search "<q>"` (local FTS) | Hybrid live/offline, --json |
| 8 | Get trending videos | dannySubsense.get_trending_videos | `trending --region <code> --limit <n>` | Region cached |
| 9 | Get video comments | dannySubsense.get_video_comments | `video comments <id> --sort time\|relevance --limit <n>` | Read-only v1; agent-safe |
| 10 | Analyze video engagement | dannySubsense.analyze_video_engagement | `video engagement <id>` (uses Analytics API) | Real per-video data, not benchmarks |
| 11 | Get channel playlists | dannySubsense.get_channel_playlists | `playlists list --channel <id>` | Stored, FTS |
| 12 | Get caption info | dannySubsense.get_video_caption_info | `video captions <id>` (Data API captions.list) | Stub for v1; full transcript deferred |
| 13 | Evaluate video for knowledge base | dannySubsense.evaluate_video_for_knowledge_base | n/a — replaced by `framework-audit` | Better: real script binding |
| 14 | Fetch transcript | anaisbetts/adhikasp/nattyraz | n/a (defer to v1 polish) | `video captions <id>` returns track URLs |
| 15 | Channel stats CLI | danvega/youtube-cli | `channel info --self --json --compact` | Multi-channel + offline cache |
| 16 | Go SDK access | google.golang.org/api/youtube/v3 | (internal — every command uses it) | Wrapped + offline cached + agent-friendly |

## Transcend — only possible with our approach

Every command below is impossible (or massively worse) with any existing tool
because each requires one or more of: (a) Analytics API OAuth scope, (b) a local
SQLite with historical data, (c) a multi-channel normalized schema, (d) script
binding via content-registry.md.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|--------------------------|
| 1 | Retention curve | `retention <video_id> [--ascii\|--json]` | 10/10 | Analytics API + local store; auto-annotates 3 sharpest drops |
| 2 | Cohort retention | `retention-cohort --pattern "<regex>" --days <N>` | 10/10 | Requires local FTS over titles + per-video retention store |
| 3 | CTR decay | `ctr-decay <video_id>` | 9/10 | Needs daily snapshots in store; not derivable from one API call |
| 4 | Watchlist benchmark | `vs-watchlist --metric ctr,retention,upload-cadence` | 10/10 | Multi-channel schema; competitor data via public Data API normalized to own channel |
| 5 | Title pattern mining | `title-patterns --winners --losers` | 9/10 | Requires token-level analysis against per-video CTR; pure local query |
| 6 | Idea-gap detection | `idea-gap [--days <N>] [--watchlist <name>]` | 9/10 | Joins watchlist search snapshots against own video corpus |
| 7 | Framework audit | `framework-audit <video_id> [--script-dir <dir>]` | 10/10 | THE KILLER — joins retention curve buckets against script Signal/BeliefShift/CTA lines via content-registry.md |
| 8 | Manual script binding | `script-link <video_id> <script_path>` | 7/10 | Fallback for framework-audit when content-registry.md auto-detection fails |
| 9 | Watchlist suggest | `watchlist suggest --niche "<keywords>" --top <N>` | 8/10 | Auto-discovers competitor channels via Search API + Data API channel stats; ranks by relevance |
| 10 | Watchlist management | `watchlist add/remove/list` | 6/10 | YAML-backed local registry; multi-channel sync respects watchlist |
| 11 | Hybrid sync | `sync [--full\|--since <date>] [--data-source auto\|local\|live]` | 8/10 | Incremental own + watchlist; quota-aware (caps search.list usage); cursors per-table |
| 12 | OAuth + Studio login | `login` | 6/10 | One-time interactive; OAuth refresh + Studio cookie capture + SAPISIDHASH validation |
| 13 | Daily quota report | `quota` | 5/10 | Reads logged usage from stderr-mirror file; flags budget overflow |
| 14 | Cross-DB joins | `sql --attach <other.db> "SELECT …"` | 7/10 | ATTACH to content-ops-library.db for join with publishing pipeline |
| 15 | Sniff doctor | `sniff-doctor` | 5/10 | Verifies Studio Innertube schema at user runtime; deferred to polish but stubbed |

Build all 15. Items scored ≥8 go into the README's "Unique Features" and SKILL's
"Unique Capabilities" sections.

## Stub disclosures (explicit per skill contract)

None planned. Every row above is shipping-scope. The Studio Innertube layer (real-time
analytics + A/B thumbnails) is deferred to **polish**, not stubbed in v1. The
hero commands all run on the public Analytics API. If Phase 3 hits a blocker
that requires Studio session credentials I do not have, I will return to
Phase 1.5 with a revised manifest rather than silently downgrade.

The `login` command's Studio-session capture path is shipped but cannot be
verified live in this generation run (no logged-in user). It will fail closed
with typed exit 4 at user runtime if cookies expired or never captured.

## Source priority for combo rendering
Single provider (YouTube). No combo CLI priority logic. The README leads with
`framework-audit` and `retention`; the rest follows.
