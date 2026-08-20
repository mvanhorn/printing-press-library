# Tech Twitter CLI Brief

## API Identity
- **Domain:** Curated high-signal tech discourse from X/Twitter. Tech Twitter ("We doomscroll, you upskill") ingests, dedupes, summarizes, quality-scores, and topic-tags tweets/articles, then exposes them through (a) a public **agent-evidence surface** designed for machine clients and (b) a set of **public read APIs**. Domain: techtwitter.com.
- **Users:** AI agents that need *cited* evidence for "what changed / launched / is being argued / worth reading"; developers and researchers who track tech Twitter without the firehose; tooling that wants a deduped, quality-scored stream instead of raw X.
- **Data profile:** Curated tweet rows with engagement metrics + a computed `quality_score`, Claude-written `summary`, SEO title, keywords, topic tags, and `content_type` (`tweet` | `article`). Plus products, author profiles, in-app newsletters ("Threads"), topic momentum, and command-center signals.

## Reachability Risk
- **None / Low.** Every targeted endpoint returns live HTTP 200 (captured 2026-06-14). No auth required for the public surface.
- **Two reachability rules the client must honor:**
  1. **Browser User-Agent is mandatory.** A bare `curl/*` UA gets `403 Forbidden` on every non-machine path (`lib/bot-detection.ts` allows scraper UAs only on `/llms*.txt`, `/.well-known/agent-card.json`, `/api/agent/context`, `/api/a2a`). A normal Chrome UA returns 200. → CLI client default `User-Agent` must be browser-shaped.
  2. **Allowlist gate.** `lib/public-routes.ts:PUBLIC_API_PATHS` + a tweet-detail UUID regex define the unauthenticated surface. Non-allowlisted API paths `307`-redirect to `/` (Supabase session proxy). The CLI must only target allowlisted endpoints (see Data Layer); auth-only endpoints (`/api/topics` root list, `/api/tweets/dates|today-top|yesterday-top|archive`, `/api/profiles/[handle]`, `/api/command/narratives|streams|author-watch|engagement-breakdown|threads`) are **out of scope** — they redirect for anonymous callers.
- Probe-safe endpoint used: `GET /api/agent/context?kind=auto` (200, 16 KB evidence bundle).

## Top Workflows
1. **Semantic search over curated tweets** — `GET /api/tweets/search?q=` (`{query, searchType, totalCount, tweets[]}`). The single highest-value command; returns relevance-ranked, deduped, summarized tweets.
2. **Agent evidence bundles** — `GET /api/agent/context?kind={auto|what-changed|launches|arguments|read-list|narrative-alert}&question=&q=&topic=&window={24h|48h|7d}&limit=`. Returns `{evidence[], facets, instructions, next}` with per-row provenance, `qualityScore`, `whyIncluded`, and `canonicalUrl` citations. Mirrors the A2A `SendMessage` skill set.
3. **Browse the curated stream** — trending (`/api/tweets/trending`), latest (`/api/tweets/latest`, single tweet + `nextTweetId`), by author (`/api/tweets/author?handle=`), by topic (`/api/tweets/topic?slug=`), by month (`/api/tweets/monthly?year=&month=`), by date (`/api/command/tweets-by-date?date=`), single tweet (`/api/tweets/{uuid}`).
4. **Read & discover** — long-form articles (`/api/articles?limit=&offset=`), product launches (`/api/products`), in-app newsletters (`/api/newsletters`), author profiles (`/api/profiles/search?q=`), RSS (`/rss`).
5. **Command-center signals** — hot-takes / high-reply debates (`/api/command/hot-takes` → `{count, tweets[]}` with `replyRatio`), main character of the day (`/api/command/main-character` → `{authors[]}`), topic momentum heatmap (`/api/command/heatmap` → `{topics[]}` with `keyword,count,engagement,slug`), corpus stats (`/api/command/stats`).

## Table Stakes (from adjacent tools — no direct competitor exists)
There is no existing CLI for *this* API (it is the user's own site). Table-stakes are drawn from adjacent categories: HN CLIs (`hackernews` readers), RSS/feed readers, X/Twitter CLIs, and agent-evidence/MCP tools.
- list / search / get with filters and `--limit`
- structured `--json` output that pipes cleanly to `jq`
- a "latest / trending / front-page" feed command
- offline cache so repeat reads don't re-hit the network
- open-in-browser / canonical-URL surfacing
- author/feed following and per-author history
- agent/MCP exposure of the read surface

## Data Layer
- **Primary entities (syncable):**
  - `tweets` — core. Fields: `id` (uuid), `tweet_url`, `tweet_text`, `summary`, `author_handle`, `author_name`, `bookmark_count`, `comment_count`, `like_count`, `retweet_count`, `quality_score`, `content_type` (`tweet`|`article`), `created_at`, `timestamp`, `slug`, `seo_title`, `keywords`, `tweet_topics`/`topics`, media (`image_url`, `video_url`, `link_card_*`), `is_pinned`.
  - `articles` — `content_type='article'` rows + `article_title`, `slug` (`/api/articles` → `{articles, limit, offset, total}`).
  - `products` — `{id, name, tagline, url, logo_url, category, created_at}`.
  - `profiles` (authors) — `{handle, name, avatar_url, bio, follower_count, following_count, location, tweet_count, total_engagement, most_recent_tweet}`.
  - `newsletters` — `{id, title, subtitle, slug, published_at}`.
  - `topics` (momentum) — `{keyword, slug, count, engagement}` from heatmap.
- **Sync cursor:** `created_at` / `timestamp` (ISO8601), newest-first; trending/search are the broad pull sources, author/topic/monthly are scoped pulls.
- **FTS:** tweet/article text over `tweet_text` + `summary` + `seo_title` + `keywords` + `author_name`; products over `name` + `tagline`; profiles over `handle` + `name` + `bio`.

## Codebase Intelligence
- Source: the live Next.js 16 repo (`/Users/danielkhunter/Code/tech-twitter`) is the source of truth, cross-checked against live responses.
- Auth: **none** for the public surface; Supabase session only gates write/admin and non-allowlisted routes.
- Ranking: server-side `score = (bookmark*4 + comment*3 + retweet*2 + like) / ((hours+2)^1.5)`; `quality_score` is precomputed per row.
- Reachability gate: `lib/public-routes.ts` allowlist + `lib/bot-detection.ts` UA filter (see Reachability Risk).

## User Vision
- User chose "Full public API" scope (read endpoints + agent-evidence surface) and "Let's go". No additional feature constraints volunteered. This is the user's own site.

## Product Thesis
- **Name:** `techtwitter-pp-cli` (binary); display name **Tech Twitter**.
- **Why it should exist:** Tech Twitter's value is *curation* — a deduped, quality-scored, summarized, topic-tagged slice of tech X. A CLI turns that into an **offline-searchable, agent-native local mirror**: FTS over curated rows, evidence bundles with baked-in citations, and a local SQLite store that **compounds** — narrative/topic momentum over time, day-over-day trending diffs, and per-author engagement history that no single stateless API call can answer. One binary, no key, no auth.

## Build Priorities
1. **Data layer + sync + FTS** over tweets/articles/products/profiles/newsletters/topics (Priority 0). Browser-UA client; allowlist-only endpoints.
2. **Absorb everything reachable** (Priority 1): search, trending, latest, author, topic, monthly, tweet get, by-date, articles, products, profiles search, newsletters, hot-takes, main-character, heatmap, stats, agent-context evidence (all 6 kinds), a2a evidence.
3. **Transcend** (Priority 2): offline momentum/narrative tracking, trending day-over-day diff, author engagement history, local evidence-bundle composition, read-list/digest — all powered by the compounding local store.
