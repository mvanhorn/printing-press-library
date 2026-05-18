# Trustpilot CLI Brief

## API Identity
- Domain: Consumer review aggregator. Public review pages at `https://www.trustpilot.com/review/<domain>` (e.g. `trustpilot.com/review/thriftbooks.com`).
- Users: Researchers, analysts, sentiment-tracking agents, journalists, product teams, anyone evaluating a company before doing business. In our context: agent-native consumers (e.g. last30days) pulling recent reviews to enrich stories.
- Data profile: Public reviews keyed by `(domain, reviewId)`. Each review carries rating 1-5, title, body, publishedDate, experiencedDate, consumer name + country, verified flag, business reply, language. Per company: businessUnit metadata + trustScore + totalReviews + filter pagination.

## Reachability Risk
- High. AWS WAF + JS challenge + TLS fingerprinting (JA3/JA4). stdlib HTTP and Surf-with-Chrome-fingerprint both return `403 AWS WAF marker` directly (`printing-press probe-reachability` confirmed mode `browser_clearance_http`).
- Evidence: multiple post-2024 blog posts (RoundProxies, Scrapfly, Substack "Lab #53") confirm AWS WAF, document JS token challenge issuing `aws-waf-token` cookie with 5-15 minute lifespan.
- Workaround pattern (confirmed convergent across sources): one-shot Chrome via chromedp or agent-browser to harvest the cookie + page 1 data, then plain net/http for subsequent pages via `/_next/data/{buildId}/review/<domain>.json`. Cookie refresh on 403.
- No Cloudflare. WAF only.

## Top Workflows
1. **Pull recent reviews for an entity by domain** — `reviews thriftbooks.com --limit 100 --json`. The headline command. Agent-callable. Default sort: recency.
2. **Top recent good + bad** — `top-recent thriftbooks.com --window 30d --good 5 --bad 5 --json`. The killer integration for last30days: pulls a balanced view of recent sentiment in one call.
3. **Search company by name** — `search "thriftbooks"` returns canonical domain(s) so agent can resolve a vague entity name to a Trustpilot key.
4. **Sync to local SQLite** — `sync thriftbooks.com --max-pages 25` snapshots all reviews into a local FTS5-indexed store for offline queries.
5. **Sentiment drift over time** — `drift thriftbooks.com --weeks 12` compares trustScore + 5-star vs 1-star mix week-over-week from synced data.
6. **Cross-entity comparison** — `compare thriftbooks.com bookshop.org --window 90d` shows trustScore, review velocity, sentiment side-by-side from local store.

## Table Stakes
- Get reviews for a company (any scraper does this).
- Pagination through all pages (`?page=2..N`).
- Filter by stars (`?stars=1`), language (`?language=en`), sort (recency/relevance).
- JSON output mode.
- Per-review fields: id, rating, title, text, dates, consumer, verified, business reply, language.
- Company metadata: trustScore, totalReviews, displayName, domain, categories.
- 403 / WAF handling (cookie refresh or browser fallback).
- 20-reviews/page default. Bust the ~200-page cutoff by star-segmenting (`?stars=1..5` × `?language=en` × …).

## Data Layer
- Primary entities:
  - `companies` (domain, displayName, trustScore, totalReviews, categories, lastSyncedAt)
  - `reviews` (id, domain, rating, title, text, publishedAt, experiencedAt, consumerName, consumerCountry, verified, language, businessReplyText, businessReplyAt, rawJSON)
  - `sync_cursors` (domain, lastPage, lastBuildId, lastSyncedAt, totalPages, totalCount)
  - `cookies` (key=`aws-waf-token`, value, capturedAt, expiresAt, source)
- Sync cursor: per-domain `lastPage` + `lastBuildId`; restart pagination when buildId changes (Next.js deploy).
- FTS5: `reviews_fts` on `(title, text, businessReplyText)` with rating + language + domain columns for hybrid filter-then-search queries.
- Reviews are append-mostly (a review can be edited or deleted; UPSERT on `id`).

## Codebase Intelligence
- No DeepWiki run (no upstream "official" repo to query — this is a website target).
- Source: 3 Python scrapers + 2 commercial scraping guides (Scrapfly, ScrapeOps) confirm convergent `__NEXT_DATA__` extraction path.
- Auth: cookie (`aws-waf-token`). No bearer token, no API key for public review pages.
- Data model from `__NEXT_DATA__`:
  - `props.pageProps.businessUnit` — company metadata
  - `props.pageProps.reviews[]` — review array
  - `props.pageProps.filters.pagination.totalPages` — pagination
  - `__NEXT_DATA__.buildId` — top-level, used to construct `/_next/data/{buildId}/...json` URLs
- Rate limiting: IP-level. Single-digit pages/minute reliably works; high volume needs proxy rotation (out of scope for this CLI).
- Architecture insight: post-page-1 we can hit JSON-only API (`/_next/data/{buildId}/review/<domain>.json?page=N`) — sub-100ms responses, no HTML parsing. The cookie harvest + buildId lookup happens once per session.

## User Vision
- "Get as many Trustpilot reviews as we can — good AND bad — for an entity name. Fast, via CLI, stored locally in SQLite."
- "Imagine pairing with last30days. If someone ran `last30days thriftbooks` and the Trustpilot CLI was installed, it could pull in top recent reviews good and bad to potentially be featured in the last30days story."
- Implications:
  - **Speed matters.** First-page reviews should be in the user's hands in under 10 seconds (one browser harvest + one HTML parse).
  - **Balance matters.** Headline integration command must return both good and bad sides explicitly, not just "top N by recency."
  - **Agent-native is the point.** JSON output, agent_context manifest, MCP exposure must be first-class. last30days needs to call this without ceremony.
  - **Local SQLite is mandatory.** This is not nice-to-have; it's the storage layer for the integration. Reviews stay locally so the integration is free and offline after initial sync.

## Product Thesis
- Name: `trustpilot-pp-cli`
- Display name: Trustpilot (preserve canonical brand casing)
- Why it should exist:
  - No existing Go CLI for Trustpilot review scraping.
  - No MCP server, no Claude skill, no Claude plugin for Trustpilot anywhere on lobehub / mcpmarket / claude-plugins-official.
  - Existing Python scrapers are unmaintained, low-star, single-purpose, lack offline SQLite, lack agent-native JSON conventions, lack a "balanced good+bad recent" surface.
  - last30days integration requires a callable agent-tool, and Python scrapers don't provide that.
  - WAF bypass tooling (one-shot Chrome → cookie → JSON API) is identical work whether done in Python or Go — Go gives us single-binary distribution, no runtime deps, sub-second startup, and easy MCP exposure.

## Build Priorities
1. **Foundation**: SQLite schema (companies, reviews, sync_cursors, cookies, reviews_fts). Cookie harvest layer using agent-browser or chromedp. `__NEXT_DATA__` parser. `/_next/data/{buildId}/...` JSON client.
2. **Absorbed (table stakes)**: `reviews <domain>`, `search <name>`, `sync <domain>`, filter flags (stars, language, sort, date), `--json`, `--limit`, pagination, page-cutoff workaround via star-segmenting.
3. **Transcendence (novel)**: `top-recent <domain> --window 30d --good N --bad N` (last30days bridge); `drift <domain> --weeks N`; `compare <a> <b> --window 90d`; offline `search "<term> --rating 1"` via FTS5; `surge <domain>` detect review velocity spikes from synced snapshots; `sentiment-bins <domain>` showing the 5-bin rating distribution over time; `agent-bundle <domain>` one-shot agent payload with everything an external CLI needs.

## Reachability Strategy (definitive plan)
- **Browser layer**: agent-browser (already installed, 0.23.4) to fetch the company page, harvest `aws-waf-token` cookie, and dump the raw `__NEXT_DATA__` JSON. One invocation = one cookie + buildId + page 1 of reviews. ~5-10 seconds.
- **HTTP layer**: `internal/source/trustpilot/http.go` reuses the cookie via standard Go `net/http` against `/_next/data/{buildId}/review/<domain>.json?page=N&sort=...&stars=...`. Sub-100ms per page.
- **Refresh policy**: if any HTTP request returns 403 OR if cookie is older than 4 minutes (60% buffer below the 5-15min lifespan), re-harvest via browser.
- **Storage of cookie**: SQLite `cookies` table, single row per (host, token-name).
- **Fallback**: if agent-browser is missing on the user's machine, the CLI can attempt direct HTTP first (works briefly at low rates) and degrade gracefully with a clear error message naming the install path.
- **No proxy rotation in v1.** That's a known-gap. Document in README. Users hitting volume-limited 403s get a clear error.
