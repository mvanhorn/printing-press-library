# Naver Blog — Novel Features Brainstorm (Step 1.5c.5 audit trail)

**Subagent run:** 2026-05-19T16:35-ish
**Inputs:** Brief, absorb-scoring rubric, no prior research (first print), Codebase Intelligence enumerated in brief

---

## Customer model

**Persona 1: Eunji, AKLABS Korea digital marketing operator (the Chilly campaign owner)**

- **Today (without this CLI):** She runs `chilly-naver-weekly`, a Hermes cron job that fires 6 keyword/hashtag queries (e.g., 칠리 청결제, #여성청결제) against Naver Blog through `web_search` + `web_extract`. It returns URLs; she then fires a Playwright helper that opens each post inside a Chromium instance to scrape `span.u_likeit_text._count.num` for likes, plus a regex for `commentCount`. The output lands in a Google Sheet. Once a month she turns that sheet into a Korean slide deck. The Playwright dependency is a maintenance ulcer — Chrome version drift breaks the job roughly quarterly, and `web_search` rotates its selectors faster than that.
- **Weekly ritual:** Monday morning the cron writes ~20-40 fresh blog URLs and engagement counts to the Chilly sheet. She skims for sponsored-feeling posts (체험단) versus genuine reviews, flags top performers, and pings the agency about underperformers. Tuesday she checks a handful of repeat-influencer feeds (manual URL list) to see whether they posted again.
- **Frustration:** The likes count. Static HTML always returns 0 because the count is rendered by a separate XHR after the iframe loads, and the iframe selector breaks whenever Naver ships a frontend change. She has rewritten the Playwright helper three times in twelve months. Comments are stable (regex on static HTML), but likes are the single field most likely to derail a Monday morning.

**Persona 2: Jin-ho, Korean influencer-marketing analyst**

- **Today (without this CLI):** Tracks ~30 Naver Blog influencers across women's wellness, beauty, and lifestyle. He has a Notion list of `blog_id` values and copy-pastes them into a browser one at a time, scrolling each blog's mobile feed page to eyeball post cadence and engagement floor. For sponsored disclosure audits he opens individual posts and CTRL-F for the word "협찬" or "체험단" — there is no scriptable way to do this today.
- **Weekly ritual:** Friday afternoon "influencer health check" — for each blog, what did they post this week, did engagement hold, did they disclose sponsorship.
- **Frustration:** No way to bulk-pull a blog's recent posts with engagement *and* see whether each post is sponsored. The Naver Open API requires developer credentials and returns the wrong fields. Every OSS scraper he tried either requires Playwright or drops the likes column.

**Persona 3: Su-ah, the user wearing her ad-hoc-scripting hat**

- **Today (without this CLI):** When she gets a list of post URLs from Slack ("rivals are running these — pull engagement"), she manually pastes each URL into the Playwright helper one at a time. For two dozen URLs that's an afternoon of waiting on Chromium spinning up and burning RAM.
- **Weekly ritual:** Ad-hoc, 1-3 times a week — receive URL bundle from Slack, return engagement table to whoever asked.
- **Frustration:** No batch primitive that takes a list of URLs and returns engagement in deterministic JSON.

---

## Candidates (pre-cut)

(See sub-agent's full table in the run output. 16 candidates generated, sources labeled (a)-(f). Inline kill/keep verdicts applied per the rubric.)

| # | Candidate | Command | Persona | Source | Verdict |
|---|---|---|---|---|---|
| C1 | Engagement diff over time | `post diff <url>` | Eunji, Jin-ho | (a),(c) | Pass |
| C2 | Sponsored-post detector | `--flag-sponsored` + col | Jin-ho, Eunji | (a),(b) | Pass |
| C3 | Influencer health snapshot | `blog health --ids-file` | Jin-ho | (a),(c) | Pass |
| C4 | Engagement-floor quantiles | `blog stats <id>` | Jin-ho | (a),(c) | Pass → merged with C3 |
| C5 | Cron-mode bundle | `cron <queries-file>` | Eunji | (a),(e) | Pass |
| C6 | Hashtag co-occurrence | `hashtag neighbors` | Eunji | (b),(c) | Pass |
| C7 | Backfill-by-URL-list | `post-batch --from-file` | Su-ah | (a),(e) | Pass (reframed as novel slice over absorb #15) |
| C8 | Neighbor-blog discovery (이웃) | `blog neighbors` | Jin-ho | (b) | **Kill** — auth-gated, conflicts with no-auth lock |
| C9 | Category browse | `blog categories` + `feed --category` | Jin-ho | (b) | Pass |
| C10 | Offline Korean FTS | `search local <query>` | Eunji, Jin-ho | (c) | Pass |
| C11 | Top-by-engagement | `search top --top N` | Eunji | (a),(c) | **Kill** — thin rename; `jq` does it |
| C12 | Likes drift watchlist | `watchlist refresh` | Eunji | (a),(c) | **Kill** — subsumed by C1 |
| C13 | Word-count / read-time | `post get --include-stats` | Jin-ho | (b) | **Kill** — Korean segmentation = LLM territory |
| C14 | HAR-based selector probe | `dev har-probe` | Eunji-as-maintainer | (e) | **Kill** — scope creep, not weekly |
| C15 | Engagement-source audit | `db audit --engagement-divergence` | ops | (c),(e) | **Kill** — diagnostic, not weekly |
| C16 | Cadence histogram | `blog cadence <id>` | Jin-ho | (a),(c) | **Kill** — below 5/10 floor; subsumed by N3's `days-since-last-post` |

---

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| N1 | Engagement diff over time | `post diff <url> [--since <ts>]` | 9/10 | hand-code | Queries `engagement_history` for `(blog_id, log_no)`, returns delta vs latest snapshot + writes new sample; no Naver call if `--since` resolves to cache, else re-fetches via reaction API | Brief calls out `engagement_history` as novel; Eunji's Playwright currently has no history; competitors are point-in-time only |
| N2 | Sponsored-post detector | `--flag-sponsored` flag on `post`/`feed`/`search`/`post-batch` outputs | 8/10 | spec-emits (flag) + hand-code (matcher) | Regex over parsed `body_text` for KFTC disclosure phrases (협찬, 체험단, 광고 포함, 유료광고 포함, "본 포스팅은 ... 받아 작성됨") — returns `{sponsored: bool, markers_matched: [...]}` | Jin-ho's weekly disclosure audit; KFTC mandates a finite phrase set; zero new I/O (uses absorbed body_text) |
| N3 | Influencer health snapshot | `blog health --ids-file <file> --window 7d` | 8/10 | hand-code | For each blog_id, refresh feed cache, compute posts-in-window, median likes, median comments, sponsored ratio (via N2), days-since-last-post | Jin-ho's Friday ritual; no OSS scraper composes feed walk + engagement + sponsored flag |
| N4 | Cron-mode bundle | `cron <queries-file>` | 8/10 | hand-code | Reads JSONL/YAML query list, runs `search`/`hashtag`, enriches with engagement (reaction API), dedupes by canonical URL, writes posts+searches+engagement to SQLite, streams TSV on stdout for sheet ingestion | Brief User Vision names "replace Chilly Hermes cron's web_search + web_extract + Playwright" as headline integration |
| N5 | Hashtag co-occurrence | `hashtag neighbors <#tag> --top 20` | 6/10 | hand-code | Joins `posts.tags_json` for matched posts, counts tag co-occurrence, ranks desc | Korean blog culture clusters hashtags densely; pure local SQL after `hashtag` |
| N6 | Offline Korean FTS | `search local <query>` | 7/10 | hand-code | Queries `posts_fts` for Korean substrings, joins back for engagement, ranks by FTS + likes | Brief specifies FTS5 + Korean tokenization; Eunji/Jin-ho rerun same historical queries weekly |
| N7 | File-driven URL backfill | `post-batch --from-file <path>` | 7/10 | spec-emits (flag) + hand-code (file reader) | Reads URLs from CSV/JSON/newline-delimited, canonicalizes, runs `post get` concurrent with AdaptiveLimiter, preserves input order, emits per-URL error JSON | Su-ah's Slack-bundle workflow; novel slice over absorbed `post-batch` (#15) |
| N8 | Category browse | `blog categories <blog_id>` + `feed --category <n>` | 5/10 | spec-emits (--category flag) + hand-code (summary command) | Reads cached categories_json; --category applies SQL filter or live `feed` URL param | Naver-specific numbered category system; absorbed feed already returns category info |

**Hand-code commitment count:** 5 features are fully hand-code (N1, N3, N4, N5, N6). N2/N7/N8 have a `spec-emits` portion plus a small `hand-code` portion. Total novel-feature LoC estimate: ~600-1000 lines across `internal/cli/` + a few `internal/lib/` helpers (sponsored matcher, FTS5 query, URL canonicalizer port).

### Killed candidates

| Feature | Kill reason | Closest survivor |
|---|---|---|
| C8 Neighbor-blog (이웃) | Auth-gated; conflicts with no-auth lock | N3 health snapshot |
| C11 Top-by-engagement | Thin rename; `jq` does it from `search` output | N4 cron bundle |
| C12 Watchlist | Subsumed by N1 + shell loop; CLI shouldn't own list state | N1 post diff |
| C13 Word-count/read-time | Korean word segmentation = LLM territory; verifiability fails | N2 sponsored detector (real content signal) |
| C14 HAR selector probe | Scope creep, not weekly | (none — README HAR section covers it) |
| C15 Engagement-source audit | Diagnostic, not weekly | (none — README SQL query covers it) |
| C16 Cadence histogram | <5/10 floor; subsumed by N3's days-since-last-post | N3 health snapshot |
