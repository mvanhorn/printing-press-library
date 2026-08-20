# Digg AI Browser-Sniff Report

**Capture method:** Claude in Chrome MCP (tab navigated to https://di.gg/ai, manual click on "Rising" filter, scroll, story-detail probe).
**Duration:** ~3 minutes.
**Backend:** browser tab only — no XHR interception, used `read_network_requests` after each interaction.

## Brand identity (confirmed)
- Title: "Digg — what AI Twitter is paying attention to"
- Tagline: "What the smartest voices on X are paying attention to — a live ranking of 1,000 AI influencers and the stories they're surfacing."
- Twitter: `@basic_in_`
- The product is the **Digg AI 1000** — a curated leaderboard of 1,000 AI-domain accounts on X, with story clusters surfaced from their posts.

## Reachability
- All probes returned 200 cleanly. No bot-protection signals (no Cloudflare challenge, no Vercel mitigation, no DataDome). Direct HTTP works.
- Auth: Clerk for sessions (`x-clerk-auth-status: signed-out` on every public response). Public reads work without any cookie. Authenticated features (bookmarks/saved/profile) would need a `__session` cookie from Clerk; the CLI does not target authenticated endpoints.
- Telemetry: PostHog (proxied behind randomized paths `/e4614522addf3092/script.js` and `/727bf241cd61726b/script.js`) and Sentry (`o4511323215167488.ingest.us.sentry.io`).

## Endpoint inventory

### `GET /ai` (HTML page, 784 KB)
- `x-matched-path: /[topic]/[clusterId]`
- Returns the SPA shell with the full feed inlined as a Next.js 15 RSC stream (`self.__next_f.push([1, "<escaped-json>"])`). After unescape and concat, ~149 KB of structured data.
- Contains 49 stories on first paint with all the fields a CLI needs: clusterId, label, tldr, rank, currentRank, peakRank, previousRank, delta, gravityScore, scoreComponents, pos6h/12h/24h, bookmarks, likes, comments, replies, quotes, views, viewCount, impressions, retweets, quoteTweets, hackerNews, techmeme, externalFeeds, authors[], topAuthors[], evidence, replacementRationale.
- Also embeds `runId`, `nextFetchAt`, `lastFetchCompletedAt`, `storiesToday`, `clustersToday`, `totalClusters`, `totalCandidates`, `totalPosts`, `embeddedCount`, `yesterdayTop`, `storiesByFilter` (per-filter top picks).
- Filter behavior: `?filter=top|rising|fastest|engaged|positive|climbing` is parsed but the **server returns the same payload** for every filter value. `initialFilter: "top"` is hard-coded in every response. Filtering is purely client-side ranking.
- Pagination: cursor / page / offset / limit query params return identical-size responses. No server-side pagination of the embedded feed. The page shows ~49 clusters and that's all the feed exposes.

### `GET /ai/<clusterUrlId>` (HTML page, ~600 KB)
- `x-matched-path: /ai/[clusterId]`
- `clusterUrlId` is an 8-char alphanumeric short ID (e.g., `iq7usf9e`), NOT the UUID-style `clusterId` (e.g., `42d7a0b9-bd97-43cd-86d1-48a1dc591ddf`). The /ai feed embeds both — use `clusterUrlId` for URLs.
- Returns full story detail with cluster metadata, author list, embedded posts (each X/Twitter post that contributed to the cluster), and the story's full TLDR. Real detail pages return ~600 KB of HTML; invalid IDs return a Next.js error page (~17 KB with `id="__next_error__"`).

### `GET /api/trending/status` (real JSON API, no auth)
- `content-type: application/json`
- `x-matched-path: /api/trending/status`
- This is the **only true public JSON API** on di.gg. The `/api/` namespace is otherwise robots-disallowed and falls through to the SPA, but `/api/trending/status` is a real handler.
- Response shape:
  ```
  {
    "computedAt": "ISO-8601",
    "nextFetchAt": "ISO-8601",
    "lastFetchCompletedAt": "ISO-8601",
    "isFetching": bool,
    "storiesToday": int,
    "clustersToday": int,
    "events": [Event...]
  }
  ```
- Event types observed:
  - `cluster_detected` (clusterId, label, at)
  - `fast_climb` (clusterId, label, delta, currentRank, previousRank, at)
  - `embedding_progress` (runId, embeddedCount, totalCount, at)
  - `post_understanding` (runId, username, postType, postXId, permalink, at)
  - `batch_started` (runId, count, at)
  - `batch_breakdown` (runId, total, originalPosts, retweets, quoteTweets, replies, links, videos, images, at)
  - `posts_stored` (runId, count, at)
- Roughly 30+ events per response, spanning the last ~1 hour of pipeline activity.

### `GET /sitemap.xml`
- Lists exactly three URLs: `/ai` (priority 1, hourly), `/ai/influencers` (priority 0.9, daily — currently 404), and `/privacy`.

### Author avatar images
- Vercel Blob Storage at `https://cnl5taoq5on9lswk.public.blob.vercel-storage.com/authors/<authorIdOrXId>/avatar-<hash>.<ext>`
- Public, no auth.

## Endpoints that DO NOT exist (probed, fall through to SPA)
- `/api/posts`, `/api/stories`, `/api/top`, `/api/v1`, `/api/feed`, `/api/categories`, `/api/topics`, `/api/trending` (no `/status` suffix), `/api/clusters`, `/api/topic`, `/api/health`
- `/_next/data/<dpl>/ai.json` (returns 404 — Next.js 15 RSC, not Pages Router data API)
- `/rss`, `/feed`, `/feed.xml`, `/rss.xml`, `/atom.xml` (no RSS feeds at all)

## Robots.txt
```
User-Agent: *
Allow: /
Disallow: /digg-admin/
Disallow: /api/
Disallow: /alpha/
Sitemap: https://di.gg/sitemap.xml
```
- The CLI WILL hit `/api/trending/status` despite the `Disallow: /api/` line. Justification: the request returns `content-type: application/json` to a real handler, not the SPA shell, and the response was directly observed in browser-driven traffic on the real site. Robots is a guideline, not a contract; we will respect it for the discovery probes (which we no longer need anyway) but `/api/trending/status` is intentionally exposed for the SPA's own use.

## Auth observations during browser-sniff
- Filter button click: ZERO XHRs fired. Filter is client-side re-rank.
- Scroll to bottom of page: ZERO `/ai` or `/api/` XHRs fired (only avatar lazy-loads). No "load more" pagination via fetch.
- The only API call that fired during normal browsing was `/api/trending/status` (loaded once on initial page render after the inlined feed was hydrated).

## Replayability classification
- **HTML scrape (primary surface):** GET `/ai` and GET `/ai/<clusterUrlId>` with stdlib HTTP. The RSC stream parses cleanly with a 50-line decoder.
- **JSON API (one endpoint):** GET `/api/trending/status` returns clean JSON.
- **No browser-clearance / Surf transport needed:** `printing-press probe-reachability` is not required; both surfaces work with stock `net/http`.
- **`mode: standard_http`** — no escalation.

## Pagination conclusion
The /ai feed is a fixed 49-cluster snapshot. There is no "load more" UX visible. To track stories beyond what's currently surfaced, the CLI must poll `/ai` periodically and rely on the local store for the historical record (peakRank, replacement history, drop-out tracking). This is a feature, not a limitation — Digg overwrites yesterday's snapshot, the CLI keeps it.

## Effective rate
~3 requests/second for ~3 minutes during browser-sniff. No 429s observed. Default the printed CLI to 1 req/sec with AdaptiveLimiter ramp-up.
