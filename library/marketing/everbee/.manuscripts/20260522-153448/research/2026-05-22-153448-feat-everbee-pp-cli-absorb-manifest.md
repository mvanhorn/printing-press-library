# EverBee Absorb Manifest

## Source Tools
- EverBee Product Analytics: product/listing metrics, tags, sales/revenue estimates, trends, filters, export.
- EverBee Shop Analyzer: competitor shop sales/revenue/tag/pricing analysis.
- EverBee Keyword Research: keyword volume, competition, score, trend/context, filters, export.
- eRank: keyword explorer, listing audit, rank checker, competitor tracking, trend/sales-map style research.
- Alura/EtsyHunt/Sale Samurai/Marmalead: product research, keyword discovery, listing/tag optimization, trend forecasting, and shop/competitor analysis.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Product analytics listing table | EverBee Product Analytics | `(generated endpoint) product_analytics list_default_product_analytics` | Agent-readable JSON and local snapshots instead of UI-only tables. |
| 2 | Keyword metrics table | EverBee Keyword Research | `(generated endpoint) keyword_research list_default_keyword_suggestion` | Repeatable keyword pulls with `--json`, `--select`, and local history. |
| 3 | Shop analyzer table | EverBee Shop Analyzer | `(generated endpoint) shops list_shops` | Competitor shop data can be searched, diffed, and joined to tags/products. |
| 4 | Saved folders / research organization | EverBee app | `(generated endpoint) folders list_folders` | Agents can enumerate research groupings without opening the UI. |
| 5 | Product/listing filters | EverBee Product Analytics | `(behavior in everbee-pp-cli product-analytics) order, page, per-page, and time-window filters` | CLI filters are scriptable and reproducible. |
| 6 | Keyword filters and sort | EverBee Keyword Research | `(behavior in everbee-pp-cli keyword-research) order, direction, page, and search-type filters` | Keyword pulls become batchable. |
| 7 | Shop result pagination and sorting | EverBee Shop Analyzer | `(behavior in everbee-pp-cli shops) order, direction, page, and per-page filters` | Competitor research can be paged and stored locally. |
| 8 | Tag inspiration from winning products | EverBee Product Analytics, eRank, Alura | `everbee-pp-cli tags gap --query candle --shop my-shop --json` | Joins high-performing tags with keyword competition and local shop coverage. |
| 9 | Listing/research export | EverBee exports, eRank exports | `(behavior in everbee-pp-cli sync) persist product, shop, keyword, and folder responses locally` | Local SQLite makes exports queryable instead of email-only CSV. |
| 10 | Competitor pricing gaps | EverBee Shop Analyzer, Alura | `everbee-pp-cli shop gaps --shop competitor-shop --json` | Highlights underpriced or underserved competitor segments from saved shop/listing data. |
| 11 | Keyword opportunity scoring | EverBee Keyword Score, eRank Keyword Explorer | `everbee-pp-cli niche score --keyword "mother's day mug" --json` | Scores opportunity using demand, competition, tags, and listing momentum in one response. |
| 12 | Trend / momentum review | EverBee trends, Marmalead trend forecasting | `everbee-pp-cli trends diff --query "teacher gift" --days 30 --json` | Diffs saved snapshots so agents can see change over time, not only current tables. |
| 13 | Competitor tracking | eRank competitor tracking, EverBee Shop Analyzer | `everbee-pp-cli competitors watch --shop competitor-shop --json` | Replays saved shop snapshots and flags changed tags, price bands, and top listings. |
| 14 | Listing audit style checks | eRank listing audit, EtsyHunt listing optimization | `everbee-pp-cli listing audit --listing-id 123456789 --json` | Audits tag coverage and keyword fit from EverBee-derived data without changing Etsy listings. |
| 15 | Keyword cluster/variation discovery | Marmalead Storm, Sale Samurai keyword tools | `everbee-pp-cli keywords cluster --seed "wedding sign" --json` | Groups related keywords by demand/competition and tag overlap from saved pulls. |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | Opportunity shortlist | `opportunity shortlist --query "teacher gift" --limit 25 --json` | hand-code | Requires joining product analytics, keyword metrics, tags, and local history into one ranked list. |
| 2 | Niche score | `niche score --keyword "mother's day mug" --json` | hand-code | Combines demand, competition, product saturation, pricing, and trend deltas across multiple EverBee workflows. |
| 3 | Shop gaps | `shop gaps --shop competitor-shop --json` | hand-code | Uses competitor shop data plus keyword/tag history to identify missing products and pricing openings. |
| 4 | Tag gap | `tags gap --query candle --shop my-shop --json` | hand-code | Compares tags used by winning listings against a target shop's known keyword/tag footprint. |
| 5 | Keyword clusters | `keywords cluster --seed "wedding sign" --json` | hand-code | Groups keyword suggestions by shared terms, score, and competition using local FTS/SQLite. |
| 6 | Trend diff | `trends diff --query "teacher gift" --days 30 --json` | hand-code | Requires saved snapshots over time; the EverBee UI exposes current/trend views, not agent-ready diffs. |
| 7 | Competitor watch | `competitors watch --shop competitor-shop --json` | hand-code | Turns repeated shop analyzer snapshots into change detection for products, prices, and tags. |
| 8 | Listing audit | `listing audit --listing-id 123456789 --json` | hand-code | Applies keyword/tag/product research context to a listing without requiring a separate Etsy listing tool. |

## Stubs
- None. If an endpoint proves plan-gated or non-replayable during build/dogfood, return to this gate for revised approval instead of shipping a hidden stub.

## Risk Notes
- The captured spec has only five replayable endpoints. It covers the requested workflow roots but not every UI action EverBee markets, so generation should avoid claiming unsupported create/update/export-email behavior unless captured later.
- Auth is `x-access-token` from a Google-login browser session. The CLI should document `EVERBEE_ACCESS_TOKEN` until a durable browser-login helper is proven.
- Revenue/sales/search-volume figures are estimates from EverBee and should be presented as research signals, not exact Etsy truth.
