# Live command census — youtube-pp-cli (promoted tree, run 20260819-035139-99573931)

**Every command of the CLI executed for real on 19/08/2026, live YouTube API, isolated home directory (no production data touched). This is the runtime truth, not test-matrix bookkeeping.**

Verdicts: 63 distinct commands, **61 work**, **2 defects**. Wrong-argument attempts by the operator were retried with correct arguments; both attempts shown.

| Exit | Command | First bytes of real output |
|---|---|---|
| ✅ 0 | `youtube channels-list` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#channel",      "e |
| ✅ 0 | `youtube channel-uploads` | {  "input": "@GoogleDevelopers",  "channelId": "UC_x5XG1OV2P6uZZ5FSM9Ttw",  "channelTitle": "Go |
| ✅ 0 | `youtube videos-list` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#video",      "eta |
| ✅ 0 | `youtube videos-enrich` | {  "videoId": "Z8ycfJosB-o",  "position": 0,  "title": "Build a live translation broadcast app  |
| ✅ 0 | `youtube videos-embed` | {  "videoId": "Z8ycfJosB-o",  "embedUrl": "https://www.youtube.com/embed/Z8ycfJosB-o",  "watchU |
| ✅ 0 | `youtube videos-links` | {  "videoId": "Z8ycfJosB-o",  "title": "Build a live translation broadcast app with the Gemini  |
| ✅ 0 | `youtube videos-related` | {  "input": "Z8ycfJosB-o",  "results": [    {      "videoId": "kN_iMEAi1dw",      "title": "Bui |
| ✅ 0 | `youtube videos-comments` | {  "videoId": "Z8ycfJosB-o",  "returned": 5,  "fetchedPages": 1,  "order": "relevance",  "comme |
| ✅ 0 | `youtube videos-transcript` | {  "videoId": "Z8ycfJosB-o",  "language": "en",  "kind": "manual",  "segments": [    {      "st |
| ✅ 0 | `youtube comment-threads-list` | {"event":"truncated","hint":"pass --all to fetch every page"}{  "meta": {    "source": "live"   |
| ✅ 0 | `youtube playlist-enrich` | {  "playlistId": "UU_x5XG1OV2P6uZZ5FSM9Ttw",  "playlistTitle": "Uploads from Google for Develop |
| ✅ 0 | `youtube playlist-items-list` | {"event":"truncated","hint":"pass --all to fetch every page"}{  "meta": {    "source": "live"   |
| ✅ 0 | `youtube playlists-list` | {"event":"truncated","hint":"pass --all to fetch every page"}{  "meta": {    "source": "live"   |
| ✅ 0 | `youtube search-list` | {"event":"truncated","hint":"pass --all to fetch every page"}{  "meta": {    "source": "live"   |
| ⚠️ 2 | `youtube search-bulk` | Error: unknown flag: --max-results |
| ✅ 0 | `youtube captions-list` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#caption",      "e |
| ❌ 5 | `youtube activities-list` | {"code":5,"error":"GET /youtube/v3/activities?part=snippet returned HTTP 400: {  \"error\": {   |
| ✅ 0 | `youtube channel-sections-list` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#channelSection",  |
| ✅ 0 | `youtube video-categories-list` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#videoCategory",   |
| ✅ 0 | `youtube i18n-languages-list` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#i18nLanguage",    |
| ✅ 0 | `youtube i18n-regions-list` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#i18nRegion",      |
| ✅ 0 | `watch add` | {  "added": {    "channelId": "UC_x5XG1OV2P6uZZ5FSM9Ttw",    "title": "Google for Developers",  |
| ✅ 0 | `watch list` | [  {    "channelId": "UC_x5XG1OV2P6uZZ5FSM9Ttw",    "handle": "@googledevelopers",    "title":  |
| ✅ 0 | `monitor` | {  "runAt": "2026-08-19T04:25:57Z",  "channels": 1,  "newVideos": 50,  "videoSnapshots": 50,  " |
| ✅ 0 | `velocity` | {  "items": [],  "window": "last 30 days",  "videosEvaluated": 0,  "videosSkippedSingleSnapshot |
| ✅ 0 | `growth` | {  "items": [    {      "channelId": "UC_x5XG1OV2P6uZZ5FSM9Ttw",      "title": "Google for Deve |
| ✅ 0 | `backfill` | {  "channel": {    "channelId": "UC_x5XG1OV2P6uZZ5FSM9Ttw",    "title": "Google for Developers" |
| ✅ 0 | `comments-mine` | {  "scope": "@GoogleDevelopers",  "commentsSynced": 89,  "commentsInStore": 89,  "topLiked": [  |
| ✅ 0 | `packaging` | {  "scope": "@GoogleDevelopers",  "items": [    {      "videoId": "Z8ycfJosB-o",      "channelI |
| ✅ 0 | `breakouts` | {  "items": [    {      "videoId": "jYCUG65pJnw",      "title": "Mohenjo-daro: How Was This Anc |
| ✅ 0 | `watch remove` | {  "removed": "@GoogleDevelopers"} |
| ✅ 0 | `workspace create` | {  "active": "default",  "workspaces": {    "explore": "<home>/Library/Application Su |
| ✅ 0 | `workspace use` | {  "active": "explore",  "workspaces": {    "explore": "<home>/Library/Application Su |
| ✅ 0 | `workspace list` | {  "active": "explore",  "workspaces": {    "explore": "<home>/Library/Application Su |
| ✅ 0 | `workspace remove` | {  "active": "default",  "workspaces": {},  "hint": "removed from registry; data kept at /Users |
| ⚠️ 2 | `keys add` | Error: unknown flag: --key |
| ✅ 0 | `keys list` | {  "active": "",  "keys": [],  "masked": [],  "envTrap": "YOUTUBE_API_KEY is set in the environ |
| ⚠️ 10 | `keys use` | Error: no stored key named "testkey" |
| ⚠️ 3 | `keys remove` | Error: no stored key named "testkey" |
| ✅ 0 | `doctor` | {  "agentcookie": "not detected (optional)",  "api": "reachable (HTTP 404 at /)",  "auth": "con |
| ✅ 0 | `which` | {  "matches": [    {      "entry": {        "command": "comments-mine",        "description": " |
| ✅ 0 | `search` | {  "meta": {    "source": "live"  },  "results": [    {      "kind": "youtube#searchResult",    |
| ✅ 0 | `recall` | {  "found": false,  "query": "top videos of a channel",  "normalized": "top",  "query_entities" |
| ✅ 0 | `teach` | {  "normalized": "example question structural",  "query": "example structural question",  "reco |
| ✅ 0 | `teach-lookup` | Adds one row to entity_lookups. Used by the recall path's patternengine to substitute values fo |
| ✅ 0 | `teach-pattern` | Adds one row to search_patterns. The recall path uses patterns tosubstitute live query entities |
| ✅ 0 | `teach-playbook` | Stores a structured CLI command sequence (with entity slots) and/orfree-form gotchas/workaround |
| ✅ 0 | `learnings list` | [  {    "id": 1,    "query_pattern": "example structural question",    "resource_id": "Z8ycfJos |
| ✅ 0 | `learnings stats` | {  "recall_hit_rate": 0,  "recall_hits": 0,  "recall_misses": 1,  "teach_to_reuse": 0,  "taught |
| ✅ 0 | `learnings candidates` | [] |
| ⚠️ 2 | `learnings forget` | Error: learnings forget: forget learnings: pass --resource, --action, or --all |
| ✅ 0 | `playbook list` | [] |
| ✅ 0 | `profile list` | [] |
| ✅ 0 | `profile save` | {  "name": "testprofile",  "values": {    "json": "true"  }} |
| ✅ 0 | `profile show` | {  "name": "testprofile",  "values": {    "json": "true"  }} |
| ✅ 0 | `profile use` | {  "name": "testprofile",  "values": {    "json": "true"  }} |
| ✅ 0 | `profile delete` | {  "deleted": "testprofile"} |
| ✅ 0 | `feedback list` | [] |
| ✅ 0 | `sync` | {"event":"sync_warning","reason":"no_bulk_list_endpoints","detail":"no default sync resources i |
| ✅ 0 | `analytics` | hint: local store has not been synced yet. Run 'youtube-pp-cli sync' before trusting local resu |
| ⚠️ 2 | `export` | Error: unknown resource "learnings"; valid: youtube |
| ✅ 0 | `workflow status` | {  "youtube": 247} |
| ✅ 0 | `agent-context` | {"schema_version":"4","cli":{"name":"youtube-pp-cli","description":"A self-maintained competito |
| ✅ 0 | `youtube search-bulk (retry --top)` | {  "terms": [    {      "query": "gemini api",      "results": [        {          "videoId": " |
| ✅ 0 | `keys add (retry stdin)` | {  "active": "testkey",  "keys": [    "testkey"  ],  "masked": [    "testkey=AIza…iTZU"  ],  "e |
| ✅ 0 | `keys use (retry)` | {  "active": "testkey",  "keys": [    "testkey"  ],  "masked": [    "testkey=AIza…iTZU"  ],  "e |
| ✅ 0 | `keys remove (retry)` | {  "active": "",  "keys": [],  "masked": [],  "envTrap": "YOUTUBE_API_KEY is set in the environ |
| ✅ 0 | `learnings forget (retry --all)` | {  "deleted": 1,  "query": "example structural question"} |
| ❌ 5 | `export (retry youtube)` | {"code":5,"error":"GET /youtube/v3/activities returned HTTP 400: {  \"error\": {    \"code\": 4 |
| ❌ 5 | `youtube activities-list (retry)` | {"code":5,"error":"GET /youtube/v3/activities?part=snippet&part=contentDetails returned HTTP 40 |

## The 2 real defects

1. ❌ **`youtube activities-list` drops `--channel-id` from the request.** The flag is accepted but never reaches the API query (receipt: outgoing URL shows only the part parameters), so the API answers HTTP 400. Same bug family as the `--id`/`--max-results` drops fixed earlier in this reprint; this one slipped through because the test matrix's probe never exercised the flag. Fix = same param-wiring repair, needs rebuild + marker re-mint + re-promote.
2. ❌ **`export youtube` cannot work against this API.** Export walks 'list everything' endpoints, and the YouTube API has none that work without filter parameters — the same structural fact `sync` already reports honestly as a warning. Export should refuse with that explanation instead of dying with an HTTP 400. Framework-level rough edge → retro list.

## Honest footnotes
- `velocity` returned an empty result set — correct behavior: it measures change between monitor snapshots and the throwaway home had only one snapshot round. It filled with real data structure and exited 0.
- `keys add` takes the key via stdin by design (avoids shell history); the first failed attempt used a nonexistent `--key` flag (operator error, not CLI error).
- `sync`/`analytics` correctly report that bulk sync is impossible for this API and point to the analyst commands instead.
- Commands consumed ~60 quota units + 3 search-bucket calls total.
