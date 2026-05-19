# Naver Blog CLI — Research Brief

**Stamp:** 2026-05-19-162524
**Argument:** `"Naver Blog"`
**Gate 1 reference:** [`~/.claude/plans/pp-naver-blog-gate1.md`](/Users/johnchang/.claude/plans/pp-naver-blog-gate1.md)
**Auth posture:** Sniffed public endpoints only. No `NAVER_CLIENT_ID`/`SECRET`. No login. (Locked Phase 0.5.)

## API Identity
- **Domain:** South Korea's dominant blog platform; `blog.naver.com` and mobile mirror `m.blog.naver.com`.
- **Users:** Korean creators, brand marketing teams (the user's Chilly campaign uses Naver Blog influencers heavily), and any researcher/analyst tracking Korean consumer or cultural content.
- **Data profile:** Posts have stable identifiers `(blog_id, log_no)`. Engagement (likes/공감, comments) is dynamic and JS-rendered. Search/hashtag results rotate selectors every few months. Korean text is UTF-8 throughout.

## Reachability Risk
**Low** for the primary endpoints, **medium** for search/hashtag.
- `m.blog.naver.com/api/blogs/{blog_id}/post-list` and `m.blog.naver.com/{blog_id}/{log_no}` work reliably from working OSS scrapers (hames, krsite-dl, snudm, tiger-beom, user's `fetch_sheet.py`) — provided a real-browser `User-Agent` and `Referer: https://m.blog.naver.com/{blog_id}` are sent.
- `search.naver.com/search.naver?where=view` is JS-rendered in 2025/26 and DOM selectors (`sds-comps-*`) rotate every few months. Mobile post-list API is stable; integrated SERP page is not.
- Non-Korean IPs occasionally serve degraded content. User runs from KR.

## Top Workflows
1. **Find every recent post matching a keyword/hashtag, with engagement.** The Chilly cron's current job — 6 keyword/hashtag queries → list of URLs → engagement enrichment.
2. **Backfill engagement for a known list of URLs.** Operator already has post URLs (from a spreadsheet, prior crawl, or Slack), wants likes + comments + upload date now.
3. **Crawl one blog's full feed.** "Give me the last N posts from `blogId=X`" — useful for influencer audits and longitudinal tracking.
4. **Resolve a raw post URL to canonical `(blog_id, log_no)` + metadata in one call.** Useful as a primitive for ad-hoc scripting.
5. **Cache locally to compose with SQL/jq.** Local SQLite store of posts/blogs/tags lets users run "all posts about X in April with >50 likes" without re-fetching.

## Table Stakes (absorbed from competitors)
- **Per-blog feed walk** via `m.blog.naver.com/api/blogs/{blog_id}/post-list` (hames, krsite-dl, hyunsikhwang)
- **PC fallback** via `PostTitleListAsync.naver` with the invalid-escape sanitizer (hames)
- **Mobile post detail** — body via `.se-main-container` inline parse, no iframe needed (hames, krsite-dl, user's fetch_sheet.py, tiger-beom)
- **5-strategy tag extraction** from post HTML: `gsTagName` JS var → `tagNames` JSON → `og:tag` meta → DOM `.post_tag` → hashtag anchors (hames)
- **Comment count from static HTML** via `id="commentCount"` or `id="floating_bottom_commentCount"` regex (user's fetch_sheet.py)
- **Likes count via JS-rendered iframe** (user's fetch_sheet.py: `span.u_likeit_text._count.num` inside `#mainFrame`)
- **UA rotation with backoff** on 403/empty responses (hames: 3 UAs, 1.5s/3s/4.5s backoff)
- **Date format normalizers** for 10-digit epoch, 13-digit ms, 14-digit YYYYMMDDHHMMSS, and Korean relative `N시간 전`
- **URL canonicalization** — collapse `blog.naver.com/{id}/{n}` and `m.blog.naver.com/{id}/{n}` and `PostView.naver?blogId=X&logNo=Y` into a single key (user's `lib/naver_url.py`)
- **Korean query URL-encoding** with `urllib.parse.quote` equivalent (all scrapers)

## Data Layer (SQLite-backed local store)
- **Primary entities:**
  - `posts` — `(blog_id, log_no)` PK; columns: `url`, `title`, `body_text`, `body_html_compressed`, `published_at`, `category_no`, `category_name`, `nickname`, `likes`, `comments`, `tags_json`, `images_json`, `last_seen_at`, `engagement_source` (`static` | `browser` | `xhr_if_found`)
  - `blogs` — `blog_id` PK; columns: `nickname`, `latest_post_at`, `post_count_observed`, `categories_json`, `last_seen_at`
  - `searches` — `(query, executed_at)`; columns: `query`, `query_kind` (`keyword` | `hashtag`), `rank_url_json`, `month_param`, `executed_at`, `top_n_urls`
  - `engagement_history` — `(blog_id, log_no, captured_at)`; columns: `likes`, `comments`, `source` — lets the user track engagement over time, which is novel against every reviewed OSS scraper
- **Sync cursor:** Per-blog `latest_log_no` to make `feed --since` cheap.
- **FTS5/search:** `posts_fts` on `title`, `body_text`, `tags`, `nickname`. Korean tokenization: SQLite's default `unicode61` tokenizer plus a Korean-aware `trigram` extension where available; fall back to `unicode61` + `LIKE %query%` for substring fidelity.

## Codebase Intelligence
- **Source files studied (highest-signal):**
  - `baek-labs/hames` `arsenal/naver_blog_scraper.js` — Node.js; closest behavioral analogue. UA rotation, retry, 5-strategy tag extraction, mobile detail enrichment.
  - `krsite-dl/krsite-dl/krsite_dl/extractor/naverblog.py` — Python; comprehensive whole-blog walk pattern.
  - User's `~/.claude/skills/chilly-monthly-report/scripts/fetch_sheet.py` (lines 685–768) — production Playwright code for engagement; **this is the contract the new CLI must match exactly for `likes`/`comments`**.
  - User's `~/.claude/skills/chilly-monthly-report/scripts/lib/naver_url.py` — clean canonicalization; port semantics directly into Go.
- **Auth:** None for read endpoints. Headers `User-Agent`, `Referer`, `Accept-Language: ko-KR,ko;q=0.9,en;q=0.8` are load-bearing.
- **Rate limiting:** No published numbers. Working scrapers space 1–3s between requests, 3 retries with exponential backoff (1.5s → 3s → 4.5s). The CLI uses `cliutil.AdaptiveLimiter` per source.
- **Iframe gotcha:** PC URL wraps body in `<iframe id="mainFrame" src="/PostView.naver?...">`. Mobile URL does not. **Always use mobile URL for body extraction.**

## User Vision (from Gate 1 + briefing)
- General-purpose v1; standalone, not yet integrated into the Chilly cron.
- **Output accuracy must be byte-identical** to what's rendered on the live page at the same instant. No "within X%" tolerances. If a field can't be obtained exactly, omit it. (Saved as feedback memory.)
- 5 commands: `search`, `hashtag`, `post`, `post-batch`, `feed`.
- Output: JSON to stdout default; `--output <path>` for file; `--db <path>` for SQLite.
- Maintenance budget: low ("fix when broken"). No `doctor` in v1 but clean exit codes + structured JSON errors so it can be retrofitted.
- Keep CLI private (not for public-library publish).

## Product Thesis
- **Name:** `naver-blog-pp-cli`
- **Headline:** Every Naver Blog scraping pattern the OSS ecosystem has learned, in one Go binary — with a local SQLite store, an offline FTS search, and engagement counts that match the live page exactly.
- **Why it should exist:** Today the user's Chilly pipeline is a `web_search` + `web_extract` Hermes job plus a Playwright helper. The new CLI gives them one tool, one config, one set of exit codes, deterministic JSON, and a queryable historical store — without a server, a Chrome instance running in the background, or a 25k/day Naver Developer quota.

## Build Priorities
1. **Foundation (Priority 0):** SQLite store with `posts`/`blogs`/`searches`/`engagement_history` + FTS5; URL canonicalizer port; HTTP client with UA rotation + `cliutil.AdaptiveLimiter` + retry/backoff; date-format normalizer.
2. **Absorbed core (Priority 1):**
   - `search` — `search.naver.com/search.naver?where=view&query=` static parse (with selector resilience: try modern `sds-comps-*` first, fall back to `li.bx._svp_item`)
   - `hashtag` — same plumbing as `search` with `#` prefix, plus a `--require-all` AND mode
   - `post` — `m.blog.naver.com/{blog_id}/{log_no}` HTML parse (title, body, category, tags via 5-strategy, comment count via regex)
   - `post-batch` — same as `post` but with concurrency + rate limit + per-URL error JSON
   - `feed` — `/api/blogs/{blog_id}/post-list` JSON, fallback to `PostTitleListAsync.naver` (with hames's invalid-escape sanitizer)
   - URL parsing helper (importable from Go; matches `lib/naver_url.py` semantics)
3. **Engagement subsystem (Priority 1, critical):**
   - **Plan A:** Use the public reaction/like XHR if Phase 1.7 browser-sniff finds it. (Apify's `hanpro` claims this exists.) **This is the single highest-value Phase 1.7 outcome — eliminates Playwright.**
   - **Plan B:** Embed a headless-browser fallback via `chromedp` for the iframe `span.u_likeit_text._count.num` extract, mirroring the user's `fetch_sheet.py` semantics exactly. Selected automatically when Plan A is unavailable. Documented `--no-browser` flag for users who don't have Chrome.
4. **Transcendence (Priority 2):** populated by the novel-features subagent in Phase 1.5c.5.
5. **Polish (Priority 3):** terse-flag enrichment, README quick-start, MCP read-only annotations.

## Open Questions for Phase 1.7 Browser-Sniff (locked priority)
1. **The reaction/like XHR.** Open `m.blog.naver.com/<blogId>/<logNo>` in DevTools, filter Network for `like` / `sympathy` / `reaction` / `count` / `unify`. Capture: URL, query params, required headers (especially CSRF or referrer-bound), response shape. **If this is parameterless or only needs a public referer, Plan A above ships and `chromedp` is removed.**
2. **`m.blog.naver.com/CommentList.naver` viability.** snudm uses the dated `.nhn` form. Confirm `.naver` returns parseable HTML/JSON and what headers it needs. Useful if we want to optionally list comments (currently scope is count-only).
3. **Hashtag-native JSON endpoint.** Probe `section.blog.naver.com/api/...` and `blog.naver.com/api/...` while browsing tag-filtered views. If a `?tag=` JSON endpoint exists, it replaces the brittle `search.naver.com?where=view` parsing for the `hashtag` command.

## Acceptance Criteria (locked, accuracy-corrected)
- All 5 commands return valid JSON for the canary queries from the Gate 1 cron (6 keywords/hashtags).
- Live test: **100% URL coverage** vs the most recent `chilly-naver-weekly` cron output for the same queries on the same date (modulo any posts genuinely deleted between runs).
- **Per-post engagement: byte-identical to what's currently visible on the page at the same instant.** No tolerance band. Cross-time drift is data changing, not a defect.
- No auth required (`unset NAVER_CLIENT_ID NAVER_CLIENT_SECRET`; empty cookie jar).
- README documents HAR-capture steps for future re-sniffing when Naver next rotates selectors.
- Clean exit codes (0 / 2 / 3 / 4 / 5 / 7 / 10) + structured JSON errors.

## Top Pain Points to Beat
1. **Likes need JS to render** (every static scraper sees 0). The CLI fixes this by sniffing the XHR if it's public, falling back to embedded `chromedp` otherwise — without the user installing Playwright or maintaining a Chromium dep.
2. **Search selectors rotate every few months** (scrapfly's 2025/26 note on `sds-comps-*`). The CLI ships dual-selector strategy (modern + legacy) and a `--debug-html` flag to make re-discovery cheap.
3. **PC URL iframe wrap** confuses every first-time scraper. The CLI canonicalizes every input URL to mobile form and exposes that as the source of truth.

## References (verbatim from the Phase 1 research agent)
- [scrapfly: How to Scrape Naver.com 2026](https://scrapfly.io/blog/posts/how-to-scrape-naver)
- [scrape.do: Scraping Naver](https://scrape.do/blog/naver-scraping/)
- [krsite-dl naverblog.py](https://github.com/krsite-dl/krsite-dl/blob/master/krsite_dl/extractor/naverblog.py)
- [baek-labs/hames naver_blog_scraper.js](https://github.com/baek-labs/hames/blob/main/arsenal/naver_blog_scraper.js)
- [snudm/naver-blog-crawler](https://github.com/snudm/naver-blog-crawler)
- [tiger-beom/naverblog_scraping](https://github.com/tiger-beom/naverblog_scraping)
- [isnow890/naver-search-mcp](https://github.com/isnow890/naver-search-mcp)
- [Naver OpenAPI guide](https://naver.github.io/naver-openapi-guide/apilist.html)
- [Apify hanpro/naver-blog-scraper](https://apify.com/hanpro/naver-blog-scraper)
