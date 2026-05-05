# Hacker News CLI Brief

## API Identity
- **Domain:** Tech news and discussion. The classic site for posting, ranking, and discussing technology stories, project launches, and "Ask HN" / "Show HN" / "Who is Hiring" threads.
- **Users:** Hackers, founders, engineers, researchers, recruiters, journalists tracking discourse.
- **Data profile:** Two read-only public APIs — Firebase Realtime (`hacker-news.firebaseio.com/v0`) returns the canonical item/user/list bodies; Algolia HN Search (`hn.algolia.com/api/v1`) returns full-text search, date ranges, and aggregated hit metadata. No write surface, no auth, no rate limits in practice (both Google CDN-fronted).

## Reachability Risk
- **None.** Both Firebase HN (`200` on `/v0/topstories.json`) and Algolia HN (`200` on `/api/v1/search`) responded immediately during this session's reachability probe (2026-05-04). Both APIs have been stable for >10 years; no recent issue tracker reports of blocking or breakage.

## Top Workflows
1. **Browse & filter front page** — show top/new/best stories, narrow to Show/Ask/Job, follow comment threads. The dominant workflow shared by every existing CLI (haxor-news, circumflex, hn-cli, hacker-feeds-cli).
2. **Search the full corpus by topic** — "what is HN saying about X" — only practical via Algolia, requires date filters, sort by points/comments/recency. Power users care about both popular hits and recent buzz.
3. **Mine 'Who is Hiring' / 'Freelancer? Seeking Freelancer?' threads** — confirmed killer use case (HNHIRING.com is a popular service; haxor-news has dedicated `hiring` regex command; n8n workflow templates exist). Recruiters, job-seekers, and trend-watchers all need structured access.
4. **Track a user's submissions / discussion** — see what someone posts/comments, score traction, filter to recent. haxor-news exposes `user`; HN's own profile page is poor for analysis.
5. **Spot polarizing or trending stories** — high-velocity climbs, controversial discussions (high comment-to-point ratio), reposts of an old URL. None of the existing tools do this; the data exists but only as snapshots.

## Table Stakes
- `top`, `new`, `best`, `show`, `ask`, `job` story lists with `--limit` and `--json`
- Single-item fetch (story, comment, user) by ID
- Comment thread traversal
- Algolia full-text search with `--query`, `--tags`, `--date-range`, `--sort`
- User profile lookup
- HTML rendering of linked story content (haxor-news, circumflex)
- Pipe-friendly output, exit codes, regex filtering

## Data Layer
- **Primary entities:** `items` (one table covering stories, comments, jobs, polls, pollopts — the HN data model is unified), `users` (id, karma, about, created, submitted), `lists` (top/new/best/show/ask/job — IDs only, snapshotted with timestamp).
- **Sync cursor:** Firebase `/v0/maxitem` is a monotonic counter; combined with `/v0/updates` (which returns recently-changed item IDs and updated profiles) this gives a clean incremental sync. List endpoints are snapshots — store with `(list_kind, captured_at, position, item_id)`.
- **FTS/search:** SQLite FTS5 over `items.title`, `items.text`, `users.about`. The novel angle: every story or comment ever fetched stays searchable offline forever; users see Algolia's freshness for live search, plus a personal corpus that grows with use.

## Codebase Intelligence
- Source: official HN API documentation at `github.com/HackerNews/API` and Algolia docs at `hn.algolia.com/api`.
- **Auth:** none on either API. No headers, no env var.
- **Data model:** unified `item` type with `type ∈ {story, comment, job, poll, pollopt}`. Stories link to a URL or carry text; comments have a `parent` chain rooted at a story; jobs are stories whose author is `whoishiring` (monthly threads).
- **Rate limiting:** none documented; both APIs are CDN-fronted and accept hundreds of calls/second in practice. The CLI should still concurrent-fetch the front page (a typical `topstories.json` returns 500 IDs and each must be fetched individually from `/v0/item/<id>.json`).
- **Architecture insight:** Firebase is the source of truth but is fan-out — to see "the front page" you fetch one list (500 IDs) and then `N` parallel item fetches. Algolia is fan-in — one search returns the full hit metadata in one round-trip, but it's eventually-consistent vs Firebase by ~minutes.

## User Vision
None provided — user requested a refresh reprint with no specific vision bias. Reconcile prior features against current personas and the v3.9.0 machine.

## Product Thesis
- **Name:** `hackernews-pp-cli` (slug `hackernews`, display name "Hacker News")
- **Why it should exist:** Every existing HN CLI is a TUI reader optimized for human scrolling. None of them: (a) build a local SQLite corpus that compounds across sessions, (b) expose `--json --select` for agent consumption, (c) join Firebase's canonical item bodies with Algolia's search-time scoring, or (d) compute analytical views (controversy, repost lookup, story velocity, hiring stats) that require a local store. This CLI is the read-only HN power user's data layer — offline searchable, agent-native, scriptable, and built to compound.

## Build Priorities
1. **Foundation (Phase 3 P0):** SQLite store schema for items+users+lists, sync command (Firebase incremental + Algolia bulk seed), FTS5 search, SQL passthrough.
2. **Absorbed (Phase 3 P1):** Every Firebase + Algolia endpoint as typed commands. Match haxor-news's `top/new/best/show/ask/job/user` browse, regex filtering on hiring/freelance threads.
3. **Transcend (Phase 3 P2):** Compound analytical commands — front-page diff (`since`), topic pulse, hiring stats, controversy, repost finder, story velocity, structured thread digest, local FTS — all backed by the local store.
