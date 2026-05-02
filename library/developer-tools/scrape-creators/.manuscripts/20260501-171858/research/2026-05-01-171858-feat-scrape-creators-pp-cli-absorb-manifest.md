# ScrapeCreators CLI Absorb Manifest (May 2026 regen)

## Scope

Spec: `https://docs.scrapecreators.com/openapi.json` — 114 GET endpoints, all read-only, across 23 platforms. The generator will emit one typed Cobra command per endpoint, plus the cross-platform compound commands listed in the Transcendence section.

Auth: api_key via `x-api-key`. Env var: `SCRAPE_CREATORS_API_KEY`.

---

## Absorbed (114 endpoints — generator emits all)

The full 114 endpoints are auto-emitted as typed commands by the generator and become typed MCP tools through the cobra-tree mirror. Below is a flagship selection per platform — the rest follow the same pattern.

### Account (4)
| # | Endpoint | Command | Note |
|---|---|---|---|
| 1 | `/v1/account/credit-balance` | `account balance` | Backs `account budget` transcendence |
| 2 | `/v1/account/get-api-usage` | `account api-usage` | Backs `account budget` |
| 3 | `/v1/account/get-daily-usage-count` | `account daily-usage` | |
| 4 | `/v1/account/get-most-used-routes` | `account most-used-routes` | |

### TikTok (24) — flagship subset
| # | Endpoint | Command |
|---|---|---|
| 1 | `/v1/tiktok/profile` | `tiktok profile <handle>` |
| 2 | `/v3/tiktok/profile/videos` | `tiktok videos <handle>` |
| 3 | `/v2/tiktok/video` | `tiktok video <url>` |
| 4 | `/v1/tiktok/video/transcript` | `tiktok transcript <url>` |
| 5 | `/v1/tiktok/video/comments` | `tiktok comments <url>` |
| 6 | `/v1/tiktok/user/audience` | `tiktok audience <handle>` |
| 7 | `/v1/tiktok/search/keyword` | `tiktok search keyword <q>` |
| 8 | `/v1/tiktok/search/hashtag` | `tiktok search hashtag <tag>` |
| 9 | `/v1/tiktok/get-trending-feed` | `tiktok trending` |
| 10 | `/v1/tiktok/song` | `tiktok song <id>` |
| 11 | `/v1/tiktok/song/videos` | `tiktok song-videos <id>` |
| 12 | `/v1/tiktok/shop/products` | `tiktok shop products` |

(+12 more: comment replies, hashtag search, top search, popular creators/hashtags, user followers/following/live/showcase, product/product reviews, shop search.)

### Instagram (12)
Profiles, posts, reels, story highlights, comments, transcripts, search reels, embed HTML.

### YouTube (12)
Channel details/videos/shorts, video/short details, transcripts, comments, comment replies, community posts, playlists, search, hashtag search, trending shorts.

### Facebook (12)
Profile, profile photos/posts/reels, posts, post comments, post transcripts, group posts. Ad Library: ad details, company ads, search ads, search companies.

### LinkedIn (6)
Profile, company, company posts, post. Ad library: search ads, ad details.

### Twitter/X (6)
Profile, user tweets, tweet details, tweet transcripts, community, community tweets.

### Reddit (7)
Subreddit posts/details/search, post comments, full search. Ads: search ads, ad details.

### Threads (5), Bluesky (3), TruthSocial (3), Twitch (3), Pinterest (4), Google (4)
All endpoints emitted as typed commands.

### Single-endpoint platforms (each becomes one command)
Snapchat, Kick, Amazon, detect-age-gender, Linktree, Komi, Pillar, Linkbio, Linkme.

---

## Transcendence (only possible with our SQLite + cross-platform approach)

### Reconciliation: prior 8 novel features

| # | Prior feature (v1.3.2) | Verdict | Rationale |
|---|---|---|---|
| 1 | `videos spikes @handle` | **KEEP + extend** | Was TikTok-only. Now multi-platform: TikTok/IG reels/YouTube shorts/Twitter all expose engagement counts. Rename to `content spikes`. |
| 2 | `transcripts search` | **KEEP + extend** | Was TikTok-only. Now FTS across **5 platforms with transcripts**: TikTok, YouTube, Instagram (v2), Facebook, Twitter. |
| 3 | `profile compare @h1 @h2` | **KEEP + extend** | Was within-platform. Now cross-platform (compare same handle on TikTok vs IG vs YouTube) and across-platform (creator A on TikTok vs creator B on YouTube). |
| 4 | `videos cadence` | **KEEP + extend** | Posting frequency by day/hour. Useful on every post-bearing platform. Rename to `content cadence`. |
| 5 | `profile track` | **KEEP + extend** | Daily follower snapshots. Now multi-platform per snapshot. |
| 6 | `account budget` | **KEEP** | More valuable now: `/v1/account/get-api-usage` + `get-most-used-routes` provide API-side data we can fuse with our local `usage_log` for projection. |
| 7 | `search trends` (hashtag delta) | **KEEP + extend** | Was TikTok hashtags. Now spans TikTok + YouTube + Instagram via per-platform hashtag-search endpoints. Rename to `trends delta`. |
| 8 | `videos analyze` (engagement ranker) | **KEEP + extend** | Was TikTok-only. Now `content analyze` across platforms; engagement formula varies per platform but normalized scoring works across the board. |

**No prior feature dropped.** All 8 stay; 6 of 8 are extended cross-platform with the new spec.

### New transcendence features (unlocked by 23-platform breadth)

| # | Feature | Command | Score | Why only we can do this |
|---|---|---|---|---|
| 9 | Cross-platform presence matrix | `creator find <handle>` | 9/10 | Probes profile endpoints across TikTok/IG/YouTube/Twitter/LinkedIn/Threads/Bluesky/Pinterest/Snapchat/Twitch/TruthSocial; returns "exists/doesn't" per platform with follower count. Single-call alternative does not exist. |
| 10 | Ad library unified search | `ads search <brand>` | 8/10 | Joins Facebook adLibrary/search/companies + Google adLibrary/advertisers/search + LinkedIn ads/search into one ranked result set. Three endpoints, one query, deduplicated by company name. |
| 11 | Link-in-bio universal resolver | `bio resolve <url>` | 7/10 | Auto-detects linktree.ee / komi.io / pillar.io / linkbio / linkme URLs and dispatches to the right endpoint, then normalizes the destination list. Five endpoints unified. |
| 12 | Trend triangulation | `trends triangulate <topic>` | 9/10 | Snapshots a hashtag/keyword across TikTok search hashtag + YouTube search hashtag + Reddit search + Threads search. Detects which platform a trend is rising fastest on — the cross-platform leading-indicator question no single endpoint answers. |
| 13 | Brand ad campaign monitor | `ads monitor <brand>` | 7/10 | Snapshots a brand's Facebook+Google+LinkedIn ads to SQLite; on rerun, diffs new vs gone. Cron-friendly. |

### Final scoring summary

| Feature | Command | Score | Status |
|---|---|---|---|
| Engagement spike detector | `content spikes <handle>` | 8/10 | Reconciled (extended) |
| Transcript FTS (5 platforms) | `transcripts search` | 9/10 | Reconciled (extended) |
| Multi-creator comparison | `profile compare` | 8/10 | Reconciled (extended) |
| Content cadence | `content cadence <handle>` | 7/10 | Reconciled (extended) |
| Follower growth tracker | `profile track` | 8/10 | Reconciled (extended) |
| Credit burn monitor | `account budget` | 7/10 | Reconciled |
| Hashtag trend delta | `trends delta` | 7/10 | Reconciled (extended) |
| Engagement-rate ranker | `content analyze` | 7/10 | Reconciled (extended) |
| Cross-platform presence | `creator find <handle>` | 9/10 | New |
| Ad library unified search | `ads search <brand>` | 8/10 | New |
| Link-in-bio resolver | `bio resolve <url>` | 7/10 | New |
| Trend triangulation | `trends triangulate` | 9/10 | New |
| Brand ad campaign monitor | `ads monitor <brand>` | 7/10 | New |

13 transcendence features, all ≥ 7/10. Every prior feature retained.

---

## Build priorities (Phase 3)

**Priority 0 (foundation)**
- SQLite store: `creators(handle, platform, ...)`, `content`, `comments`, `transcripts (FTS5)`, `ads`, `trends`, `usage_log`.
- `sync` command: per-platform sync hitting profile + recent content + transcripts.
- `search`, `sql` over local store.

**Priority 1 (absorbed)** — Generator emits all 114 endpoints. Hand-touch only ugly operationId names and skipped complex bodies (none expected since all GETs).

**Priority 2 (transcendence)** — 13 commands above. Order:
1. `account budget` (relies on existing endpoints + local `usage_log` — easy first win)
2. `creator find` (cross-platform fanout via existing profile endpoints)
3. `transcripts search` (FTS over local store)
4. `content spikes` / `content analyze` (engagement statistics over local store)
5. `content cadence` (date/hour aggregation over local store)
6. `profile track` (snapshot table + diff)
7. `profile compare` (rendered table over local store)
8. `ads search` (3-endpoint fanout, dedup)
9. `ads monitor` (snapshot diff over `ads` table)
10. `bio resolve` (URL detection + dispatch)
11. `trends delta` / `trends triangulate` (snapshot table + per-platform fanout)

Stubs: none planned. Every shipping-scope feature has an implementation path through existing endpoints + the local SQLite store.

**Priority 3 (polish)** — flag descriptions, ugly command names, README/SKILL narrative, MCP read-only annotations on novel commands.
