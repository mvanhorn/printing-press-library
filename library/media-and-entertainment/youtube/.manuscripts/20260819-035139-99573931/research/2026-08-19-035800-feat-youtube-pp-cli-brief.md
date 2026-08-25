# YouTube CLI Brief (reprint 2026-08-19)

## API Identity
- Domain: YouTube Data API v3 — official Google REST API, `https://youtube.googleapis.com`, discovery revision 20260817 (saved: `research/youtube-v3-discovery-20260817.json`, 32 resources / 83 methods / 211 schemas, verified identical surface to 13/08 revision).
- Users: **YouTube analysts and channel operators** — people doing channel research, competitor analysis, niche evaluation, and packaging decisions (the operator persona: runs their own channels, studies ~100 competitor channels, wants outlier/cadence/title evidence, not embed snippets). Secondary: AI agents doing YouTube research via MCP.
- Data profile: videos, channels, playlists/playlistItems (the uploads-playlist chain), commentThreads, plus read-only-with-api-key resources the prior CLI omitted: videoCategories, i18nLanguages/i18nRegions, channelSections. Search is rationed (own 100-calls/day bucket, 1 unit/call, paging costs per page); everything else shares 10,000 units/day at ~1 unit/call.

## Reachability Risk
- **None.** Live-verified this session: `search.list` returned `youtube#searchListResponse` with the stored config key (receipt in session transcript, 19/08/2026). Official, stable, documented API.
- Auth: API key (`YOUTUBE_API_KEY` canonical — but note the operator's env copy is dead; the working key lives in the CLI credentials file, so doctor/auth guidance must handle the env-shadows-config trap).

## Top Workflows (analyst persona)
1. **Backfill a channel's full upload history locally** — @handle → uploads playlist → all videoIds → batched statistics into SQLite. This is the foundation everything else needs and what NO existing free CLI does. Cost: ~2 units per 100 videos.
2. **Find outliers** — which videos beat their own channel's median by how much (the metric vidIQ/ViewStats/OutlierKit sell subscriptions for).
3. **Read upload cadence** — median gap, liveness, accelerating or dying (competitor vitality check before entering a niche).
4. **Bulk competitive sweep** — search-bulk terms → channels → backfill → compare, all offline afterwards.
5. **Content forensics** — transcripts (timedtext, no OAuth), top comments by likes, description links, related-by-topic.

## Table Stakes (from the ecosystem sweep)
- Bin-Huang/youtube-data-cli: full read surface (search, channels, videos, playlists, playlistItems, commentThreads, videoCategories, i18n, activities, captions-list), JSON to stdout, agent-first. **No store, no sync, no analysis, no transcripts without OAuth.**
- pauling-ai/youtube-mcp-server (40 tools): search suggestions (no quota), trending, categories, transcripts w/ scraping fallback, quota tracking. Analytics/Reporting API tools are OAuth-only — out of scope for an api-key CLI, and our operator already has an OAuth pipeline elsewhere.
- the operator's youtube-data-ss MCP: searchVideos, getVideoDetails, getChannelStatistics, getChannelTopVideos, getTrendingVideos, getTranscripts, compareVideos, getVideoEngagementRatio, getRelatedVideos.
- Prior youtube-pp-cli novels (must not regress): search-bulk, videos-transcript (incl. lost --format markdown/text — restore), videos-embed, videos-related (topic-weighted ranking + HTML-unescape), videos-comments (top by likes), channel-uploads, playlist-enrich, videos-enrich, videos-links, teach/recall/learnings/playbook learning loop, MCP schema-key sanitizer.

## Data Layer
- Primary entities: videos (with statistics snapshot), channels (with statistics), playlist_items, comment threads, transcripts (cached), search results.
- Sync cursor: per-channel backfill via uploads playlist (`playlistItems.list` pages + `videos.list` batched by 50 ids — THE fix for the prior "sync is a no-op" gap). Snapshot date on stats rows so growth can be computed later.
- FTS/search: titles + descriptions + transcripts.
- Cache freshness: OFF deliberately (quota-metered API; manual sync + doctor cache report). 

## User Vision
- Verbatim: "improve and reprint our youtube pp and make a real power horse for youtube analysis work … make no weak shit … make us fame." Target: analyst power-horse, not blogger embed helper.

## Known open bugs to fix by design (live-verified receipts)
1. Generator param drop: `channels-list` lacks `--id`, `search-list` lacks `--max-results` (both proven live 19/08). Spec declares them; the generated command must wire every declared param. Press 4.31.0 expected to fix; verify per-param after generate.
2. Bulk sync no-op (`no_bulk_list_endpoints`) → replaced by channel backfill sync design above.
3. Lost rich transcript (--format markdown/text, cache-alias, 4 tests) — salvage at `patches/salvage-20260814-youtube-transcript/`; restore.
4. Phase-5 marker/fingerprint + `pp:data-source` annotations — satisfied by running the current pipeline end-to-end.

## Product Thesis
- Name: youtube-pp-cli (Youtube Data)
- Why it should exist: every free tool is a stateless API mirror; every analysis tool is a paid SaaS. This CLI is the only free, offline-capable YouTube analyst: full read surface + local channel histories + statistically honest outlier/cadence/title analysis (median-based, sample-floor-guarded, per the 110-source method dossier) + transcripts without OAuth + a learning loop — all agent-native.

## Build Priorities
1. Channel backfill sync (the data path everything depends on).
2. Full read-only endpoint surface with complete param wiring (incl. videoCategories, i18n, channelSections, activities).
3. Analysis commands: outliers, cadence, titles (binding methods: median gap, floors 6/9/12, titles descriptive-only <100 videos; outlier ratio feeds titles).
4. Restore/carry all nine prior novels + rich transcript + learning loop.
5. MCP intent surface for the multi-step analysis workflows (user-accepted enrichment).

## Reachability Gate
- Decision: PASS
- Evidence: authenticated search.list returned 2xx (youtube#searchListResponse) live this session with the stored config key; unauthenticated probe of GET /youtube/v3/i18nRegions returned HTTP 403 (expected key-required response). No bot protection, official API.
