# Trustpilot CLI — Absorb Manifest

## Goal
Build a Go CLI for Trustpilot reviews that matches every feature in every existing scraper
(Python and JS) AND ships features no existing tool has — with a particular focus on the
last30days agent-pairing workflow the user described in briefing.

## Absorbed features (match or beat the existing universe)

| #  | Feature                                | Best Source                            | Our Implementation                                  | Added Value                                                                |
|----|----------------------------------------|----------------------------------------|-----------------------------------------------------|----------------------------------------------------------------------------|
| 1  | Fetch reviews for a company            | irfanalidv/trustpilot_scraper (Python) | `reviews <domain>` via cookie + `/_next/data` JSON  | Go single-binary; sub-100ms per page after cookie warmup; agent-native JSON|
| 2  | Paginate through all reviews           | All scrapers                           | `reviews <domain> --max-pages N` auto-loop          | Bust 200-page cutoff by star-segmenting (1..5)                             |
| 3  | Filter by star rating                  | Scrapfly guide                         | `--stars 1..5` flag (repeatable)                    | Multi-rating selection + FTS5 offline filter                               |
| 4  | Filter by language                     | Scrapfly guide                         | `--lang en` flag                                    | ISO 639-1 codes                                                            |
| 5  | Sort by recency or relevance           | Scrapfly guide                         | `--sort recency\|relevance` (default recency)       |                                                                            |
| 6  | Filter by date window                  | Trustpilot site                        | `--date last30days\|last3months\|last6months\|last12months` |                                                                    |
| 7  | Filter by verified-only                | Trustpilot site                        | `--verified` flag                                   |                                                                            |
| 8  | Search company by name                 | Scrapfly guide                         | `search "<name>"` via JSON-API + HTML fallback      | Resolves vague names to canonical domain key                               |
| 9  | Get business unit metadata             | All scrapers                           | `info <domain>`                                     | TrustScore, totalReviews, stars distribution, categories, isClaimed        |
| 10 | Get business reply on each review      | All scrapers                           | First-class `reply` field in JSON output            |                                                                            |
| 11 | Export to CSV / JSON                   | trustpilot-scraper (Python)            | `--json --csv --select --compact --quiet`           | Dotted-path field filtering via `--select`                                 |
| 12 | Detect 403 / WAF block                 | None                                   | Auto cookie refresh on 403 + age > 4 min            | Built-in retry — no scraper does this transparently                        |
| 13 | Cookie harvest via headless browser    | AndreaBilliar (Selenium UC)            | `auth login --chrome` via agent-browser/chromedp    | Persisted to SQLite; resumes across CLI invocations                        |
| 14 | List companies by category             | Trustpilot site                        | `categories list`, `categories show <slug>`         |                                                                            |
| 15 | Trustpilot's own AI summary            | Trustpilot site                        | First-class field in `info` output                  | Pre-computed AI summary surfaced for downstream agents                     |
| 16 | Similar / competitor companies         | Trustpilot site                        | `info <domain> --similar` returns 8 entries         |                                                                            |
| 17 | Review distribution histogram          | Computed from API                      | `info` includes 5-bin rating histogram              | Reads filters.reviewStatistics.ratings                                     |

## Transcendence features (only possible with our approach)

| #  | Feature                              | Command                                                      | Persona       | Score  | How It Works                                                                                                                                       | Evidence                                                                                                                |
|----|--------------------------------------|--------------------------------------------------------------|---------------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------|
| 1  | Balanced recent slice                | `top-recent <domain> --window 30d --good N --bad N --json`   | Riley         | 10/10  | Two filtered `/_next/data/{buildId}/review/<domain>.json` pulls (`stars=4..5` + `stars=1..2`) merged and date-filtered locally                     | User Vision explicitly names last30days pairing as headline workflow; no existing scraper exposes a balanced-mix surface |
| 2  | One-shot agent bundle                | `agent-bundle <domain> --json`                               | Riley         | 10/10  | Composes `info` + `top-recent` + Trustpilot AI summary + 5-bin histogram into one JSON payload; ideal MCP single-tool surface                      | User Vision emphasizes agent-native + MCP exposure first-class; no scraper / no MCP server exists for Trustpilot         |
| 3  | TrustScore + sentiment drift         | `drift <domain> --weeks 12 --json`                           | Sam, Jules    | 8/10   | Local SQLite aggregation over `reviews.publishedAt` bucketed by ISO week; per-week count, 1-star %, 5-star %, mean rating                          | Brief Top Workflow #5; Trustpilot has no historical-trustscore endpoint                                                  |
| 4  | Multi-company comparison             | `compare <a> <b> [<c>...] --window 90d`                      | Sam           | 8/10   | Joins `companies` + windowed `reviews` tables in local store across N domains; side-by-side TrustScore, velocity, 1/5-star mix                     | Brief Top Workflow #6; no existing scraper supports multi-company queries                                                |
| 5  | Review-velocity surge                | `surge <domain> [--baseline 90d --window 7d --stars 1]`      | Sam           | 7/10   | Z-score of recent-window review count or 1-star count vs rolling baseline computed in SQL over synced rows                                         | Trustpilot pile-ons precede news cycles; no scraper surfaces this                                                        |
| 6  | Offline review FTS                   | `search-reviews <domain> "<term>" [--stars 1 --window 90d]`  | Jules, Riley  | 9/10   | FTS5 query against `reviews_fts(title,text,businessReplyText)` filtered by rating/language/date columns                                            | Data Layer in brief designs reviews_fts; Trustpilot UI has no per-company review-text search                             |
| 7  | Business-reply gap                   | `replies <domain> [--unreplied --stars 1]`                   | Sam, Jules    | 7/10   | GROUP BY `rating` with `businessReplyAt IS NULL` predicate over synced reviews; lists unreplied 1-stars                                            | Every scraper carries the reply field; none aggregates reply rate                                                        |
| 8  | Country-of-reviewer mix              | `geo <domain> [--window 90d]`                                | Sam, Jules    | 6/10   | GROUP BY `consumerCountry` on synced rows; per-country count, avg rating, 1-star rate                                                              | consumerCountry in every review payload but never aggregated                                                             |
| 9  | Trustpilot topic AI passthrough      | `topics <domain> --json`                                     | Riley, Jules  | 8/10   | Reads `props.pageProps.topicAiSummaries[]` straight from the cached `__NEXT_DATA__`; no LLM, no synthesis                                          | Brief Data domain calls out topic AI as first-class; not in absorb scope                                                 |
| 10 | Similar-companies sweep              | `similar-sweep <domain>`                                     | Sam           | 7/10   | Fans out the 8 `similarBusinessUnits` from `info` into parallel `info` calls; ranks by TrustScore/totalReviews                                     | Builds on absorbed `info --similar`; Trustpilot site requires 9 manual page loads to assemble                            |
| 11 | Sync delta                           | `whats-new <domain> [--since <iso> --json]`                  | Riley, Sam    | 8/10   | Compares `reviews.publishedAt` against `sync_cursors.lastSyncedAt`; emits new reviews bucketed by star rating                                      | sync_cursors in Data Layer; agent polling pattern, no equivalent on Trustpilot                                           |
| 12 | Category leaderboard                 | `category-top <slug> [--limit 25 --min-reviews 100]`         | Sam           | 6/10   | Calls absorbed `categories show <slug>` and ranks results by TrustScore (with optional totalReviews floor)                                         | Absorbed `categories` returns navigation list, not ranking; Sam's prospecting frustration                                |

## Status notes
- No stubs. Every row in this manifest will ship as a functional command.
- All transcendence commands depend on the local SQLite store being populated via `sync`.
  `top-recent` and `agent-bundle` ALSO have a "live-first-no-sync" path that fetches the
  required two filtered pages and answers in <10 seconds without requiring a prior sync.
- `drift`, `surge`, `geo`, `replies`, `whats-new` require a prior `sync` and fail with a
  clear message naming the command to run if the store is empty.

## Killed (audit trail, not for build)

| Feature                              | Kill reason                                                              | Closest surviving sibling |
|--------------------------------------|---------------------------------------------------------------------------|---------------------------|
| Reviewer overlap                     | Trustpilot consumer identity isn't stable enough; verifiability fails    | C4 compare                |
| Language split                       | Strictly weaker than geo; fold language into `info` filters              | C9 geo                    |
| Verified-vs-unverified split         | Interesting once, not weekly; fold into `info`                           | C7 replies                |
| One-star pile-on                     | Pure subset of `surge --stars 1`                                         | C5 surge                  |
