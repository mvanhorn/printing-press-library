# Home GOAT CLI Brief

## API Identity
- Domain: Home furnishing product aggregation — multi-source search across 10+ retailers covering foundational fixtures, appliances, furniture, and decor
- Users: Homeowners doing renovations, interior designers, contractors, anyone comparison-shopping home furnishings across retailers
- Data profile: Product catalogs (titles, brands, prices, ratings, images, availability), product categories, retailer-specific inventory. Read-only aggregation — no cart/checkout/account operations.

## Reachability Risk
- **Medium-High** — mixed fleet:
  - Ferguson GraphQL: **Confirmed reachable** via anonymous JWT + custom headers. Proven in-session with live product search returning 174KB of results.
  - Article APQ/GraphQL: **Confirmed reachable** via unauthenticated persisted queries. 14 operations discovered in-session.
  - Shopify Storefront API (8+ stores): **Confirmed reachable** via public storefront access tokens. Schoolhouse product detail captured in-session.
  - Wayfair: **Blocked** — aggressive anti-bot on both WebFetch and Playwright. No direct API access. May need clearance-cookie approach.
  - AllModern: **Blocked** — same Wayfair infrastructure, same bot protection.
  - RH (Restoration Hardware): **Partially reachable** — public product pages work, no API reverse-engineered.
  - West Elm: **Partially reachable** — public pages, WSI family. Bath products redirect to Ferguson.
  - Rejuvenation: **Partially reachable** — public pages, WSI family. Bath products redirect to Ferguson.
  - IKEA: **Deferred** — user requested for later integration. Reverse-engineered APIs exist in community.
  - Dupe.com: **Deferred** — AI visual-similarity matching, later phase.

## Top Workflows
1. **Cross-retailer product search** — "find me a 24-inch wall-mounted floating vanity with sink" → fan out to Ferguson, Shopify DTC stores, Article, compare results side-by-side with normalized pricing
2. **Category-scoped comparison** — search only foundational (GC-installed) items, only furniture, only decor — routes to relevant sources automatically
3. **Price comparison / deal finding** — same product or equivalent across retailers, normalized price ranges, sale detection
4. **Product detail enrichment** — drill into a specific product: full specs, reviews, delivery estimates, related items from the source retailer
5. **Inventory/availability check** — which retailers have a specific category or product type in stock

## Table Stakes
- Multi-source fan-out with partial-failure tolerance (some sources may be down)
- Normalized product schema across all sources (title, brand, price range, rating, URL, image, source)
- Category-based routing (foundational → Ferguson/Wayfair/Rejuvenation/RH, furniture → West Elm/Article/RH/Shopify DTC/AllModern, etc.)
- Price range normalization (different retailers use different price structures)
- JSON + table output formats
- `--source` flag to restrict to specific retailers
- `--category` flag to restrict to product categories
- Local SQLite store for search history, saved products, price tracking
- `--json` and `--table` output modes
- Offline search of previously fetched results

## Data Layer
- Primary entities: Product (normalized), SearchQuery, PriceSnapshot, SavedProduct
- Sync cursor: per-source last-search timestamp for cache invalidation
- FTS/search: full-text search over cached product titles, brands, descriptions
- Price history: track price changes over time for saved products

## Source Priority
- **Ordering:** peers (no single primary — category-based routing)
- **Routing model:** product categories route to relevant sources
  - Foundational (GC/specialist-installed): Ferguson, Wayfair, Rejuvenation, RH
  - Appliances: Ferguson, Wayfair, AllModern
  - Furniture: West Elm, Article, RH, Shopify DTC, AllModern
  - Decor: West Elm, Rejuvenation, Shopify DTC
- **Economics:** All sources are free (no paid API keys). Ferguson uses auto-issued anonymous JWTs. Shopify stores use public storefront access tokens embedded in page source. Article uses unauthenticated APQ queries.
- **Inversion risk:** Ferguson has the most complete discovered API (GraphQL with full product search), but it is NOT the primary — it's one peer among many, specialized for foundational/fixture categories. Do not let Ferguson's API completeness make it the de facto primary.

## Discovered API Details

### Ferguson (foundational, appliances)
- **Endpoint:** `POST https://www.fergusonhome.com/graphql/{operationName}`
- **Auth:** `Authorization: Bearer <window.__AUTHTOKEN__>` (anonymous JWT auto-issued)
- **Headers:** `x-fergy-client-name: react-build-store`, `Content-Type: application/json`
- **Proven query:**
  ```graphql
  query ProductSearch($input: ProductSearchInput!) {
    productSearch(input: $input) {
      ... on ProductSearchResult {
        count
        products {
          url title brandName
          priceInfo { range { min max } }
          rating { reviewCount ratingValue }
        }
      }
      ... on SearchRedirect { url }
    }
  }
  ```
- **Variables:** `{ "input": { "query": "...", "offset": 0, "limit": 24 } }`
- **Gotchas:** `ProductPrice` has `range { min max }` and `unitPrice`, NOT `value`/`displayPrice`/`regularPrice`/`salePrice`. `SearchProduct` has `url`, `title`, `brandName` but NOT `brand`/`description`/`reviews`/`category`. `Rating` has `reviewCount` and `ratingValue` only.
- **Response types:** `ProductSearchResult` (with count + products array) OR `SearchRedirect` (URL redirect for category matches). Must handle both via `... on` fragments.
- **Reachability:** Confirmed reachable. 174KB response captured in-session.

### Article (furniture, decor)
- **Endpoint:** `GET https://www.article.com/graphql`
- **Auth:** None (unauthenticated)
- **Protocol:** Apollo Persisted Queries (APQ) — uses sha256 hashes instead of full query text
- **Discovered operations (14):**
  - SEARCH_PRODUCTS, PRODUCT (50+ fields), productDeliveryEstimate, CROSS_SELL, SIMILAR_PRODUCTS, PRODUCT_REVIEWS_ANALYTICS, getProductReviewsByProductId, getProductReviewsUGCMedia, PRODUCT_CUSTOMIZATION_OPTIONS, ADD_TO_CART, REMOVE_FROM_CART, UPDATE_CART_ITEM, GET_CART, ADD_TO_WISHLIST
- **Search variables:** `{ "query": "...", "pageSize": 24, "page": 1 }`
- **Reachability:** Confirmed. Unauthenticated access.

### Shopify Storefront API (furniture, decor — 8+ stores)
- **Endpoint:** `POST https://{store}.myshopify.com/api/2025-01/graphql.json`
- **Auth:** `X-Shopify-Storefront-Access-Token: <public_token>` (per-store, embedded in page source)
- **Confirmed stores + tokens:**
  - Schoolhouse: `schoolhouseelectric` / `6b9644bb298124bc9ade899eaddea363`
  - Blu Dot: `2ddf06-7b` (domain TBD)
  - Gus Modern: `gus-design-group` (token TBD)
  - Floyd: `floyd-home` (token TBD)
  - Burrow: `burrow-prod` (token TBD)
  - Lulu & Georgia: `lulu-and-georgia` (token TBD)
  - Serena & Lily: `serenaandlily` (token TBD — DataDome CAPTCHA on web, Storefront API may work)
  - Arhaus: `arhaus` (token TBD)
- **Standard query shape:** `products(first: N, query: "...")` with `title, handle, description, priceRange { minVariantPrice { amount currencyCode } maxVariantPrice { amount currencyCode } }, images(first: 1) { edges { node { url } } }, vendor`
- **Reachability:** Confirmed for Schoolhouse. One integration covers all Shopify stores.

### Wayfair (foundational, appliances, furniture)
- **Reachability:** BLOCKED — aggressive anti-bot on direct HTTP and Playwright
- **Fallback:** Browser-sniff with clearance cookie may work. Deferred to Phase 1.7 gate.
- **Products confirmed:** via web search results (product data exists, just bot-protected)

### AllModern (appliances, furniture)
- **Reachability:** BLOCKED — same Wayfair infrastructure
- **Fallback:** Same as Wayfair. Deferred to Phase 1.7.

### RH / Restoration Hardware (foundational, furniture)
- **Reachability:** Public product pages accessible, no API reverse-engineered
- **Approach:** HTML scraping or browser-sniff to discover internal API
- **Sub-brands:** RH, RH Modern, RH Baby & Child, RH Outdoor, RH Beach House, RH Ski House, RH Contemporary

### West Elm (furniture, decor)
- **Reachability:** Public pages accessible, WSI family
- **Note:** Bath product pages redirect to Ferguson — Ferguson IS the WSI bath aggregation endpoint
- **Approach:** HTML scraping or browser-sniff

### Rejuvenation (foundational, decor)
- **Reachability:** Public pages accessible, WSI family
- **Note:** Same WSI bath → Ferguson redirect pattern
- **Approach:** HTML scraping or browser-sniff

### IKEA (deferred)
- **Status:** User explicitly requested for later integration
- **Prior art:** Community reverse-engineered APIs exist (ikea-api npm package, etc.)
- **Plan:** Wire in after core sources are stable

### Dupe.com (deferred)
- **Status:** AI visual-similarity matching for cross-source lookalikes
- **Plan:** Wire in as a transcendence-style compound query after core search works

## Product Thesis
- Name: **reno-goat** (Home Greatest Of All Time)
- Why it should exist: No single CLI or tool aggregates product search across all major home furnishing retailers. Users currently open 5-10 browser tabs to comparison-shop. Home-goat fans out a single search query to 10+ sources and returns normalized results in one view. The category-based routing means users search by intent ("I need a vanity") not by retailer ("search Ferguson, then search Article, then search West Elm").

## Build Priorities
1. **Ferguson integration** — most complete discovered API, proven GraphQL schema, covers the foundational category that most renovators start with
2. **Shopify DTC integration** — one client covers 8+ stores, immediate breadth for furniture/decor categories
3. **Article integration** — APQ-based, unique inventory (modern furniture), confirmed reachable
4. **Category-based search routing** — the organizing principle: `search --category foundational "floating vanity"` routes to Ferguson+Wayfair+Rejuvenation+RH
5. **Normalized product schema + local SQLite store** — consistent output across all sources, offline search, price tracking
6. **Wayfair/AllModern** — browser-sniff gate will determine if these are viable for v1 or deferred
7. **RH/West Elm/Rejuvenation HTML scraping** — fallback for sources without discovered APIs
8. **IKEA integration** — Phase 2 addition per user request
9. **Dupe.com cross-source visual matching** — transcendence query, later phase
