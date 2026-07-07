# Google Play CLI Brief

## API Identity
- Domain: play.google.com (public store web surfaces; no official public store-browsing API exists)
- Users: mobile game/app studios (market intel, competitor tracking), ASO practitioners, indie devs, data analysts, AI agents needing structured store data
- Data profile: app listings (60+ fields incl. installs, ratings histogram, IAP range, data safety), top charts (3 collections x 58 categories x country x language), reviews (paginated, star/device filterable), search results, similar-apps graph, developer portfolios, autocomplete suggestions

## Reachability Risk
- Low (transport) / Medium (volume). All surfaces live-verified 2026-06-11 with plain curl: HTTP 200 on details/search/datasafety/developer GETs and on batchexecute POSTs (oCPfdb, UsvDTd, xdSrCf, IJ4APc, vyAe2, qnKhOb). No auth, no API key, no CSRF (the at= token is optional), no consent redirect from non-EU vantage.
- Volume risk: documented 429 enforcement since 2022 (facundoolano#590), 503+captcha with ~1 hour IP bans on aggressive fan-out (README "Throttling"), and a rate-limit sentinel string `com.google.play.gateway.proto.PlayGatewayError` inside HTTP-200 batchexecute bodies (JoMingyu utils/request.py treats it as throttle, retries 3x with escalating sleep).
- Mitigation contract for the CLI: default ~1 req/s throttle, exponential backoff on 429/503, PlayGatewayError detection as typed RateLimitError, aggressive local caching, avoid fullDetail fan-out by default.
- Parser fragility: responses are positional protojson (anonymous nested arrays); index paths shift roughly once a year on store redesigns (2018, 2019, 2020, 2022 incidents). Mitigations used by reference scrapers: fallback index paths, AF_dataServiceRequests service-request-id lookup instead of fixed ds: numbers, section probing.

## Top Workflows
1. Top-chart tracking: pull TOP_FREE / TOP_PAID / GROSSING for a game category + country, snapshot to SQLite, diff rank movements between snapshots (climbers, droppers, new entries). Nobody offers this without a self-hosted platform.
2. Competitor app intelligence: full details for an appId (exact install count, ratings histogram, IAP range, ads flag, recent changes, updated date), compare several competitors side by side.
3. Review mining: bulk-fetch reviews with sort + star + device filters, surface complaint themes and post-update sentiment, track developer reply rates.
4. ASO keyword work: where does my app rank for a search term per country, autocomplete suggestion mining (seed expansion), keyword difficulty signals from top-10 analysis.
5. Discovery/graph: similar-apps traversal, developer portfolio dumps, category browsing.

## Table Stakes
- app details (full field set incl. realInstalls exact count, sale/originalPrice)
- list/top charts (3 collections x 58 categories, num up to ~660 server cap)
- search (num up to 250 via qnKhOb pagination, price filter)
- reviews (pagination tokens, sort NEWEST/RATING/HELPFULNESS, star + device filters, developer replies, per-criteria game ratings)
- similar apps, developer portfolio (both /developer name and /dev numeric forms), suggest (autocomplete), permissions, data safety, categories enumeration
- hl/gl on everything; built-in throttle; response caching

## Data Layer
- Primary entities: apps (detail snapshots, keyed appId + fetched_at), chart_snapshots (collection, category, country, rank, appId, captured_at), reviews (reviewId, appId, score, text, at, device), keyword_ranks (term, country, rank, appId, captured_at), developers
- Sync cursor: captured_at timestamps per chart/keyword snapshot; reviews continuation tokens
- FTS/search: app title + description + summary; review text

## Codebase Intelligence
- Source: direct source reading of facundoolano/google-play-scraper (JS, reference impl, maintenance-only) and JoMingyu/google-play-scraper (Python, dormant since 2024-08), plus live curl verification
- Auth: none. No tokens, no cookies required (cookie jar recommended for pagination consistency only)
- Data model: positional protojson under AF_initDataCallback ds: keys on HTML pages (app details ds:5 path [1,2,...]); batchexecute envelope `f.req=[[["rpcid","<inner JSON string>",null,"generic"]]]`, response `)]}'` prefix + double-encoded JSON at envelope[0][2]
- Rate limiting: 429 / 503+captcha / PlayGatewayError-in-200; ~1 hour IP bans; throttle to ~1 req/s
- Architecture: GET+HTML parse for details/search-p1/datasafety/developer/similar/categories; batchexecute POST for charts (vyAe2), reviews (oCPfdb), permissions (xdSrCf), suggest (IJ4APc), pagination (qnKhOb). rpcids are the stable part; index paths the fragile part.

## Product Thesis
- Name: google-play-pp-cli (display: Google Play)
- Why it should exist: the reference scraper ecosystem is decaying (npm maintainer stepped back, Python frozen with broken reviews_all, the only Go port is 2.5 years stale), and every existing tool is stateless. A maintained single-binary Go CLI with a local SQLite mirror unlocks the two features the whole ecosystem lacks: chart-rank history/diffing and listing change detection, both of which commercial tools (Sensor Tower, AppTweak, AppMagic) paywall despite the raw data being public. Agent-native JSON output plus typed rate-limit errors make it the right Play Store tool for AI workflows.

## Build Priorities
1. Hand-written internal Play client in Go: batchexecute envelope/framing, AF_initDataCallback extraction with fallback paths, PlayGatewayError-aware rate limiting
2. Full absorb surface: details, charts, search, reviews, similar, developer, suggest, permissions, datasafety, categories
3. SQLite mirror + snapshot commands: chart snapshots, app detail snapshots, keyword rank snapshots
4. Transcendence: chart diffing, listing change detection, review intelligence, keyword rank tracking, competitor comparison
