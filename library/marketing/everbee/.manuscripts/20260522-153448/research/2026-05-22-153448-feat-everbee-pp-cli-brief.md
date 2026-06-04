# EverBee CLI Brief

## API Identity
- Domain: Etsy seller and print-on-demand market research.
- Users: Etsy sellers, POD creators, niche researchers, SEO operators, and agents comparing product, shop, and keyword opportunities.
- Data profile: product/listing metrics, shop competitor metrics, keyword metrics, tag analytics, trend windows, favorites, reviews, pricing, sales estimates, revenue estimates, and exports.
- Target: EverBee website workflows from:
  - https://app.everbee.io/product-analytics
  - https://app.everbee.io/shop-analyzer
  - https://app.everbee.io/keyword-research

## Reachability Risk
- Medium/high. The requested targets are app pages, not a documented OpenAPI spec. Auth uses Google SSO/browser session per user context. Discovery must browser-sniff authenticated traffic and only ship replayable HTTP/HTML surfaces.
- Programmatic results are likely tied to EverBee's private app/backend and account plan limits. The CLI should preserve honest errors for login-required, plan-gated, and export-email flows.

## Top Workflows
1. Product opportunity search: query a niche, return listing/product metrics ranked by demand, revenue, sales, reviews, conversion, and listing age.
2. Keyword opportunity research: query seed keywords, return volume, competition, keyword score, trends, related terms, and tags.
3. Shop competitor analysis: inspect an Etsy shop, summarize best-performing listings, pricing bands, tags, revenue estimates, and product gaps.
4. Tag/listing deep dive: pull tags and listing details from high-performing products, compare against keyword demand and competition.
5. Export and snapshot research: persist product, shop, and keyword runs locally so agents can diff niches over time without reloading the app UI.

## Table Stakes
- Product Analytics: product name/image, shop name, sales data, revenue estimates, keyword rankings, search volume, listing details, shop details, trend windows, tags, filters, currency, and CSV export. Evidence: EverBee Help "Product Analytics"; EverBee product page.
- Shop Analyzer: competitor sales data, revenue, tags, pricing and niche strategy insights, web app and Chrome extension access. Evidence: EverBee Help "Shop Analyzer".
- Keyword Research: search volume over the last 30 days, competition, keyword score, filters/custom columns, and export. Evidence: EverBee Help "Keyword Research".
- Competitors: eRank, Alura, Marmalead, EtsyHunt, Sale Samurai, Insight Agent, RankE Now/Slyst. Common table stakes are keyword volume/competition, listing audits, competitor tracking, trend forecasting, and shop analytics.

## Data Layer
- Primary entities: research_runs, products/listings, shops, keywords, tags, trend_points, exports, and snapshots.
- Sync cursor: run timestamp plus query/shop/keyword identity; browser-sniffed APIs may have pagination/cursor tokens.
- FTS/search: keyword text, product titles, shop names, tags, categories, and notes.
- Derived tables: keyword opportunity score, listing momentum, shop gap matrix, niche snapshot diffs.

## Auth
- User context: EverBee auth is currently Google login.
- Expected CLI auth model: browser-session capture / Google SSO cookie reuse, not API-key auth.
- Browser capture should validate replayability before enabling `auth login --chrome` or recommending the `press-auth` companion.

## Product Thesis
- Name: EverBee Research CLI
- Why it should exist: EverBee's app is optimized for interactive Etsy research; agents need the same product, shop, and keyword workflows in batchable, repeatable, machine-readable form with local snapshots, diffs, and SQL/search.

## Build Priorities
1. Browser-sniff the three requested app workflows and produce an internal spec from replayable traffic.
2. Generate read-oriented commands for product analytics, shop analyzer, and keyword research.
3. Add local SQLite persistence for research runs, products, shops, keywords, tags, and trends.
4. Add cross-workflow commands: niche score, shop gaps, tag gap, keyword clusters, trend diff, and opportunity shortlist.
5. Verify auth/session behavior and plan-gated errors honestly; no fake data or resident browser runtime.

## Sources
- EverBee Help: Product Analytics, https://help.everbee.io/en/article/3-product-analytics
- EverBee Help: Shop Analyzer, https://help.everbee.io/en/article/2-shop-analyzer
- EverBee Help: Keyword Research, https://help.everbee.io/en/article/4-keyword-research-suite
- EverBee Help: Welcome, https://help.everbee.io/en/article/1-welcome-to-everbee
- EverBee Product Analytics page, https://everbee.io/product-analytics
- EverBee Keyword Research page, https://everbee.io/keyword-research
