# Browser-Sniff Discovery Report — reno-goat

## Run ID: 20260525-182759
## Date: 2026-05-25

## Sources Discovered

### West Elm (NEW — via browser-sniff)
- **API:** Constructor.io Search API
- **Endpoint:** `GET https://ac.cnstrc.com/search/{query}`
- **Auth:** API key in query param: `key=key_SQBuGmXjiXmP0UNI`
- **Client ID:** `ciojs-client-2.77.1`
- **Transport:** `standard_http` (no bot protection on Constructor.io)
- **Pagination:** `offset` + `num_results_per_page` (default 20)
- **Sort options:** relevance, lowestPrice (asc/desc), highestPrice (asc/desc), name (A-Z/Z-A), newcore
- **Facets available:** Product Type, Material, Width, Shop by Room, Vanity Type, Category, Collection, Wood Finish, Color, Features
- **Product data fields:** title, url, image_url, description, lowestPrice, highestPrice, regularPriceMin, regularPriceMax, salePriceMin, salePriceMax, id, productBlurb, prodinfo, maxDiscountPercent, productPriceType, altImages, hoverImages, swatchesDisplay, flags, eligibleForQuickBuy
- **Additional endpoints discovered:**
  - `GET /search/stores.json?brands=WE&lat=...&lng=...&radius=...` — store locator
  - `GET /api/inventory/v1/transit/route.json?postalCode=...` — delivery routes
  - `GET /api/catalog/v1/attributes/images.json?attributes=color` — color swatches
  - `GET /promotion/eligibility/group/{product-slug}/eligiblePromotions.json?price=...` — promo eligibility
  - `GET /api/content/v1/content-blocks.json` — content/promo blocks
  - `GET /cacheable/availability/getDeliveryInformationByPostalCode.json?postalCode=...` — delivery info
- **Autocomplete:** `GET https://ac.cnstrc.com/autocomplete/{query}?key=key_SQBuGmXjiXmP0UNI&num_results_Products=0&num_results_Search+Suggestions=5`

### Rejuvenation (NEW — via browser-sniff)
- **API:** Constructor.io Search API (same provider as West Elm — WSI family)
- **Endpoint:** `GET https://ac.cnstrc.com/search/{query}`
- **Auth:** API key in query param: `key=key_9BhS51IOFNhJejk4`
- **Client ID:** `ciojs-client-2.77.1`
- **Transport:** `standard_http`
- **Same query shape as West Elm** — only the API key differs
- **Additional endpoints discovered:**
  - `GET /search/stores.json?brands=RJ&lat=...&lng=...&radius=...` — store locator
  - `GET /api/catalog/v1/category/inspiration/shop/{category}/data.json` — category catalog
  - `GET /api/inventory/v1/transit/route.json?postalCode=...` — delivery routes
- **Autocomplete:** `GET https://ac.cnstrc.com/autocomplete/{query}?key=key_9BhS51IOFNhJejk4`

### Wayfair (PARTIAL — via browser-sniff)
- **API:** Federated GraphQL
- **Endpoint:** `POST https://www.wayfair.com/federation/graphql`
- **Transport:** `browser_clearance_http` — PerimeterX protection
- **Discovered operations:** HeaderExperienceGetCartQuantity, SearchBlankState, SearchBlankStateCoreListingCollection, GlobalHelpFeatureTogglesQuery, GlobalHelpPageConfiguration, BlockBuilderFooterExperienceQuery
- **Status:** Page loaded but actual search results didn't render — PerimeterX likely detected automated browsing. Some GraphQL requests returned 429.
- **For v1:** Defer to clearance-cookie approach or integrate later.

### AllModern (DEFERRED)
- Same Wayfair infrastructure (both brands owned by Wayfair Inc.)
- Will share the same integration approach once Wayfair is solved.

### RH / Restoration Hardware (BLOCKED)
- DataDome CAPTCHA wall on page load via Playwright
- Could not discover any API endpoints
- Would require user to manually solve CAPTCHA challenge
- **For v1:** Defer to later phase.

## Architecture Insight: Constructor.io as WSI Family Search Layer

Both West Elm and Rejuvenation use Constructor.io (cnstrc.com) as their search infrastructure. This is a SaaS search service — the API is well-documented, stable, and uses simple API keys in query parameters. The same Go client can serve both stores by swapping the API key.

Constructor.io also provides:
- Autocomplete/typeahead suggestions
- Behavioral tracking (session_start, search_result_load)
- Pre-filter expressions for store/fulfillment filtering
- Hidden facets support

This is a high-value discovery — one Constructor.io integration gives us West Elm + Rejuvenation with identical code paths, and the API is inherently more stable than reverse-engineered GraphQL endpoints.

## v1 Source Tier Summary

### Tier 1 — API fully discovered, ready to generate
1. Ferguson (GraphQL, anonymous JWT)
2. West Elm (Constructor.io, public key)
3. Rejuvenation (Constructor.io, public key)
4. Article (APQ GraphQL, unauthenticated)
5. Shopify DTC — 8+ stores (Storefront API, public tokens)

### Tier 2 — API partially discovered, deferred
6. Wayfair (federated GraphQL, needs clearance cookies)
7. AllModern (same as Wayfair)

### Tier 3 — Blocked, deferred
8. RH (DataDome CAPTCHA)
9. IKEA (user-deferred)
10. Dupe.com (user-deferred)
