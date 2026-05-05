# Hacker News CLI — Absorb Manifest

> Reprint of `~/printing-press/library/hackernews/` (run 20260427-120911, machine v2.3.9) onto current machine v3.9.0.

## Tools surveyed

- **haxor-news** (donnemartin/haxor-news, Python, ~3.9k stars) — top, best, show, ask, jobs, new, onion, view, view -c, hiring [regex], freelance [regex], user, comments_unseen, comments_recent, comments_query, --browser
- **circumflex** (bensadeh/circumflex, Go) — TUI HN reader: front-page browse, comment thread navigation, syntax highlighting, Vim-style navigation
- **hn-cli** (rafaelrinaldi/hn-cli, JS) — front-page list, open in browser
- **hacker-feeds-cli** (Mayandev/hacker-feeds-cli, JS) — multi-source feed aggregator
- **mtharrison/hackernews, saivarshith2000/hacker_news_cli, jbranchaud/hn-cli, gamontal/hackernews-cli, pyhn** — minor readers
- **AutoCLI** — multi-site fetcher
- **cyanheads/hacker-news-mcp-server** (MCP) — Firebase feeds, threaded discussions, user profiles, Algolia full-text search
- **GeorgeNance/hackernews-mcp** (MCP) — top stories, story detail with markdown extraction, popular comments with filtering, search by keyword + time range
- **node-hn-api** (npm v4.0.1, TypeScript Firebase wrapper, recently maintained)
- **hacker-news-api** (npm Algolia wrapper)
- **firebase-hackernews, hacker-news-firebase, hackernews-api** (npm, older Firebase wrappers)
- **HNHIRING.com** — web service; proves Who-is-Hiring mining is a real demand

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Top stories list | haxor-news `top`, circumflex front | `stories top --limit N` | offline reads, --json/--select, agent-native |
| 2 | New stories list | haxor-news `new` | `stories new --limit N` | same |
| 3 | Best stories list | haxor-news `best` | `stories best --limit N` | same |
| 4 | Show HN stories | haxor-news `show` | `stories show --limit N` | same |
| 5 | Ask HN stories | haxor-news `ask` | `stories ask --limit N` | same |
| 6 | Job stories | haxor-news `jobs` | `stories job --limit N` | same |
| 7 | Single story fetch by ID | every CLI / Firebase /item | `items get <id>` | structured JSON, --select |
| 8 | Comment thread fetch | haxor-news `view -c`, circumflex | `items thread <id>` | preserved tree structure, --json |
| 9 | User profile | haxor-news `user` | `users get <id>` | structured JSON |
| 10 | User submissions | Algolia /users, haxor-news | `users submitted <id>` | filter by type / date range |
| 11 | Algolia full-text search | cyanheads MCP | `search "<q>"` | --tags, --date-range, --sort, --json |
| 12 | Search by date (recency) | Algolia /search_by_date | `search "<q>" --recent` | flag wraps the alt endpoint |
| 13 | Filter Who-is-Hiring by regex | haxor-news `hiring` | `hiring month <yyyy-mm> --query <regex>` | --json structured rows |
| 14 | Filter Freelance by regex | haxor-news `freelance` | `freelance month <yyyy-mm> --query <regex>` | same |
| 15 | Open story link in browser | haxor-news `--browser`, hn-cli | `items open <id>` | --launch flag (default print URL); short-circuits in verify |
| 16 | Comment regex filter | haxor-news `-cq` | `items thread <id> --filter <regex>` | runs on synced comments |
| 17 | Comments since timestamp | haxor-news `-cr` | `items thread <id> --since <duration>` | duration arg (e.g. `2h`) |
| 18 | Max-item ID (sync watermark) | Firebase /maxitem | exposed via `doctor` | foundation for incremental sync |
| 19 | Recent updates feed | Firebase /updates | `sync --updates` | fetch only changed items |
| 20 | Karma/about/created | every CLI | covered by row 9 | — |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence | Source |
|---|---------|---------|-------|--------------|----------|--------|
| 1 | Front-page diff | `since` | 9/10 | Reads two `lists` snapshots from local SQLite (`(list_kind, captured_at, position, item_id)`) and emits added/removed/moved rows | Brief Workflow #5; prior `novel_features_built` (kept); Persona C's frustration | prior (kept) |
| 2 | Topic pulse | `pulse <q>` | 9/10 | Calls Algolia `/search_by_date` with `numericFilters=created_at_i>...` once per day-bucket and aggregates hits/score/comments locally | Brief Workflow #2; Persona A's spreadsheet substitute; Algolia returns hits, not aggregations | prior (kept) |
| 3 | Hiring thread aggregator | `hiring stats --months N` | 9/10 | Fetches `/user/whoishiring` submitted IDs, takes the last N "Who is Hiring" root items, fetches comment children, tokenizes with named regex passes (languages, remote, location, salary signal) | Brief Workflow #3 ("confirmed killer use case"); HNHIRING.com proves demand; Persona B's quarter-comparison frustration | prior (reframed from `hiring-stats`) |
| 4 | Hiring company tracker | `hiring companies --months N` | 8/10 | Joins tokenized monthly hiring threads on a normalized company token; emits first-seen, last-seen, months-posted count | Brief Workflow #3 + Persona B's "which companies posted in 3 of last 6"; cross-month join is impossible from any single endpoint | new |
| 5 | Controversial | `controversial --window 7d` | 8/10 | `SELECT id, title, score, descendants, descendants*1.0/score AS ratio FROM items WHERE type='story' AND created_at > now()-:window ORDER BY ratio DESC` | Brief Workflow #5; no competitor surfaces this; prior `novel_features_built` (kept) | prior (kept) |
| 6 | Repost finder | `repost <url>` | 8/10 | Algolia search with `restrictSearchableAttributes=url`, then sort by points | Brief Workflow #5; Persona D's pre-submit ritual; prior `novel_features_built` (kept) | prior (kept) |
| 7 | Story velocity | `velocity <id>` | 8/10 | `SELECT captured_at, position FROM lists WHERE item_id=:id ORDER BY captured_at`; emits rank-over-time series | Brief Workflow #5 ("high-velocity climbs"); no live API surface for rank-over-time; Persona C | prior (kept) |
| 8 | User stats | `users stats <user>` | 7/10 | Joins `users.submitted` IDs with synced `items` to compute median/p90 score, traction buckets, hour-of-day distribution | Brief Workflow #4; Persona D's posting-window question; absorbs sibling `users best-hour` candidate | prior (reframed from `my`) |
| 9 | Local FTS search | `search local "<q>"` | 7/10 | SQLite FTS5 virtual table over `items.title || ' ' || items.text` and `users.about`; corpus grows with every sync | Brief Data Layer; Algolia drops/reindexes history; Persona A's "replay last month's investigation" need | prior (reframed from `local-search`) |
| 10 | Sync foundation | `sync` | 8/10 | Firebase incremental via `/v0/maxitem` + `/v0/updates`, plus list snapshots | Brief Build Priorities #1; foundation for #1, #5, #7, #9 | prior (kept) |

## Dropped prior features

| Prior feature | Drop reason |
|---------------|-------------|
| `tldr` (Thread tldr structured) | Folded into `items thread <id> --digest` flag — deterministic stats survive as a flag, but the standalone command did not justify its own row, and the `tldr` name reads as LLM-coded for what is purely mechanical aggregation |

## Scope summary

- **Absorbed:** 20 features (matches and beats every existing CLI/MCP listed above with --json/--select/agent-native output and offline reads where applicable)
- **Transcendence:** 10 features (7 prior keeps, 2 prior reframes carrying old intent under new namespace, 1 new: `hiring companies`)
- **Dropped from prior:** 1 (`tldr` → folded into `items thread --digest` flag)
