# Naver Blog CLI — Absorb Manifest

**Stamp:** 2026-05-19-162524
**Gate 1 reference:** [`~/.claude/plans/pp-naver-blog-gate1.md`](/Users/johnchang/.claude/plans/pp-naver-blog-gate1.md)
**Auth:** `none` (Gate 1 lock; verified at Phase 1.9 reachability gate)
**Accuracy criterion:** 100% same-instant match (CLI + MCP). No "within X% variance."

## Tools surveyed

| Tool | Status | Contribution |
|---|---|---|
| baek-labs/hames (`arsenal/naver_blog_scraper.js`) | Working (Node.js) | Mobile API + PostTitleListAsync fallback, UA rotation, retry, 5-strategy tag extraction |
| krsite-dl/krsite-dl (`extractor/naverblog.py`) | Working (Python, 2026-02 commit) | Whole-blog walk, PostView iframe handling, date format normalizer, image/video extraction |
| snudm/naver-blog-crawler | Stale (Python 2, `.nhn` endpoints) | Section directory crawl pattern, comment-list breadcrumb |
| tiger-beom/naverblog_scraping | Working pattern (Python, 2021) | search.naver.com SERP parsing, iframe-strip, body via `.se-main-container` / `#postViewArea` |
| isnow890/naver-search-mcp | Working (TS, official API) | MCP tool naming reference (uses official Naver Search API with X-Naver-Client-Id) |
| naver-blog-backer (PyPI) | Working (small) | Post backup to markdown reference |
| Apify hanpro/naver-blog-scraper | Working (closed) | Most feature-complete: likes via "Naver's reaction API" (which we now have public), `listNumComment`, `tags` array, dedup; warns `sds-comps-*` selectors rotate |
| User's `~/.claude/skills/chilly-monthly-report/scripts/fetch_sheet.py` | Working (production) | Playwright-based engagement scraper; **the contract the new CLI must match exactly** for `likes` / `comments` |
| User's `~/.claude/skills/chilly-monthly-report/scripts/lib/naver_url.py` | Working (production) | URL canonicalization across 3 input shapes; port to Go directly |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Status | Added Value |
|---|---------|------------|-------------------|--------|-------------|
| 1 | Per-blog feed walk (mobile JSON API) | hames + krsite-dl + hyunsikhwang | `blogs feed <blog_id>` via `m.blog.naver.com/api/blogs/<id>/post-list` | shipping | --since cursor, SQLite cache, MCP-exposed, --category filter (N8) |
| 2 | Per-blog feed PC fallback (PostTitleListAsync) | hames (invalid-escape sanitizer) | secondary path triggered when mobile feed returns 0 items | shipping | Sanitization built-in; one-flag opt-in |
| 3 | Mobile post detail (body + meta + tags) | hames + user fetch_sheet.py | `posts get` reads `.se-main-container`, og:* meta, gsTagName | shipping | URL canonicalization removes iframe-trick friction |
| 4 | Tag extraction (gsTagName CSV) | hames (5-strategy) | `gsTagName` regex (verified working, single source of truth) | shipping | One stable strategy beats five flaky ones; documented sources for fallback |
| 5 | Static-HTML comment count | user fetch_sheet.py | PostView.naver `<em id="commentCount">N</em>` regex | shipping | Zero JS rendering; empty = 0; exact integer |
| 6 | **Likes count (PHASE 1.7 WIN)** | (no OSS scraper had this publicly) | `apis.naver.com/blogserver/like/v1/search/contents` direct, batched in 1 call per N posts | shipping | **Eliminates Playwright dependency entirely.** Same integer Naver renders to the page |
| 7 | UA rotation + retry backoff | hames (3 UAs, 1.5s/3s/4.5s) | `cliutil.AdaptiveLimiter` per source | shipping | Press-standard, 429-aware, surfaces `*cliutil.RateLimitError` |
| 8 | Date format normalization | krsite-dl (10/13-digit epoch, YYYYMMDDHHMMSS, Korean relative) | unified ISO output across all outputs | shipping | Consistent timestamps everywhere |
| 9 | URL canonicalization (3 input shapes → mobile) | user `lib/naver_url.py` | port to Go in `internal/lib/naverurl/` | shipping | Mirrors Python semantics, importable from every command |
| 10 | Keyword search SERP (mobile) | tiger-beom + hames | `search posts --query <q>` via `m.search.naver.com/search.naver?where=m_view` | shipping | Dual-selector strategy planned; --debug-html flag |
| 11 | Hashtag search via `#` prefix | hames + tiger-beom (extractor) | `search hashtag --tags <tags> [--require-all]` | shipping | Only OSS tool with --require-all intersect mode |
| 12 | Korean UTF-8 + URL-encoding | all working scrapers | native Go strings + `url.QueryEscape` | shipping | + Korean-aware SQLite FTS5 tokenization (`unicode61` + trigram fallback) |
| 13 | Post body backup to markdown | naver-blog-backer (PyPI) | `posts get --format markdown` | shipping | One flag, one binary; no Python required |
| 14 | MCP exposure (read-only tools) | isnow890/naver-search-mcp (official-API only) | every read-only command auto-exposed via Cobra-tree mirror | shipping | Works without Naver Developer credentials, no 25k/day quota |
| 15 | Batch post lookup | (none in OSS; Apify hanpro is closed) | `post-batch` with reaction API batched | shipping | New OSS capability; batched in 1 HTTP call per N posts |
| 16 | Snippet / brief contents | feed JSON `briefContents` | `feed` output column | shipping | Free with feed; no separate fetch |
| 17 | Read / view count | feed JSON `readCount` | `feed` output column | shipping | Nobody else surfaces this in OSS |
| 18 | Engagement source provenance | (n/a) | `engagement_source` column: `feed` \| `reaction-api` \| `static-html` | shipping | Audit trail for accuracy debugging |
| 19 | Local SQLite cache + FTS5 | (no OSS competitor has this) | `internal/store/` with posts / blogs / searches / engagement_history tables | shipping | Foundation for transcendence (N1, N3, N5, N6) |
| 20 | Clean exit codes + structured JSON errors | (none) | `0/2/3/4/5/7/10` per press standard | shipping | Gate 1 explicit acceptance criterion |

**Counts:** 20 absorbed features, 0 stubs.

## Transcendence (only possible with our approach)

8 survivors from the novel-features subagent. Score ≥5/10 minimum. **Hand-code count: 5 features are fully hand-code (N1, N3, N4, N5, N6). N2/N7/N8 have a small hand-code portion plus spec-emits.** Approving this manifest commits the build to ~600-1000 LoC across `internal/cli/` and `internal/lib/`.

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| N1 | Engagement diff over time | `posts diff <url> [--since <ts>]` | 9/10 | hand-code | Requires local `engagement_history` snapshots in SQLite — no API call answers "what changed since I last looked." Competitors are point-in-time only. Persona: Eunji (Monday "what's new"), Jin-ho (Friday "what moved"). |
| N2 | Sponsored-post detector | `posts get --flag-sponsored` flag (column flows through feed/search/post-batch) | 8/10 | spec-emits flag + hand-code matcher | Mechanical regex over `body_text` for KFTC-required Korean disclosure phrases (협찬, 체험단, 광고 포함, 유료광고 포함, "본 포스팅은 ... 받아 작성됨"). No OSS tool composes this. Persona: Jin-ho (weekly sponsored audits). |
| N3 | Influencer health snapshot | `blogs health --ids-file <file> --window 7d` | 8/10 | hand-code | Combines feed walk + engagement + sponsored flag (N2) over N blogs in one command. Pure local SQL after sync. Persona: Jin-ho's Friday ritual. |
| N4 | Cron-mode bundle | `cron <queries-file>` | 8/10 | hand-code | Replaces the Chilly Hermes web_search+web_extract+Playwright pipeline with one binary call. Reads queries.yaml, runs search/hashtag, enriches engagement (reaction API), dedupes by canonical URL, streams TSV. Persona: Eunji's headline integration. |
| N5 | Hashtag co-occurrence | `search hashtag-neighbors <#tag> --top 20` | 6/10 | hand-code | Local SQL join over `posts.tags_json` for matched posts. Korean blog culture clusters hashtags densely; no API surfaces this. Persona: Eunji (campaign-tag strategy). |
| N6 | Offline Korean FTS | `search local <query>` | 7/10 | hand-code | FTS5 (`unicode61` + trigram) over `posts_fts`; ranked by FTS + likes. No Naver tool ships offline Korean search. Persona: Eunji + Jin-ho rerun the same historical queries weekly. |
| N7 | File-driven URL backfill | `posts batch --from-file <path>` | 7/10 | spec-emits flag + hand-code file reader | Accepts CSV/JSON/newline-delimited URLs, canonicalizes each, runs concurrent `posts get` with AdaptiveLimiter, **preserves input order**, emits per-URL error JSON. Persona: Su-ah's Slack-bundle workflow. |
| N8 | Category browse | `blogs categories <blog_id>` + `--category` flag on feed | 5/10 | spec-emits flag + hand-code summary | Reads cached `categories_json`; --category applies SQL filter or live URL param. Naver-specific numbered category system. Persona: Jin-ho (topic segmentation). |

## Stubs in this manifest

**None.** Every row is shipping-scope. The user has not approved any stubs.

## Locked defaults (Phase 1.5 gate)

- **Hashtag intersect with `--require-all`** — **local intersect**: run N SERP searches in sequence, intersect URL sets in Go. Auditable (`--debug` shows per-tag set sizes). Cost: N HTTP calls for N tags (~100ms each).
- **Cron-mode (N4) output format** — **JSONL on stdout by default**. `--format jsonl|tsv|csv` switches. Rationale: keep the CLI format-agnostic. Chilly Hermes cron wrapper converts JSONL → TSV via `jq` before pushing to Google Sheets.
- **Concurrent fanout (`posts batch` / `cron`)** — **5 concurrent, 1s pacing** per `cliutil.AdaptiveLimiter`. Exposed as `--concurrency` and `--pacing` flags for tuning. Reaction API is batched in 1 call per 20 URLs so likes have zero rate-limit risk; bottleneck is post HTML fetches.

## Combined feature surface

- **Top-level commands the user invokes:** `posts get`, `posts batch`, `posts diff`, `blogs feed`, `blogs health`, `blogs categories`, `search posts`, `search hashtag`, `search hashtag-neighbors`, `search local`, `cron`, `reactions get`, `db`, `doctor`, `version` — 15 top-level surfaces.
- **MCP tool surface:** All read-only commands auto-mirrored (~12 user-facing + ~13 framework = ~25 tools). Under the <30 default-MCP threshold; no MCP enrichment needed in Phase 2.
- **Gate 1 command alias commitment:** Phase 3 adds shorthand top-level commands `post`, `post-batch`, `feed`, `search`, `hashtag` that delegate to the resource/endpoint shape above. Gate 1 names work; press's idiomatic shape is also exposed.
