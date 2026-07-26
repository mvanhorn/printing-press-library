# Flipkart CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Keyword search with pagination | atharao/flipkart-scraper, Sunil-nith/Web_Scraping_Flipkart_Product_Data | `flipkart search "<query>" --page N --limit N` | Offline re-search via FTS5, `--json`/`--csv`, `--select` field filtering |
| 2 | Product detail fetch by URL/ID | mdvenukumar/flipkart-scraper, dvishal485/flipkart-scraper (Rust) | `flipkart product get <url-or-id>` | Persists every fetch to SQLite, building price history automatically |
| 3 | Price/MRP/discount%/rating/review-count extraction | all community scrapers | Typed fields on every product record | Consistent schema across search + detail + affiliate-API modes |
| 4 | CSV export | atharao/flipkart-scraper | `--csv` on every list command | Works on any command, not just search |
| 5 | Stock/COD availability flags | mdvenukumar, Riyad654 scrapers | Typed boolean fields | Queryable via local SQL |
| 6 | Official Affiliate API mode (category feed, delta feed, search by keyword/ID) | Flipkart Affiliate API docs, hi-imcodeman/flipkart-scraper, flipkart-affiliate-client (npm) | `flipkart feed category <cat>`, `flipkart feed delta <cat> --from-version N` | Only tool absorbing both the scraped surface AND the official API behind one schema |
| 7 | Bank/card offer text extraction on product pages | Bank Offers Aggregator (Apify MCP) | `flipkart product offers <url-or-id>` | Structured offer rows (bank, card, discount) persisted for comparison |
| 8 | Price drop alerting (single product) | muhammedashharps/Flipkart-Price-Tracker, Ryuk-me/Flipkart-Price-Tracker, nuhmanpk/PriceTrackerBot | `flipkart watch add <url> --threshold <price>`, `flipkart watch check` | Local SQLite tracks many products + full historical series, not one script per product |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| 1 | Watchlist digest | `flipkart watch digest` | 10/10 | hand-code | Joins the local watchlist against price_history for a "what changed since I last checked" summary — no server-side watchlist exists on Flipkart |
| 2 | Multi-product compare | `flipkart compare <url1> <url2> [url3...]` | 8/10 | hand-code | Renders a unified diff table across price/rating/discount/specs for N products — no single Flipkart call does this |
| 3 | Feed digest with auto cursor | `flipkart feed digest <category>` | 8/10 | hand-code | Auto-resolves the Delta Feed API's `fromVersion` from a locally persisted cursor, then ranks deltas by discount-% change |
| 4 | Saved-search diff | `flipkart search diff "<query>"` | 7/10 | hand-code | Diffs two timestamped snapshots of the same query in `search_results`, surfacing new/removed/price-changed products |
| 5 | Category deal scanner | `flipkart deals category <cat> --min-discount N` | 7/10 | hand-code | Filters a category snapshot by discount threshold and persists matches so the scan is offline-queryable later |
| 6 | Best-card arbitrage | `flipkart offers best-card <url1> <url2>...` | 7/10 | hand-code | Aggregates the local offers table across a product set to find the single card maximizing total stacked savings |

Minimum 5 transcendence features: met (6 survivors, all scoring >= 7/10).

## Killed candidates (audit trail, not shipping)
| Feature | Kill reason |
|---|---|
| Biggest price drops across everything ever fetched | Redundant vs. watchlist digest, weaker signal-to-noise |
| Similar/frequently-compared product extraction | No evidence this HTML block is reliably present |
| Brand-store product pull | Thin rename of `search --brand=X` |
| Seller price/rating comparison | No workflow evidence for multiple comparable sellers per product |
| Big Billion Days/sale-event tracker | Scope creep, no stable endpoint |
| In-category discount/rating rank | Fragile precondition, thin output |

## Scope Note
Personal order/account history is explicitly OUT OF SCOPE per user decision — no Flipkart API surface (official or scraped) exposes it without a full authenticated personal-account session, which the user declined to build.
