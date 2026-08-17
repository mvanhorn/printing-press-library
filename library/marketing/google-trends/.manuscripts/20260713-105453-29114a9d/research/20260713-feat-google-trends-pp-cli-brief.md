# Google Trends CLI Brief

## API Identity
- Domain: search-interest and trending-topic data from Google Trends (trends.google.com). No official public API exists (Google's "Trends API" is an invite-only alpha as of July 2025, not usable here). Access is the reverse-engineered internal widget/token contract that pytrends and its successors use.
- Users: marketers, SEO/content researchers, growth analysts, data scientists building seasonality/forecasting models, and agents doing ad-hoc "what's trending" checks.
- Data profile: time-series (0-100 relative interest scale, sampled/normalized per request — not absolute volume), geographic breakdowns, and short-lived trending-topic lists. No persistent per-user account data.

## Reachability Risk
- [High, CONFIRMED LIVE] evidence: pytrends (GeneralMills/pytrends, 3.7k★, the dominant wrapper) was archived/read-only on 2025-04-17 — maintainers effectively conceded the anti-scraping cat-and-mouse game. 429/blocking issues recur across its entire lifetime: #538 (2022), #560/#561 (2023, header spoofing didn't help), #578/#622 (2023, proxy rotation at GCP scale still hit 429s), #602 "It broke again" (2023), #631 (2024), #638 (2025-02, Google changed the trending endpoint contract outright). #594 "Is PyTrends working for ANYONE right now?" captures the ambient uncertainty.
- **Live probe confirms the risk empirically.** `cli-printing-press probe-reachability` against `trends.google.com/trends/explore` returned `mode: browser_clearance_http` — BOTH plain stdlib HTTP and Surf's Chrome-TLS-fingerprint transport got HTTP 429 anonymously (no cookie). A plain `curl` GET of the homepage also 429'd immediately (`Error 429 (Too Many Requests)!!1` — the literal Google rate-limit page). By contrast, a real headless-Chrome session via agent-browser (full CDP, real TLS + JS execution) succeeded cleanly through the entire explore→token→widget flow (see Codebase Intelligence below). **Conclusion: this API requires a clearance cookie harvested from a real browser session, not just a Chrome-TLS-fingerprint client.** `needs_browser_capture: true`, `needs_clearance_cookie: true` per the probe's own recommendation.
- Tier/permission hints from 4xx body: none — no auth tiers exist; blocking is anti-scraping (429/CAPTCHA-style), not permission-based.
- Probe-safe endpoint used: `probe-reachability` against `/trends/explore` (read-only GET, no state mutation).
- **Phase 1.9 base reachability check:** `GET /trends/api/explore/pickers/geo` returned **200** via plain unauthenticated `curl` (Result: 2xx → PASS per decision matrix). This reveals a two-tier reachability picture: the static lookup endpoints (geo/category pickers) are openly accessible with no cookie at all, but the actual data-fetching flow (`POST /api/explore` → `GET /widgetdata/*`) requires the browser-warmed NID clearance cookie confirmed above. The generated CLI should treat pickers as always-available and gate only the interest/region/related/trending commands behind `auth login --chrome`.

## Top Workflows
1. Keyword interest-over-time (0-100 relative scale) across a timeframe — the single most common call.
2. Multi-keyword comparison (up to 5 terms, shared scale) — competitive/brand/topic comparison.
3. Geographic breakdown (interest-by-region: country/region/DMA/city) — market prioritization, localization.
4. Related/rising queries and topics — SEO content-gap discovery, "what's coming next" signal.
5. Trending-now / daily/realtime trending searches — newsjacking, live monitoring dashboards.
6. (Secondary) Long-range historical + seasonality analysis via overlapping-window stitching beyond the native 5-year daily cap.

## Table Stakes
- Interest-over-time, multi-range interest-over-time, interest-by-region (comparedgeo), related topics/queries, top charts, autocomplete/suggestions, category picker, daily/realtime trending searches — all present in pytrends and its successors (trendspyg, pytrends-modern).
- Up to 5 keywords per comparison call (Google-imposed ceiling, not a wrapper choice).
- Timeframe presets (`now 1-H`, `now 7-d`, `today 5-y`, etc.) plus custom `YYYY-MM-DD YYYY-MM-DD` ranges (UTC-only).
- Property/gprop filter: web (default), images, news, youtube, froogle.
- Category filtering by numeric ID (no official name→ID table published anywhere — community-reverse-engineered).
- Export to CSV/JSON/DataFrame-equivalent (trendspyg, Nao-30/google-trends-cli).
- CLI framing seen in Nao-30/google-trends-cli: content-creator workflows (`trending`, `related`, `compare`, `topic-growth`, `writing-opportunities`).

## Data Layer
- Primary entities: `keyword_query` (a saved keyword+geo+timeframe+category+property combo and its results), `region_interest` (interest-by-region rows), `related_term` (topic/query, rising or top, per parent keyword), `trending_topic` (daily/realtime trending entries), `category` (id→name lookup table — shipped as seed data, since no API endpoint reliably serves it).
- Sync cursor: trending-topic data is time-windowed and short-lived (realtime = last few hours, daily = last few days) — sync means "pull latest window," not incremental cursor pagination.
- FTS/search: full-text search over locally cached related-terms and trending-topic history is a strong transcendence candidate (Google Trends itself has no way to search "what topics spiked involving X over the last 90 days I've tracked").

## Codebase Intelligence
- Source: GitHub source-reading of the two most relevant repos.
- gogtrends (groovili/gogtrends, 89★, Go, active): the only real Go prior art. Library only — no CLI. Exposes InterestOverTime, InterestByLocation, RelatedTopics/Queries, Daily, Realtime, autocomplete, categories/locations as Go functions. Its HTTP client and endpoint constants are a useful cross-check for the exact wire contract, but it ships no command surface — full whitespace for a Go CLI.
- pytrends (frozen reference, Python): two-step auth — GET `/explore/?geo={hl}` to harvest only the `NID` cookie, then POST `/api/explore` with `hl`, `tz`, `req` (JSON: comparisonItem[], category, property) to get per-widget tokens, then GET each widget URL with `{req, token, tz}`. All responses are prefixed with Google's XSSI defense string `)]}',` — strip 4 chars for `/api/explore`, 5 chars for virtually every widget/daily/realtime/topcharts/autocomplete/categories response, before JSON-decoding.
- Auth: no token/key — cookie-based session (`NID`) harvested from an anonymous page load, refreshed per retry/proxy rotation. `auth.type: cookie` in spec terms, but no login flow — the cookie is obtained anonymously.
- Data model: request→widget-token→widget-data three-hop pattern. `comparisonItem` is an array (keyword × geo pairs), category is a single numeric ID applied across the whole comparison, property (`gprop`) filters search vertical.
- Rate limiting: no documented quota; empirically throttles aggressively and inconsistently (429s reported even with rotating proxies at scale). Treat as "always rate-limit-defensive," not "check headers for a limit."
- Architecture insight: the two-request pattern (explore → token → widget) means every "single" user-facing command is actually 2 HTTP round-trips minimum; batching multiple widgets from one explore call (as pytrends does) is the efficient path and should be mirrored internally even though the CLI exposes single commands.
- **Live browser-sniff capture (2026-07-13) confirms the core contract exactly matches pytrends' documented shape and is currently live.** Captured via agent-browser (headless Chrome/CDP) driving a real search flow (type "coffee" → click Explore): `GET /trends/api/explore/pickers/geo`, `GET /trends/api/explore/pickers/category`, `POST /trends/api/explore` (returns per-widget tokens), `GET /trends/api/widgetdata/multiline` (interest over time), `GET /trends/api/widgetdata/comparedgeo` (interest by region), `GET /trends/api/widgetdata/relatedsearches` (related queries/topics, both rising and top via a `restriction` param) — all returned HTTP 200 inside the real browser session.
- **Trending Now has moved off pytrends' old REST-ish endpoints entirely** (confirms the #638 GitHub issue). It is now served by Google's internal `batchexecute` RPC protocol at `POST /_/TrendsUi/data/batchexecute?rpcids=<id>&f.sid=<session>&bl=<build-label>&...` with an opaque, versioned, nested-array `f.req` payload (URL-encoded JSON-in-JSON, not a clean object). Observed RPC IDs: `DqDTgb` (trending list), `Tnt4U` (unknown/secondary), `g4kJzf` (related-entity lookup for each trending term, keyed by geo+term). `f.sid` and `bl` are minted per page load and must be scraped from the `/trending` page's inline bootstrap JS or an initial handshake request before the RPC call can be replayed — this is NOT a stable, directly-callable endpoint the way the explore/widget flow is.

## User Vision
- (none — user selected "let's go" at briefing; no upfront vision provided beyond confirming the research-first-then-browser-sniff-to-fill-gaps approach during Phase 0 disambiguation)

## Product Thesis
- Name: Google Trends CLI (working title `google-trends-pp-cli`)
- Why it should exist: pytrends (the de-facto standard) is archived and frozen; its two live Python successors (trendspyg, pytrends-modern) plus every MCP server found either proxy a paid scraping API or hit the trivial RSS feed — none reimplement the real explore→token→widget flow themselves. No full-featured Go CLI exists at all (gogtrends is a library, and the two Go "CLI" projects found only scrape the trivial daily-trends RSS feed). This CLI absorbs the real widget-token contract, ships retry/backoff and proxy rotation as first-class (not bolted on, unlike pytrends), and adds a local SQLite layer so "what was trending around X three weeks ago" — a question Google Trends' own UI cannot answer — becomes a query instead of a re-scrape.

## Build Priorities
1. Data layer + explore→token→widget client with built-in retry/backoff and XSSI-prefix stripping (Priority 0 foundation) — this unlocks every other command.
2. Absorb every pytrends/trendspyg/gogtrends feature: interest-over-time, multi-range, interest-by-region, related topics/queries, daily/realtime trending, autocomplete, category+geo lookup tables shipped as seed data (Priority 1).
3. Transcendence: local SQLite history of every synced query (search/query-your-own-trend-history, rising-topic diffing across snapshots, seasonality detection via stitched multi-window daily data, offline related-term full-text search) (Priority 2).
