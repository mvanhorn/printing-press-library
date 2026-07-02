# The Points Guy CLI Brief

## API Identity
- Domain: Travel rewards / points-and-miles / credit-card content site (The Points Guy, thepointsguy.com; owned by Red Ventures).
- Users: Points-and-miles enthusiasts, credit-card optimizers/churners, frequent travelers, and AI agents that need TPG's card terms + valuations as structured data.
- Data profile: Editorial articles & news, a credit-card database (~196 card + category URLs), TPG's signature **monthly points valuations** (cents-per-point for airline/hotel/transferable currencies), guides, and a glossary.
- Auth: NONE. All content is public, SSR, and reachable with a plain HTTP client. No key gate.

## Reachability Risk
- None. Every probed surface returns 200 to a normal User-Agent. No Cloudflare/WAF challenge, no login wall for content.
- Runtime mode: standard_http (plain replayable HTTP). No browser sidecar required.

## Discovered Replayable Surfaces (browser-sniff via direct-HTTP discovery; pre-approved website target)
1. **Algolia search** — indexes `TPG_RUNWAY_PROD` (content), `TPG_NEWSLETTERS_PROD`, `TPG_RUNWAY_PROD_query_suggestions`.
   - Endpoint: `POST https://{appId}-dsn.algolia.net/1/indexes/TPG_RUNWAY_PROD/query`, headers `x-algolia-application-id` + `x-algolia-api-key` (public search-only key), body `{"query":"...","hitsPerPage":N}`.
   - Verified: query "amex platinum" -> 275 hits, 6ms. Hit fields: title, author, category, date, url, featured_image, objectID.
   - App ID + search key are public frontend values embedded in `_app-*.js`. **CLI discovers them at runtime** (fetch homepage -> find _app chunk -> extract), so no credential value is baked into source and key rotation is survived.
2. **SSR page HTML `__NEXT_DATA__`** — `props.pageProps.dehydratedState.queries[]` (React Query cache). Query keys observed: `Article` (article body/author + structured credit-card attributes: Purchase APR, Balance Transfer APR, Cash Advance APR, Penalty APR, intro APRs, fees), `FilteredArticleTiles`, `PageDisambiguator`, `CachedTime`. buildId rotates per deploy, so `/_next/data/<buildId>/*.json` is unreliable; SSR-HTML extraction is the robust path.
3. **RSS feed** `/feed/` (text/xml, ~358KB) — latest articles: title, link, pubDate, dc:creator, category, description, content:encoded.
4. **Sitemaps** (from robots.txt): `wp-sitemap.xml` -> per-category article sitemaps (airline, aviation, credit-cards, cruise, deals, disney, hotel, loyalty-programs, news x9, other); `sitemap_cards.xml` (196 card + category URLs); `news-sitemap.xml` (recent news); `sitemap-nl-archive.xml` (newsletters).
5. **Valuations** — `/loyalty-programs/monthly-valuations/` article HTML carries the cents-per-point tables (e.g. Chase Ultimate Rewards, Amex MR, Marriott Bonvoy, Hilton Honors, Delta SkyMiles, United MileagePlus; values like 2.05c / 2.2c / 1.9c).
- Backend origins seen in runtimeConfig (not called directly; SSR/Algolia preferred): aerodrome BFF (tpg-aerodrome...), headless WordPress (tpg-runway...), watchtower API.

## Top Workflows
1. "What are my points/miles worth?" -> valuations lookup by program, in cents-per-point, with redeem-vs-cash guidance.
2. "Search TPG for X" -> Algolia full-text search across news, guides, reviews, deals.
3. "Show me card <name>" -> card detail: welcome bonus, annual fee, APRs, rewards, from the card DB.
4. "Best cards for <category>" -> best-of category pages (travel, airline, no-annual-fee, lounge access, etc.).
5. "What's the latest?" -> recent news/deals via RSS + news sitemap.
6. "Read <article>" -> fetch + render an article's content as text/markdown.

## Table Stakes (from adjacent tools; no direct TPG CLI exists)
- Full-text content search (site search).
- Points/miles valuations lookup (AwardWallet, NerdWallet, Frequent Miler all publish these).
- Credit-card lookup with structured terms (NerdWallet/TPG card pages).
- Latest-articles / news feed (RSS readers).
- Category browse.

## Data Layer
- Primary entities:
  - `articles` (objectID, title, url, author, category, date, excerpt, body)
  - `cards` (slug, name, url, issuer, network, annual_fee, welcome_bonus, purchase_apr, balance_transfer_apr, cash_advance_apr, rewards, category_tags)
  - `valuations` (program, program_type[airline|hotel|transferable|other], cents_per_point, month, source_url)
- Sync cursor: article date / valuations month.
- FTS/search: FTS5 over articles (title+body) and cards (name+rewards) for offline search beyond Algolia.

## Product Thesis
- Name: The Points Guy CLI (`thepointsguy-pp-cli`); display "The Points Guy".
- Why it should exist: TPG is the reference source for points valuations and card terms, but that data is trapped in long articles and JS-rendered pages. This is the first CLI + local SQLite mirror that makes TPG's valuations, card database, and content queryable, scriptable, offline, and agent-native (--json, --select, typed exits) — plus a live Algolia search no scraper reproduces.

## Build Priorities
1. Data layer + sync for articles / cards / valuations (Priority 0).
2. Absorbed surface: search (Algolia), card lookup, best-of category, valuations lookup, latest/news, read-article, browse-by-category (Priority 1).
3. Transcendence: local FTS, valuation-based redeem/cash calculator, card compare, valuation drift over months, "worth it?" redemption checker, portfolio valuation (Priority 2).

## Reachability Gate
- Decision: PASS
- Evidence: homepage 200 text/html; /feed/ 200 xml; /robots.txt 200; sitemap_cards.xml 200 (196 urls); Algolia TPG_RUNWAY_PROD query 200 (275 hits, 6ms). No challenge/WAF. Runtime mode: standard_http.
