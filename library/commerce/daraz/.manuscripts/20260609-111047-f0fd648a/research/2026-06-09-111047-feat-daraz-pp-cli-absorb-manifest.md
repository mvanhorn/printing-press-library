# Daraz CLI — Absorb Manifest

Single source: daraz.pk website (replayable JSON + SSR HTML, `standard_http`). CLI name: `daraz-pp-cli`. Display: **Daraz.pk**. Category: commerce.

## Absorbed (match or beat every existing Daraz/Lazada tool)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | Keyword product search | oxylabs/lazada-scraper, daraz scrapers | `(generated endpoint) products search` | Offline store, `--json`/`--csv`/`--select`, typed exit codes, bounded scan |
| 2 | Sort results | all scrapers | `(behavior in daraz-pp-cli products search)` `--sort price-asc\|price-desc\|popularity\|newest\|top-sales\|rating` | Composable, documented enum |
| 3 | Filter by category/brand/price/rating/location/delivery | catalog `mods.filter` facets | `(behavior in daraz-pp-cli products search)` `--category --brand --min-price --max-price --rating --location --free-shipping` | Exposes 15 site facets as typed flags |
| 4 | Pagination + total count | scrapers | `(behavior in daraz-pp-cli products search)` `--page --limit --max-scan-pages` | Reports `totalResults`; scan cap separate from output limit |
| 5 | Product detail (price, SKUs/variations, specs, description, seller, images) | oxylabs detail | `daraz-pp-cli products get` | `__moduleData__` island extraction, `--json` |
| 6 | Reviews + rating distribution | oxylabs reviews | `(generated endpoint) reviews list` | `--rating`, `--with-images`, `--sort`, distribution summary |
| 7 | CSV / JSON / field-select output | csv scrapers | `(behavior in daraz-pp-cli products search)` `--json --csv --select` | Agent-native, pipe-clean |
| 8 | Seller's product catalog | (manual on site) | `daraz-pp-cli seller products` | Scan a seller's listings by `sellerId` |

## Transcendence (only possible with our local-store + agent-native approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Price history & drop tracking | `watch <item>` / `price-history <item>` | hand-code | Requires persisting `price_snapshots(itemId, ts, price)` locally over time; the site shows only the current price and no scraper keeps history | Use to record a product's price across runs and see the real trend / lowest-ever. Not for one-off current price — use `products get`. |
| 2 | Real-deal ranking | `deals <query>` | hand-code | Ranks a query's results by a composite of discount % × rating × sold-count, computed by local join across all scanned items — surfaces genuinely good deals, not nominal "% off" | Use to find the best-value items for a search. Do NOT use for raw listing order — use `products search --sort`. |
| 3 | Saved-search change alerts | `since <saved-search>` / `alerts` | hand-code | Diffs the current search against the last local snapshot to report NEW listings and price moves since you last checked; needs stored prior snapshot | Use to see what changed in a tracked search. For a fresh listing, use `products search`. |
| 4 | Cross-seller compare | `compare <query>` | hand-code | Local join of the same query across sellers → cheapest + best-rated side by side; no single API call returns this | Use to pick the best seller/listing for an item. Not for a single known product — use `products get`. |
| 5 | Seller scorecard | `seller stats <sellerId>` | hand-code | Aggregates a seller's catalog from the local store: avg rating, price range, listing count, discount pattern — to vet a seller before buying | Use to evaluate a seller. For their raw listings, use `seller products`. |
| 6 | Value / fake-discount detector | `value <query>` | hand-code | Flags inflated "original price" discounts by comparing each item's `originalPrice` to the local median market price for the query; rating-weighted value score | Use to detect misleading discounts and rank true value. Not a substitute for `deals` (which ranks good deals); `value` exposes bad ones. |

Minimum 5 transcendence features satisfied (6).

## Stubs
None. Every row above is shipping scope.

## Build notes
- Standard HTTP transport (`standard_http`); no auth, no browser, no cookies.
- Local SQLite store: `products`, `sellers`, `reviews`, `price_snapshots`, `saved_searches`.
- Hand-code count (Phase 3): 6 transcendence + `products get` + `seller products` = 8 hand-built commands; `products search` and `reviews list` are generated typed endpoints.
- Currency: PKR (`mainInfo.currency`).
