# Digg AI CLI Brief

## API Identity
- Domain: AI-news aggregation and discovery (https://di.gg)
- Users: developers, researchers, founders, and AI-curious readers tracking the AI news cycle
- Data profile: clusters of news stories about AI, ranked by a multi-signal gravity score with cross-aggregator references (Hacker News, Techmeme), Twitter/X engagement signals, AI-generated TLDRs, and time-windowed positivity metrics

## Reachability Risk
- None. Direct HTTP returns 200 cleanly with no bot-protection headers (no Cloudflare challenge, no Vercel mitigation, no DataDome).
- Caveat: di.gg is a separate, AI-news-only deployment of Digg. The main digg.com general-Reddit-rival relaunch was shut down March 14 2026 over a "brutal" AI-agent bot problem (per CEO Justin Mezzell). di.gg appears to be the small-team rebuild Mezzell announced. The platform is alive and serving today; the operational risk is reputational/policy, not technical.

## Source Surface
There is no documented public API. Discovery so far:

- **Single live topic**: `/ai` (sitemap also lists `/ai/influencers` but it currently 404s — likely planned, not yet wired)
- **Story dynamic route**: `/ai/[clusterId]` (matched route name `/[topic]/[clusterId]`)
- **Filters via query string**: `/ai?filter=top|rising|fastest|engaged|positive|climbing` (all return 200 to the same matched-path; server reads filter from query)
- **No public REST API**: every `/api/*` path falls through to the dynamic route and returns the SPA HTML (`x-matched-path: /[topic]/[clusterId]`)
- **`/digg-admin/`, `/api/`, `/alpha/` blocked by robots.txt**; `/ai`, `/sitemap.xml`, `/privacy` allowed
- **Stack**: Next.js 15 on Vercel with React Server Components (header `vary: rsc, next-router-state-tree, next-router-prefetch, next-router-segment-prefetch`)
- **Auth**: Clerk (header `x-clerk-auth-status: signed-out` on every public response). Note: the main digg.com uses Privy with embedded crypto wallets per Kevin Rose; di.gg uses plain Clerk sessions, no wallet/crypto layer.

## Data Layer
The full feed payload is embedded in the `/ai` HTML response as a `self.__next_f.push([1, "..."])` RSC stream. Decoding 11 pushes from a single home-page fetch yields ~149 KB of JSON-shaped data including 49 fully populated story records.

Field catalog observed in the embedded data (141 unique field names):

**Story core:**
`clusterId`, `clusterUrlId`, `shortId`, `label`, `title`, `tldr`, `url`, `permalink`, `imageUrl`, `images`, `videos`

**Ranking and scoring:**
`rank`, `currentRank`, `peakRank`, `previousRank`, `delta`, `gravityScore`, `scoreComponents`, `scores`, `numeratorCount`, `numeratorLabel`, `pageAverageRatio`, `percentAboveAverage`, `composite`

**Time-windowed positivity:**
`pos6h`, `pos12h`, `pos24h`, `posLast` — sentiment positivity per 6/12/24-hour window

**Engagement and reach:**
`bookmarks`, `likes`, `comments`, `replies`, `quotes`, `views`, `viewCount`, `impressions`, `retweets`, `quoteTweets` (Twitter/X signals integrated)

**Cross-aggregator references:**
`hackerNews`, `techmeme`, `externalFeeds` — Digg explicitly tracks where the same story is being discussed elsewhere

**Authors and influencers:**
`authors`, `topAuthors`, `contributors`, `uniqueAuthors`, `rankedAuthorCount`, `username`, `displayName`, `avatarUrl`, `xId`, `postXId`, `badges`, `influence`, `podist`

**Activity timeline:**
`activityAt`, `createdAt`, `computedAt`, `firstPostAt`, `lastFetchCompletedAt`, `lastFrozenPostAt`, `nextFetchAt`

**Topic-level aggregates:**
`topic`, `basePath`, `view`, `storiesByFilter`, `initialFilter`, `totalClusters`, `totalCandidates`, `totalPosts`, `embeddedCount`, `storiesToday`, `clustersToday`, `yesterdayTop`

**Transparency / explainability:**
`evidence`, `replacementRationale`, `reason`, `details`, `runId` — Digg explicitly exposes WHY a story replaced another in rankings and what evidence supports its placement

**Render hints:**
`renderVersion` (e.g., `friendly-tldr-topic-feed-v1`, `frozen-midnight-v1`)

**Local store plan:**
- Primary entities: `clusters` (stories), `authors`, `cluster_authors` (M:N), `cluster_snapshots` (rank/score history over time), `cross_refs` (HN/Techmeme links)
- Sync cursor: per-filter timestamp + `runId` for deduplication
- FTS: `clusters_fts` over `title|tldr|label`
- The local store unlocks rank-delta history, peakRank tracking, and "what stories did Digg de-rank and why" queries that the live site shows only at one moment

## Codebase Intelligence
- DeepWiki / source code analysis: not applicable (Digg is closed-source; only the public Next.js client surface is visible)
- Auth: Clerk session cookies (`__session`, `__client_uat`); read paths public, write paths private
- Rate limiting: not advertised. Default to conservative AdaptiveLimiter (1 req/sec) with backoff on 429
- Architecture: Next.js 15 RSC, single-topic right now, runId-stamped feeds suggest a periodic cron-driven aggregation pipeline

## User Vision
Not provided at briefing — user said "let's go".

## Source Priority
Single source — di.gg only — so no priority gate needed.

## Product Thesis
- Name: `digg-pp-cli`
- Why it should exist: Digg AI's value is a curated, transparently-ranked AI news feed with rich cross-aggregator signals (HN + Techmeme + X). The web UI shows you today's snapshot. A CLI with a local store turns it into a research tool: rank-history, "what got buried and why", author influence trends, and offline search across days/weeks of feed snapshots that the web UI silently overwrites. No CLI, scraper, MCP server, or wrapper exists today (verified npm, PyPI, GitHub for "digg" and "di.gg" — only pre-2012 historic clients).
- Constraint: read-only. Digg explicitly shut down its main site over AI-bot vote and engagement abuse. Building automated voting / bookmarking / commenting would be exactly the abuse the CEO publicly named. The CLI must NOT include any mutation features.

## Build Priorities
1. **Sync + local store** — fetch `/ai`, `/ai?filter={top,rising,fastest,engaged,positive,climbing}`, and `/ai/[clusterId]` detail pages; parse the embedded RSC stream; persist clusters, authors, snapshots, and cross-refs.
2. **Read commands matching every HN-CLI feature** — top/rising/best/search/story/comments-equivalent, --json/--select/--csv, browser-open with `digg open <id>`, agent-native output, bounded responses.
3. **Transcendence** — features only possible because we have a local store and Digg's transparency fields:
   - `digg deltas` — rank movers since last sync (rank delta, peakRank changes)
   - `digg replaced --since 24h` — show stories that were knocked out of rankings, with Digg's own `replacementRationale`
   - `digg sentiment <id> --window 6h|12h|24h` — read pos6h/pos12h/pos24h trends
   - `digg crossref <id>` — show this story on Hacker News and Techmeme via Digg's own `hackerNews`/`techmeme` fields
   - `digg authors top --by influence` — top authors by Digg's influence score
   - `digg watch` — poll, diff, alert on rank movement (read-only)
   - `digg evidence <id>` — print the `evidence` and `scoreComponents` Digg exposes for ranking transparency
4. **Browser-sniff (optional)** — verify pagination and filter-mode behavior; expose any RSC-fetch endpoints the SPA uses for "load more"

## Ethical and Operational Notes
- Use a clear, identifying User-Agent: `digg-pp-cli/<version> (+https://github.com/mvanhorn/digg-pp-cli)`
- Default rate limit: 1 req/sec with AdaptiveLimiter; respect 429s
- Respect `robots.txt`: only hit `/ai`, `/ai/[clusterId]`, `/sitemap.xml`, `/privacy`. Never hit `/api/`, `/digg-admin/`, `/alpha/`.
- No mutation features. Period. Reads only.
