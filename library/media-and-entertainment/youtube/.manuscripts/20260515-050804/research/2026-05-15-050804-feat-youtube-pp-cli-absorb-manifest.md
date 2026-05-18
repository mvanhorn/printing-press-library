# YouTube CLI Absorb Manifest

Total surface to ship: **38 generator-emitted resource commands + 10 transcendence features = 48 commands** (under the >50 MCP threshold but close — see MCP enrichment recommendation below).

## Absorbed (match or beat the API-key-compatible surface)

The user has `YOUTUBE_API_KEY` only (no OAuth client). All write operations (upload, mutations, ratings, comments-create, playlist-create, etc.) are out of scope. Only read-only public-data commands are absorbed; OAuth-required commands are emitted as stubs that print a clear "OAuth required, not configured" message.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | Search videos/channels/playlists | Bin-Huang/youtube-data-cli `search` | Generator-emitted `search` cmd with all filters + `--webapp` mode | Local cache (FTS5), `--select` for partial output, `--csv` |
| 2 | Get video details | Bin-Huang `videos` | `videos get/list` with batched IDs | Auto-cache to store; `videos enrich` for batch |
| 3 | Get channel details | Bin-Huang `channels` | `channels get/list` | Local sync; FTS over channel titles/descriptions |
| 4 | List playlists | Bin-Huang `playlists` | `playlists list` | Local sync |
| 5 | List playlist items | Bin-Huang `playlist-items` | `playlists items` | Local sync; offline browse |
| 6 | Channel sections | Bin-Huang `channel-sections` | `channels sections` | (read-only) |
| 7 | Comment threads | Bin-Huang `comment-threads` | `comments threads` | Local cache, FTS over comment text |
| 8 | Replies | Bin-Huang `comments` | `comments replies` | Local cache |
| 9 | Captions metadata | Bin-Huang `captions` | `captions list` | Read-only; lists tracks, language, kind |
| 10 | Activities | Bin-Huang `activities` | `activities list` | Local sync |
| 11 | Subscriptions (own, public) | Bin-Huang `subscriptions` | `subscriptions list` | Local sync |
| 12 | Video categories | Bin-Huang `video-categories` | `video-categories list` | (static enrichment) |
| 13 | i18n regions/languages | Bin-Huang `i18n-*` | `i18n regions/languages` | (static) |
| 14 | Video abuse report reasons | Bin-Huang `video-abuse-report-reasons` | (read-only) | |
| 15 | Trending videos | pauling-ai `youtube_trending` | `videos trending --region <r>` | Local cache, ranked output |
| 16 | Channel summary | pauling-ai `youtube_get_channel` | Default `channels get` | Beats: handles ID/handle/@username uniformly |
| 17 | List videos for a channel | pauling-ai `youtube_list_videos` | `channels videos <channelId>` | Uses uploads-playlist trick (quota-cheap), local cache |
| 18 | Comment listing for a video | pauling-ai `youtube_list_comments` | `videos comments <id>` | Composes with comment digest (novel #10) |
| 19 | Trending search suggestions | pauling-ai `youtube_search_suggestions` | `search suggest <prefix>` | Free (no quota cost), uses public autocomplete endpoint |
| 20 | Markdown caption export | nattyraz/youtube-mcp | `transcripts get <id> --format markdown` | Composes with novel transcript surface (novel #3) |
| 21 | Content freshness scoring | dannySubsense/youtube-mcp-server | (skipped — opinionated, low-fit for user vision) | |
| 22 | Channel auto-resolve (ID/handle/@) | Multiple | Implicit in all `channels` commands | Resolver helper, no separate flag needed |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|--------------------------|-------|
| 1 | Bulk search with embed-ready JSON | `search bulk --stdin` or `search bulk "term1" "term2"` | Reads keywords from stdin/args, returns top-N per term in webapp-ready JSON (videoId, title, embedUrl, thumbnailUrl, duration_ms, publishedAt, channel). Beats every existing CLI/MCP — none take a list of terms. Tied directly to user's photo→keywords→videos vision. | 10 |
| 2 | Topic-similarity related videos | `videos related <videoId>` | Rebuilds deprecated `relatedToVideoId` using topicDetails + same-channel + tag overlap from local store. Requires synced `videos` + `topics` tables — no existing tool has rebuilt this since June 2023 deprecation. | 9 |
| 3 | Transcript fetcher | `videos transcript <id>` and `videos transcripts <id...>` | Hits the unofficial timedtext endpoint with proper YouTube-web-client headers (jdepoix approach). Caches into `transcripts` table. Auto-generated or manual; language-selectable. Works on residential IP (user's machine), documented IP-block risk for cloud. | 10 |
| 4 | Transcript full-text search | `transcripts search "<query>" [--video <id>] [--channel <id>]` | FTS5 over locally-synced transcripts. Killer use case: "find moments across N synced videos that mention <topic>". Compounds with bulk-search-then-sync workflow. Only possible because we own the local store. | 10 |
| 5 | Webapp-ready output mode | `--webapp` flag (global) | Emits clean JSON shape optimized for frontend embed: `{videoId, title, channel: {id, title, handle}, embedUrl, watchUrl, thumbnailUrl, duration_ms, publishedAt, transcript_excerpt?}`. Tied directly to user's webapp-dev vision. | 9 |
| 6 | Search result diff over time | `search diff "<term>"` | Compares current `search.list` result against last cached version for that term; returns `new`, `dropped`, `unchanged`. Only possible because we cache search results by query+timestamp. | 8 |
| 7 | Channel upload watcher | `channels new-uploads <channelId> --since 7d` | Returns videos uploaded since N days ago, quota-cheap (uses uploads playlist instead of `search.list`). Local sync-aware. | 8 |
| 8 | Quota meter | `quota` | Tracks API calls in local store, estimates remaining daily quota against YouTube's 10k unit ceiling. Compounds with sync planning. | 7 |
| 9 | Bulk video enrichment | `videos enrich <id1> <id2> ...` | Batches up to 50 IDs per `videos.list` call (the API's hard ceiling), fetches `snippet,statistics,contentDetails,topicDetails`. Beats per-ID loop pattern in every other CLI. | 7 |
| 10 | Comment digest | `videos comments <id> --top 20 --threads` | Returns top-rated comments with thread depth and reply summary. Composes with sync for offline browse. | 6 |

## Stubs (OAuth required, not configured)

These are emitted with honest "OAuth required, not configured" messaging since the user has API key only:
- All upload/insert/update/delete operations on videos, channels, playlists, comments, captions, channel-sections, subscriptions, playlist-images, watermarks
- `captions.download` (requires OAuth as video owner)
- Members, memberships levels, super-chat events, live-broadcast write ops
- Analytics, revenue, retention (require YouTube Analytics API + OAuth — pauling-ai's strongest features, but out of scope here)

## MCP Enrichment Recommendation

**Tool count estimate**: 38 endpoint tools + ~13 framework tools (sync, sql, search, context, etc.) + 10 novel feature tools = **~61 MCP tools**. This crosses the >50 threshold from the skill's decision table.

**Recommend spec enrichment before generation:**
```yaml
x-mcp:
  transport: [stdio, http]    # remote-capable (user wants webapp integration)
  orchestration: code         # thin search+execute pair
  endpoint_tools: hidden      # suppress raw per-endpoint mirrors
  intents:                    # optional named workflows
    - name: bulk_search_with_transcripts
      description: Search YouTube for multiple terms, fetch transcripts for top results, return webapp-ready JSON
```

Webapp use case especially benefits from HTTP transport — the user's webapp can call the MCP server directly without a local stdio bridge.
