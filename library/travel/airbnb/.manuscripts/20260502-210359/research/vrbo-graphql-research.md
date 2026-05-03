# VRBO GraphQL Endpoint Shape — Research

Scope: Phase 1.5a research, paired with user-provided HAR for confirmation.

## Endpoint
`POST https://www.vrbo.com/graphql` (single endpoint, no version prefix).

## Auth
NONE for public ops (search, listing). Akamai-gated: `_abck`, `ak_bmsc`, `bm_sv` cookies required from a homepage warmup.

## Required headers
- `Content-Type: application/json`
- `User-Agent`: full Chrome UA, must match TLS JA3 fingerprint (Python `requests` blocked instantly; use `curl_cffi` in Python, **Surf library with Chrome impersonation in Go** — printing-press already supports this)
- `Sec-Fetch-Site`, `Sec-Fetch-Mode`, `Sec-Fetch-Dest` (missing = block trigger)
- `Origin: https://www.vrbo.com`
- `Referer`: realistic page URL
- `Accept-Language: en-US,en;q=0.9`
- `client-info` (Expedia-family header, exact format from HAR)

## Body shape
```json
{
  "operationName": "<Op>",
  "variables": { ... },
  "query": "...",
  "extensions": { "persistedQuery": { "version": 1, "sha256Hash": "..." } }
}
```

APQ enabled. If hash unknown, server falls back to `query` string. **HAR is authoritative for the persisted-query hashes.**

## Operations (from research; HAR will confirm names/hashes)

| Operation | Variables | Response root | High-gravity fields |
|---|---|---|---|
| `propertySearch` | `propertySearchCriteria.dates.{checkIn,checkOut}`, `rooms[].{adults,children}`, `geography.lbsId`, `paging.{startingIndex,pageSize}`, `filterCriteria` | `data.propertySearch.propertySearchListings[]` | id, name, avgRatingValue, reviewCount, priceSection, mediaSection.gallery.media, geo, listingKey |
| (likely `LodgingPropertyDetail` or `PropertyDetailPage`) | `propertyId` (or `listingId`), `checkIn`, `checkOut`, `adults`, `children` | `data.propertyDetail` or `data.property` | headline, description, amenities, priceSummary, fees, host, reviewDetails, location, rules |
| autosuggest (name unconfirmed) | `query`, `locale`, `siteId` | `data.suggestions[]` | term, regionId, type, latLong |
| availability/calendar (name unconfirmed) | `listingId`, `year`, `month` | `data.availability` | availabilityDays[], minNights, pricePerNight, blockedDates |

## URL patterns (for SSR fallback)
- Search: `https://www.vrbo.com/search/keywords:{slug}/arrival:{YYYY-MM-DD}/departure:{YYYY-MM-DD}/adults:{n}`
- Search alt: `https://www.vrbo.com/vacation-rentals/{region-slug}?adults=2&startDate=...&endDate=...`
- Property detail: `https://www.vrbo.com/{numericPropertyId}`
- SSR HTML embeds `__PLUGIN_STATE__` JSON in `<script>` tag (cheap entry point for name/location/thumbnail; pricing requires GraphQL).

## Akamai bypass (warmup pattern)
1. `GET https://www.vrbo.com/` → triggers sensor JS, sets `_abck` + `ak_bmsc` (HTTP-only)
2. Wait 1.2-2.5s
3. Proceed to search URL or GraphQL POST with all cookies forwarded

Rate limit: ~1 request per 2-3s safe; faster triggers 429s without IP rotation recovery. Use `cliutil.AdaptiveLimiter` per-source.

## Killer-feature support (host visibility)

For host-direct arbitrage, extract host identity from property detail. Reliability ranking:

1. `data.propertyDetail.propertyManagement.name` — PMC brand (Vacasa, Evolve, RedAwning, Turnkey, independent brand). HIGHEST signal — these always have direct booking sites.
2. `data.propertyDetail.host.displayName` or `host.name` — individual host name. ~60% findable via web search + city.
3. `data.propertyDetail.host.aboutMe` / `host.brandName` — less consistent, sometimes carries the brand.
4. `data.propertyDetail.unitId` — VRBO internal ID, not useful for finding direct.

Cross-reference tactic: `"<host or PMC name>" "<city>" site:<pmc-domain>` for big PMCs.

## Fee breakdown (for compare command)

Search results: `data.propertySearch.propertySearchListings[].priceSection`:
- `perNightPrice`, `totalPrice`
- `fees[].{feeType, amount}` (CLEANING_FEE, SERVICE_FEE, TAX)

Property detail: `data.propertyDetail.priceSummary.fees[]`:
- `label` (string: "Cleaning fee", "Service fee", "Taxes and fees")
- `amount`, `currency`

VRBO traveler-side service fee: 6-12% of subtotal. Extract SERVICE_FEE for the platform-fee delta vs direct booking.

## Risks / open questions
1. **APQ hash dependency:** if VRBO enforces persisted-query hashes and they rotate on deploy, replaying raw query strings may fail. HAR must include `extensions.persistedQuery.sha256Hash`.
2. **Detail/autosuggest/availability operationNames unconfirmed** — HAR is authoritative.
3. **`_abck` TTL** is session-scoped but Akamai revalidates periodically. Long-running CLI needs cookie refresh logic; `auth login --chrome` import + `auth refresh` pattern.
4. **`client-info` header format** unconfirmed for the public frontend.
5. **`__PLUGIN_STATE__` completeness varies**: detail pages have more than search results; pricing fees not in static blob (GraphQL required for compare command).

## Sources
- stevesie.com/apps/vrbo-api (HAR-based scraper writeup)
- apify.com/ecomscrape/vrbo-property-search-scraper
- apify.com/jupri/vrbo-property (input-schema documents the propertyId convention)
- scraperly.com/scrape/vrbo (Akamai bypass details)
- medium.com/expedia-group-tech/graphql-component-architecture-principles-homeaway (GraphQL design)
- substack.thewebscraping.club/p/scraping-akamai-protected-website (Akamai bypass mechanics)
- zenrows.com/blog/bypass-akamai
- github.com/Edioff/vrbo-scraper
- github.com/markswendsen-code/mcp-vrbo
