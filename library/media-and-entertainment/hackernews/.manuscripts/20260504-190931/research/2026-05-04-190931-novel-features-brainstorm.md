# Novel Features Brainstorm — hackernews (reprint)

> Audit trail from Phase 1.5 Step 1.5c.5 subagent (2026-05-04).
> Run ID: 20260504-190931
> Prior research: `~/printing-press/library/hackernews/research.json` (run 20260427-120911, machine v2.3.9)

## Customer model

**Persona A — Riya, the topic-tracking founder/researcher.** A solo technical founder who tracks emerging-tech discourse to inform product bets. Daily HN reader; weekly deep dives on specific topics (e.g., "what is HN saying about local LLMs this month").

- **Today (without this CLI):** She has hn.algolia.com open in two tabs — one sorted by points, one by date. She manually flips between them, copies promising titles into a Notion doc, and re-runs the same query a week later to compare. She can answer "what's the top story about X right now" but not "is mentions-of-X accelerating week over week" without copy-pasting result counts into a spreadsheet.
- **Weekly ritual:** Monday and Friday she reviews 3-5 saved topics (e.g. "vector db", "agent infra", "ZK"), reads 5-10 threads each, and flags 1-2 reposts to her newsletter draft.
- **Frustration:** Algolia returns hits but no aggregation. The only way to see whether a topic is heating up or cooling down is manual counting across paginated result pages. The pattern is per-day frequency × score × comment volume — exactly the math the API will not do for her.

**Persona B — Marco, the tech recruiter / hiring-thread miner.** A recruiter who tracks the "Who is Hiring" / "Freelancer? Seeking Freelancer?" monthly threads to source candidates and clients, plus tracks which companies post repeatedly.

- **Today (without this CLI):** He opens HNHIRING.com or scrolls the latest /item/<thread> page in his browser. Ctrl-F for "Go", "remote", "senior". He cannot easily compare this month's thread to last month's to spot new postings vs. reposts, or count how many remote roles each language drew over a quarter.
- **Weekly ritual:** First weekday of each month he scans the new "Who is Hiring" thread end-to-end, copies fresh leads into a CRM. Mid-month he Ctrl-Fs for new replies under the same regex.
- **Frustration:** Mining one month is doable with browser search; comparing across 3-6 months is essentially impossible without scraping. He cannot answer "which companies posted in 3 of the last 6 months" or "remote-role share by language over the quarter."

**Persona C — Ana, the agent-driven HN signal subscriber.** Builds an agent that wakes up daily and asks "what shifted on HN since yesterday — what climbed, what fell, what is suddenly polarizing." Consumes the CLI almost entirely through `--json` and MCP.

- **Today (without this CLI):** Her agent fetches `/topstories` every run and re-fetches every item, then naively diffs against a local JSON dump it wrote yesterday. Every poll is a full-front-page fan-out (500 items). It cannot tell why a story moved (more comments? higher score?) without manual bookkeeping.
- **Weekly ritual:** Daily morning poll and a Friday "what was controversial this week" digest fed into her reading queue.
- **Frustration:** No way to ask the API "what changed." Every diff and every controversy ranking has to be reconstructed by the agent from raw item bodies it has to fetch itself, every time. Each poll is wasteful and the agent cannot ask "rank trajectory" because there is no rank-over-time endpoint.

**Persona D — Sami, the about-to-submit Show HN poster.** A maker who has just finished a side project and wants to post it to HN without dupes, picks a posting time backed by data, and reviews how their last few submissions traveled.

- **Today (without this CLI):** Pastes their URL into Algolia search to check for prior submissions, eyeballs scores. Guesses peak HN time from blog posts. Looks at their own profile page in the browser — it shows submitted IDs but no traction summary.
- **Weekly ritual:** Episodic — every few weeks, but high-stakes when it happens.
- **Frustration:** No single answer to "has this URL been on HN before, and how did it do?" and no single answer to "across my last 20 posts, what's my median score and which ones broke 100." The data is all there, scattered.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| 1 | Front-page diff | `since` | Show stories that appeared, disappeared, or moved on the front page since last sync | C | (d) prior-keep | Keep — local snapshot diff, pure mechanical, no LLM, buildable from `lists` table |
| 2 | Topic pulse | `pulse <q>` | Per-day mentions, avg score, comment volume for a topic over N days | A | (d) prior-keep | Keep — Algolia `search_by_date` + numericFilters bucketed by day; aggregation absent from API |
| 3 | Submission tracker | `my <user>` / `users stats <user>` | User's submissions with score buckets, traction rate, best-hour hints | D | (d) prior-reframe | Reframe — narrow to score buckets + hour-of-day from synced submissions; rename to `users stats` for namespace consistency |
| 4 | Hiring stats | `hiring stats` | Aggregate Who's Hiring across N months: top languages, remote ratio, top companies | B | (d) prior-keep | Keep — fetches monthly `whoishiring` items + regex-tokenizes; mechanical, no LLM |
| 5 | Controversial | `controversial` | Stories ranked by comment-to-point ratio over a window | A, C | (d) prior-keep | Keep — pure local SQL on synced stories |
| 6 | Repost finder | `repost <url>` | Has this URL been posted before? Lists prior submissions, scores, dates | D | (d) prior-keep | Keep — Algolia URL search; trivial composition with rank by score |
| 7 | Story velocity | `velocity <id>` | Rank trajectory over time from sync snapshots | C | (d) prior-keep | Keep — requires multi-snapshot history; no live API equivalent; pure SQL on snapshots |
| 8 | Thread tldr (structured) | `items thread <id> --digest` | Top authors by reply count, root vs reply ratio, comment heat | A, C | (d) prior-reframe | Reframe — fold into `items thread`; mechanical structured digest, no AI |
| 9 | Local FTS search | `search local "<q>"` | FTS5 over every item ever synced to the local corpus | A, C | (d) prior-keep | Keep — pure local SQLite FTS5; offline; compounds with use |
| 10 | Sync foundation | `sync` | Pull lists + items + updates into local SQLite | all | (d) prior-keep | Keep — store-population command; carve-out per AGENTS.md |
| 11 | Hiring company tracker | `hiring companies --months N` | Companies that posted in M of last N monthly hiring threads, with first-seen / last-seen | B | (b) service-specific | Keep — `whoishiring` thread structure is a defining HN content pattern; cross-month join is impossible without local store |
| 12 | Front-page residency | `stories residency --window 7d` | For each story that hit the front page in the window, total minutes on page and peak rank | C | (c) cross-entity local | Cut — strict subset of `velocity` queried over a window; would duplicate snapshot SQL |
| 13 | User overlap | `users overlap <a> <b>` | Threads where two users both commented + per-user reply counts to each other | A | (c) cross-entity local | Cut — niche, no research backing, would not run weekly |
| 14 | Story heat alert | `stories heat --watch` | Polling loop that prints rising stories | C | (a) persona-driven C | Cut — daemon-shaped scope creep; agents already cron `since`/`velocity` |
| 15 | Posting-window finder | `users best-hour <user>` | For a user, distribution of submission scores by submitted-hour | D | (a) persona-driven D | Reframe — fold into candidate #3 (`users stats`) |
| 16 | Reading list export | `stories export --read myreadinglist.json` | Export selected story IDs + URLs as JSON or markdown | A | (a) persona-driven A | Cut — thin wrapper over `stories top --json --select` |
| 17 | URL co-discussion | `repost <url> --threads` | Show all prior threads for a URL with author and score, plus the top comment from each | D | (b) service-specific | Cut — output flair on candidate #6; merge into `repost` |
| 18 | Onion stories | `stories onion` | Pull theonion.com hits via Algolia source-restricted search | A | (b) service-specific | Cut — feature in haxor-news; absorb-row, not transcendence |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence | Source |
|---|---------|---------|-------|--------------|----------|--------|
| 1 | Front-page diff | `since` | 9/10 | Reads two `lists` snapshots from local SQLite (`(list_kind, captured_at, position, item_id)`) and emits added/removed/moved rows | Brief Workflow #5; prior `novel_features_built` (built once, kept); Persona C's frustration is exactly this gap | prior (kept) |
| 2 | Topic pulse | `pulse <q>` | 9/10 | Calls Algolia `/search_by_date` with `numericFilters=created_at_i>...` once per day-bucket and aggregates hits/score/comments locally | Brief Workflow #2 explicitly calls out "popular hits and recent buzz"; Persona A's spreadsheet substitute; Algolia returns hits, not aggregations | prior (kept) |
| 3 | Hiring thread aggregator | `hiring stats --months N` | 9/10 | Fetches `/user/whoishiring` submitted IDs, takes the last N "Who is Hiring" root items, fetches their comment children via Firebase, and tokenizes posts with named regex passes (languages, remote, location, salary signal) | Brief Workflow #3 ("confirmed killer use case"); HNHIRING.com proves demand; haxor-news's `hiring` regex command proves single-month value but cannot aggregate across months; Persona B's quarter-comparison frustration | prior (kept) |
| 4 | Hiring company tracker | `hiring companies --months N` | 8/10 | Joins tokenized monthly hiring threads on a normalized company token; emits first-seen, last-seen, months-posted count | Brief Workflow #3 + Persona B's "which companies posted in 3 of last 6"; HNHIRING as the demand proof; cross-month join is impossible from any single endpoint | new |
| 5 | Controversial | `controversial --window 7d` | 8/10 | `SELECT id, title, score, descendants, descendants*1.0/score AS ratio FROM items WHERE type='story' AND created_at > now()-:window ORDER BY ratio DESC` | Brief Workflow #5 explicitly names "controversial" as an unmet need; no competitor surfaces this; prior `novel_features_built` (kept) | prior (kept) |
| 6 | Repost finder | `repost <url>` | 8/10 | Algolia search with `restrictSearchableAttributes=url` (or normalized URL match), then sort by points | Brief Workflow #5 (repost detection); Persona D's pre-submit ritual; prior `novel_features_built` (kept) | prior (kept) |
| 7 | Story velocity | `velocity <id>` | 8/10 | `SELECT captured_at, position FROM lists WHERE item_id=:id ORDER BY captured_at` against multi-snapshot history; emits rank-over-time series | Brief Workflow #5 ("high-velocity climbs"); no live API surface for rank-over-time; Persona C's "is this gaining traction" question; prior `novel_features_built` (kept) | prior (kept) |
| 8 | User stats | `users stats <user>` | 7/10 | Joins `users.submitted` IDs with synced `items` to compute median/p90 score, traction-rate buckets, hour-of-day score distribution | Brief Workflow #4 ("HN's own profile page is poor for analysis"); Persona D's posting-window question; reframes prior `my` and absorbs the posting-window candidate | prior (reframed from `my`) |
| 9 | Local FTS search | `search local "<q>"` | 7/10 | SQLite FTS5 virtual table over `items.title || ' ' || items.text` and `users.about`; reads from corpus that grows with every sync | Brief Data Layer ("FTS5 over `items.title`, `items.text`, `users.about`"); Algolia drops/reindexes history; Persona A's "replay last month's investigation" need; prior `novel_features_built` (kept) | prior (reframed from `local-search`) |
| 10 | Sync foundation | `sync` | 8/10 | Firebase incremental via `/v0/maxitem` + `/v0/updates`, plus list snapshots; populates the store every other novel command reads from | Brief Build Priorities #1; foundation for #1, #5, #7, #9; prior `novel_features_built` (kept) | prior (kept) |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Thread tldr (structured) | Reframed and folded — value moves to `items thread <id> --digest` flag rather than a separate command; deterministic stats are real but "tldr" name is LLM-coded | `users stats` |
| User overlap (`users overlap a b`) | No persona asked for it; fails Pass 3 weekly-use; sibling kill | `users stats` |
| Story heat watch (`stories heat --watch`) | `--watch` daemon is scope creep per kill rubric; agents already cron `since`/`velocity` | `since`, `velocity` |
| Posting-window finder (`users best-hour`) | Strict subset of `users stats`; merging avoids two commands for one ritual | `users stats` |
| Reading list export | Thin wrapper over `stories top --json --select`; fails wrapper-vs-leverage check | `search local` |
| URL co-discussion | Pure output flair on `repost`; not a separate feature | `repost` |
| Onion stories | Already exists in haxor-news; absorb-row, not transcendence | `stories show`/`stories ask` (absorbed table) |
| Front-page residency (`stories residency`) | Strict subset of `velocity` queried over a window; would duplicate snapshot SQL | `velocity` |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| `since` (Front-page diff) | Keep | Persona C's primary daily call; pure local SQL on snapshots; score 9/10; reuse command verbatim |
| `pulse` (Topic pulse) | Keep | Persona A's weekly ritual; Algolia aggregation is the canonical gap the API will not fill; score 9/10; reuse command verbatim |
| `my` (Submission tracker) | Reframe | Same intent (Persona D), but command renamed to `users stats <user>` for namespace consistency with `users get` / `users submitted` and to absorb the posting-window-finder candidate |
| `hiring-stats` (Hiring stats) | Reframe | Promote into a `hiring` subcommand group (`hiring stats`, `hiring companies`) so the aggregator and the company tracker share infrastructure |
| `controversial` (Controversial) | Keep | Persona A + C use it weekly; pure local SQL; no competitor surfaces it; score 8/10 |
| `repost` (Repost finder) | Keep | Persona D's pre-submit ritual; minimal Algolia composition; score 8/10 |
| `velocity` (Story velocity) | Keep | Persona C's trajectory question; impossible without local snapshots; score 8/10 |
| `tldr` (Thread tldr structured) | **Drop** | Folded into `items thread <id> --digest` flag; deterministic stats survive but the standalone command does not justify a row, and the `tldr` name is misleading for what is purely mechanical aggregation |
| `local-search` (Local FTS search) | Reframe | Same intent, renamed to `search local "<q>"` so the FTS subsurface sits beside live Algolia `search` instead of as a top-level peer |
| `sync` (Sync foundation) | Keep | Foundation for half of the survivors; carve-out per AGENTS.md; reuse command verbatim |
