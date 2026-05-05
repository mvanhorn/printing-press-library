# Redfin Discovery Report

## Target
- URL: https://www.redfin.com
- Owner: Redfin Corporation (acquired by Rocket Companies in 2025)
- Spec: none (no public OpenAPI; internal Stingray endpoints documented in scraper docs)

## Capture method
Mixed-source discovery, no live browser capture run:
1. `printing-press probe-reachability https://www.redfin.com --json` → classified as `browser_clearance_http` confidence 0.6 (both stdlib + Surf got 200 with AWS WAF marker; per-IP rate limiting reported by community scrapers).
2. **Direct probe of Stingray endpoints with a Chrome User-Agent** confirmed 200 OK on:
   - `/stingray/api/gis` (returns `{}&&{json}` listing data)
   - `/stingray/api/region/<type>/<id>/<months>/aggregate-trends`
3. **Source-extracted endpoint surface** from public scraper repos (reteps/redfin, dreed47/redfin, wang-ye/redfin-scraper, alientechsw/RedfinPlus docs) and ScrapFly's Redfin scraping guide.
4. CloudFront blocks `/stingray/do/location-autocomplete` for plain stdlib HTTP — Surf with Chrome TLS fingerprint at runtime should clear it (the same pattern that clears apartments.com).

## Runtime decision
- Transport: **Surf with Chrome TLS fingerprint** (`http_transport: browser-chrome`).
- No clearance cookie capture; no resident browser.
- Auth: **none** for v1. All in-scope features work anonymously.
- Geo: US-only. Doctor command should warn non-US users.

## URL surface

### Internal Stingray API
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/stingray/api/gis` | GET | JSON map search (primary). Wrapped in `{}&&{...}` CSRF prefix. |
| `/stingray/api/gis-csv` | GET | Bulk CSV export (350-row cap per call). |
| `/stingray/do/gis-search` | GET | Legacy JSON search (kept as fallback). |
| `/stingray/api/home/details/initialInfo` | GET | First listing call — returns `listingId` and `propertyId`. |
| `/stingray/api/home/details/aboveTheFold` | GET | Photos, headline price/beds/baths/sqft. |
| `/stingray/api/home/details/belowTheFold` | GET | Amenities, price/tax/school history, AVM. |
| `/stingray/api/region/{type}/{id}/{months}/aggregate-trends` | GET | Region market trends. |
| `/stingray/do/location-autocomplete` | GET | Region autocomplete (CloudFront-gated for stdlib; Surf needed). |
| `/stingray/api/v1/rentals/{id}/floorPlans` | GET | Rentals floor plans (out of v1 scope). |

### Public feeds
| Endpoint | Purpose |
|----------|---------|
| `/newest_listings.xml` | RSS-style new-listings feed |
| `/sitemap_com_latest_updates.xml` | Updated-listings feed |

### URL slug pattern
- City: `/city/{region_id}/{state}/{city-name}` (e.g., `/city/30772/TX/Austin`)
- Listing: `/{state}/{city}/{address-slug}/home/{listing_id}`

## Replayability
**Surface qualifies under Cardinal Rule 5:** the Stingray API is a real JSON/CSV API replayable via direct HTTP. The CSRF prefix is a 4-byte strip before decode. The printed CLI uses Surf direct HTTP — no resident browser, no clearance cookie capture.

## Known quirks
- **CSRF prefix:** Every Stingray JSON response starts with the literal bytes `{}&&`. The client must strip before `json.Unmarshal`.
- **CloudFront on autocomplete:** `/stingray/do/location-autocomplete` returns 403 from CloudFront via stdlib HTTP. Surf TLS fingerprint should clear; if it doesn't, document workaround (paste region_id from URL).
- **350-row gis-csv cap:** Bulk export must slice price bands and dedupe across bands.
- **Per-IP rate limiting:** Community scrapers report blocks at scale. `cliutil.AdaptiveLimiter` defaults to 1 req/s with exponential backoff on 429.

## Notes for the generator
- `http_transport: browser-chrome` MUST be set in the spec.
- Client must implement CSRF-prefix stripping in a wrapper around `c.Get`.
- Most novel commands are hand-built in Phase 3 (synthetic CLI).
