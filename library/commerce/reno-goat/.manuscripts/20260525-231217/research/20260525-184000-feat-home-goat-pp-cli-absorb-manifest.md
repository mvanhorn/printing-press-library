# Home GOAT CLI — Absorb Manifest

## Absorb Sources

No direct CLI competitors exist in the home furnishing aggregation space. Features absorbed from:
- Retailer websites (Ferguson, West Elm, Rejuvenation, Article, Wayfair, RH)
- Shopify Storefront API capabilities
- Constructor.io search API capabilities
- Existing goat CLIs in the printing-press library (flight-goat, recipe-goat, etc.)
- Home shopping comparison sites (Google Shopping, PriceGrabber)

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Product search by keyword | All retailer sites | `search "floating vanity"` | Fans out to ALL sources in parallel, returns normalized results |
| 2 | Category filtering | Retailer site navs | `search --category foundational "faucet"` | Routes to relevant sources only based on product category |
| 3 | Source filtering | N/A (each site is siloed) | `search --source ferguson,west-elm "vanity"` | Pick specific retailers to query |
| 4 | Price range filtering | Retailer site filters | `search --min-price 500 --max-price 2000 "vanity"` | Cross-retailer price filtering with normalized ranges |
| 5 | Sort by price/relevance | Retailer site sorts | `search --sort price-asc "vanity"` | Unified sort across all sources |
| 6 | Product detail drill-down | Retailer product pages | `product <id-or-url>` | Full specs, images, reviews from source |
| 7 | Faceted search | Constructor.io, Ferguson | `search --facet "Material=Wood" "vanity"` | Facets aggregated across sources |
| 8 | Autocomplete / typeahead | Constructor.io, Ferguson | `suggest "float"` | Multi-source autocomplete suggestions |
| 9 | Price comparison | Google Shopping | `compare <product-url> [product-url...]` | Same/similar products across retailers, side-by-side |
| 10 | Store locator | West Elm, Rejuvenation | `stores --near "zip" --brand west-elm` | Find physical stores near a location |
| 11 | Delivery availability | West Elm, Ferguson | `delivery --zip 20001 <product-url>` | Check delivery options by postal code |
| 12 | Promotion/sale detection | West Elm promo API | `deals --category furniture` | Active promotions across retailers |
| 13 | Product reviews/ratings | Ferguson, Article | `reviews <product-url>` | Reviews from source retailer |
| 14 | Similar products | Article SIMILAR_PRODUCTS | `similar <product-url>` | Find similar items at the same retailer |
| 15 | Cross-sell suggestions | Article CROSS_SELL | `related <product-url>` | Related/complementary products |
| 16 | Brand browsing | All retailers | `brands --source ferguson` | List available brands at a retailer |
| 17 | Category browsing | Rejuvenation catalog API | `categories --source rejuvenation` | Browse product category tree |
| 18 | Image gallery | All retailers | `product --images <url>` | Product images with swatch/variant views |
| 19 | JSON output | Printing-press standard | `search --json "vanity"` | Machine-readable output for agent consumption |
| 20 | Table output | Printing-press standard | `search --table "vanity"` | Tabular output for terminal comparison |
| 21 | Offline search | SQLite local store | `search --offline "vanity"` | FTS5 search over previously cached results |
| 22 | Search history | SQLite local store | `history` | Past searches with timestamps and result counts |
| 23 | Saved products | SQLite local store | `save <product-url>` / `saved` | Bookmark products for later |
| 24 | Pagination | All retailer APIs | `search --page 2 --per-page 20 "vanity"` | Navigate through result pages |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| T1 | Price Watch | `watch <product-url> [--threshold 10%]` / `watches` | SQLite price history + cron-friendly poll. No single retailer shows cross-retailer price trends; we store every snapshot and alert on drops. |
| T2 | Project Tracker | `project create "Kitchen Reno"` / `project add <product-url>` / `project budget` | Group saved products into named projects with running budget totals. No retailer tracks cross-store project budgets; we aggregate spend across Ferguson, West Elm, Article, etc. |
| T3 | Stale Detector | `saved --check-stale` / `project --check-stale` | Re-fetches saved/project products and flags discontinued, out-of-stock, or price-changed items. No retailer alerts you about products saved at *other* retailers. |
| T4 | Spec Sheet Export | `product --spec-sheet <url> [--format markdown\|csv]` | Extracts and normalizes product dimensions, materials, finishes, certifications into a structured spec sheet. Individual retailers bury specs in unstructured HTML; we normalize across sources. |
| T5 | Brand Cross-Reference | `brands --cross-ref "Kohler"` / `search --brand "Kohler" --all-sources` | Shows which retailers carry a given brand and at what price points. No single retailer shows you where else a brand is sold or who has the best price. |
| T6 | Compound Category Search | `search --room bathroom "vanity"` / `search --category foundational,furniture "vanity"` | Multi-category fan-out with room-type shortcuts (bathroom → foundational+furniture+decor, kitchen → foundational+appliances+decor). No single retailer routes across product types; we chain searches to the sources that carry each category. |
| T7 | Review Aggregation | `reviews --aggregate <url>` | Cross-source review merging: finds the same product at other retailers by title/brand/model, merges reviews from all sources. Also pulls from external review sites for broader coverage. No retailer aggregates reviews from competing stores. |

## Stubs (deferred implementations)

| # | Feature | Command | Status | Reason |
|---|---------|---------|--------|--------|
| S1 | Wayfair search | `search --source wayfair` | (stub) | Requires clearance cookies; bot-protected |
| S2 | AllModern search | `search --source allmodern` | (stub) | Same Wayfair infrastructure; deferred |
| S3 | RH search | `search --source rh` | (stub) | DataDome CAPTCHA; deferred |
| S4 | IKEA search | `search --source ikea` | (stub) | User-deferred to later phase |
| S5 | Dupe visual matching | `dupe <product-url>` | (stub) | Deferred to later phase; needs Dupe.com API |
