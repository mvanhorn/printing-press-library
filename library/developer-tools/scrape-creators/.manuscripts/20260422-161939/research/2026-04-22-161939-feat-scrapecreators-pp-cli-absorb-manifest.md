# ScrapeCreators CLI Absorb Manifest

## Scope Note
User chose "existing spec only" — 13 TikTok spec endpoints are the build scope.
Examples repo (twitter.js, instagram.js, youtube.js, fbAds.js, etc.) represents future platform expansion.

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | TikTok profile fetch | spec + examples/scrapeProfilesTiktoks.js | `profile get @handle` | Offline cache, --json, --select |
| 2 | Audience demographics | spec /v1/tiktok/user/audience | `profile audience @handle` | SQLite-backed, historical diffs, --json |
| 3 | Profile videos (paginated) | spec /v3/tiktok/profile/videos | `videos list @handle` | Sync to local store, --sort-by latest/popular |
| 4 | Single video info | spec /v2/tiktok/video | `video get <url>` | --json, --select, transcript on demand |
| 5 | Video transcript | spec /v1/tiktok/video/transcript | `video transcript <url>` | FTS-indexed locally, --language |
| 6 | Live stream status | spec /v1/tiktok/user/live | `profile live @handle` | --watch flag for polling, --json |
| 7 | Video comments (paginated) | spec /v1/tiktok/video/comments | `video comments <url>` | SQLite store, sentiment analysis hook |
| 8 | Comment replies | spec /v1/tiktok/video/comment/replies | `video replies <comment-id> <url>` | Threaded, --json |
| 9 | User following list | spec /v1/tiktok/user/following | `profile following @handle` | Sync to store, pagination auto-handled |
| 10 | User followers list | spec /v1/tiktok/user/followers | `profile followers @handle` | Sync to store, diff tracking |
| 11 | User search by keyword | spec /v1/tiktok/search/users | `search users <query>` | Local FTS after sync |
| 12 | Hashtag video search | spec /v1/tiktok/search/hashtag | `search hashtag <tag>` | Paginated, --limit, SQLite-backed |
| 13 | Keyword video search | spec /v1/tiktok/search/keyword | `search keyword <query>` | --date-posted, --sort-by, --region |
| 14 | Profile + video batch sync | examples/scrapeProfilesTiktoks.js | `sync @handle` | Full sync of profile+videos+metadata |
| 15 | Trending song lookup | examples/getTrendingSongs.js | `trends songs` | Time-series snapshots in SQLite |
| 16 | Videos by song | examples/getTiktoksFromSong.js | `song videos <song-id>` | Paginated, --json |
| 17 | TikTok video download | examples/downloadTiktoks.js | `video download <url>` | No-watermark URL from video metadata |
| 18 | Account credits check | docs /account/balance | `account credits` | Show remaining credits + usage rate |

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Engagement spike detector | `videos spikes @handle --threshold 2x` | 9/10 | Requires historical video stats in SQLite to compute creator's average and flag outliers |
| 2 | Transcript full-text search | `transcripts search "keyword" --handle @handle` | 9/10 | Batch-fetches + FTS5-indexes transcripts from all synced videos; no single API call provides this |
| 3 | Content cadence analysis | `videos cadence @handle` | 8/10 | Posting frequency by day/hour from synced video timestamps — reveals creator's optimal posting patterns |
| 4 | Follower growth tracker | `profile track @handle` | 8/10 | Daily follower snapshots in SQLite enable growth curves no single API call provides |
| 5 | Credit burn rate monitor | `account budget` | 7/10 | Logs usage per command in SQLite; projects days-remaining at current pace |
| 6 | Hashtag trend delta | `search trends --hashtag <tag> --days 7` | 7/10 | Snapshots hashtag video counts over time; detects rising vs declining trends |
| 7 | Multi-creator comparison | `profile compare @handle1 @handle2` | 8/10 | Normalizes follower counts, engagement rates, posting cadence across synced profiles |
| 8 | Engagement rate calculator | `videos analyze @handle` | 7/10 | Computes (likes+comments+shares)/views per video from synced data; ranks by engagement |
