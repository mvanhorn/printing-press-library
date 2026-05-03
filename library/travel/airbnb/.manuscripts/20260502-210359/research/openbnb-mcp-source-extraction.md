# openbnb-org/mcp-server-airbnb — Source Code Extraction

Scope: Phase 1.5a.5. Read by absorb manifest builder.

## Headline (load-bearing surprise)

The 443-star MCP does NOT use Airbnb's GraphQL API. It scrapes public SSR HTML.
- No `/api/v3/StaysSearch`
- No `X-Airbnb-API-Key` bootstrap
- No DataDome workaround
- No auth (works fully logged-out)

## Endpoints used
- GET `https://www.airbnb.com/s/{slug}/homes` (search HTML)
- GET `https://www.airbnb.com/rooms/{id}` (listing detail HTML)
- GET `https://photon.komoot.io/api/?q={location}&limit=5` (geocoding primary)
- GET `https://nominatim.openstreetmap.org/search?q={location}&format=json&limit=1` (fallback)
- GET `https://www.airbnb.com/robots.txt` (compliance)

Search query params (URLSearchParams on HTML URL): checkin, checkout, adults, children, infants, pets, price_min, price_max, room_types[] (mapped from propertyType), cursor, place_id, ne_lat, ne_lng, sw_lat, sw_lng, zoom (bbox from geocoding).

## Auth bootstrap
NONE. Anonymous HTML fetch. User-Agent: `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36`.

For geocoders: rotate UA to `mcp-server-airbnb/{VERSION} (+https://github.com/openbnb-org/mcp-server-airbnb)` per Photon/Nominatim ToS.

## Reusable extraction pattern

```javascript
const html = await response.text();
const $ = cheerio.load(html);
const scriptContent = $("#data-deferred-state-0").first().text();
const clientData = JSON.parse(scriptContent);
// Walk: clientData.niobeClientData[0][1] = SSR Apollo cache 'data' payload
```

## Search response field paths
Root: `data.presentation.staysSearch.results.searchResults[]`. Per-result:
- `listing.id`, `listing.name`, `listing.title`
- `listing.coordinate.latitude` / `.longitude`
- `listing.structuredContent.primaryLine[]`, `secondaryLine[]`
- `listing.avgRatingLocalized`, `listing.avgRatingA11yLabel`, `reviewsCount`
- `pricingQuote.structuredStayDisplayPrice.primaryLine.price` / `.discountedPrice` / `.originalPrice` / `.qualifier`
- `pricingQuote.structuredStayDisplayPrice.secondaryLine.price` (total)
- `listing.formattedBadges[].text`
- Pagination cursors: `data.presentation.staysSearch.results.paginationInfo.pageCursors[]`

## Listing detail field paths
Root: `data.presentation.stayProductDetailPage.sections.sections[]`. Filtered by `sectionId` (amenities, house rules, location, highlights, description, policies). Helpers `pickBySchema()`, `flattenArraysInObject()`, `cleanObject()` strip null/undefined/__typename.

## Brittleness signals
- `#data-deferred-state-0` selector is single point of failure
- `niobeClientData[0][1]` positional indexing into Apollo cache is fragile
- Section-ID filtering for listing details is hardcoded
- No retry/backoff, no proxy support
- Photon bbox not always tight for large regions (cities OK, countries bad)
- robots.txt currently disallows `/s/` for some UAs - production needs `IGNORE_ROBOTS_TXT=true` env or `--ignore-robots-txt` flag

## What it does NOT cover (gaps for our CLI to fill)
- Wishlist read/write (requires auth)
- Trip history (requires auth)
- Saved listings (requires auth)
- Host profile detail (different endpoint)
- Reviews list (different endpoint)
- Calendar/per-night pricing (might be in stayProductDetailPage but openbnb doesn't extract)
- Map cluster pagination (different param)

These are the gaps where authenticated browser-sniff would add value.

## License
MIT - we can adopt the SSR-extraction pattern in our CLI without restriction (with attribution in NOTICE/README).
