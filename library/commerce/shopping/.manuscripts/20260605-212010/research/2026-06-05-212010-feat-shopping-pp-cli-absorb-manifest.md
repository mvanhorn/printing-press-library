# Shopping CLI — Absorb Manifest (LemmeBuyIt API)

LemmeBuyIt is a proprietary aggregated-retail data API. No competing open-source CLI/MCP wraps it (it is a paid data product), so the "absorb" set is the API's own surface; transcendence comes from the local store + compound queries.

## Absorbed (every endpoint as a typed command — generator-emitted)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | API health check | LemmeBuyIt /status | (generated endpoint) status get | --json, typed exit codes |
| 2 | List retailers | LemmeBuyIt /retailers | (generated endpoint) retailers list | offline reuse after sync |
| 3 | List categories for a retailer | /retailers/{id}/categories | (generated endpoint) retailers categories | --select, --json |
| 4 | Search/filter products (paid) | /retailers/{id}/products | (generated endpoint) retailers products | every filter as a flag, cursor paging |
| 5 | Search shopping products (free) | /retailers/{id}/shopping/products | (generated endpoint) retailers shopping products | free-tier surface, --json |
| 6 | Get product by SKU (paid) | /retailers/{id}/products/{sku} | (generated endpoint) retailers products get | --select |
| 7 | Get shopping product by SKU (free) | /retailers/{id}/shopping/products/{sku} | (generated endpoint) retailers shopping products get | --select |
| 8 | Weekly price history (product) | /retailers/{id}/products/{sku}/price-history | (generated endpoint) retailers products price-history | chart-ready --json |
| 9 | Weekly Amazon price history (ASIN) | /amazon/asins/{asin}/price-history | (generated endpoint) amazon asins price-history | --weeks, --json |

## Transcendence (only possible with our local store + cross-entity joins) — all hand-code, all read-only
| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|--------------------------|
| 1 | Retailer & product sync | sync | hand-code | 9 | Turns N single-retailer calls into one joinable local dataset |
| 2 | Cross-retailer identifier compare | compare | hand-code | 9 | Live API is per-retailer; only the store joins one UPC across 70 storefronts |
| 3 | Compound deal finder | deals | hand-code | 8 | Joins discount+price+rating+stock filters across retailers in one local pass |
| 4 | Weekly price-drop ranking | price-drops | hand-code | 8 | Ranks weekly deltas across all synced products; API returns one product's history at a time |
| 5 | FBA arbitrage margin | arbitrage | hand-code | 9 | Joins retailer buy price to Amazon fee/profitability blocks on the shared ASIN |
| 6 | Price watch tracker | watch | hand-code | 7 | "Has it changed since last look" needs retained local state the stateless API lacks |
| 7 | Retailer/category discount leaderboard | leaderboard | hand-code | 7 | Cross-retailer group-by aggregation the per-retailer API never computes |

Stub list: none. All transcendence rows are shipping scope.
