# Apartments.com Discovery Report

## Target

- URL: https://www.apartments.com
- Owner: CoStar Group
- Spec: none (no public OpenAPI/GraphQL)

## Capture method

Mixed-source discovery, **no live browser capture run** (per Phase 1.7 user decision):

1. `printing-press probe-reachability https://www.apartments.com --json` → classified runtime as `browser_http` (Surf with Chrome TLS fingerprint clears the 403 the stdlib HTTP probe sees).
2. **Source-extracted endpoint surface** from five public Python scrapers (cccdenhart/apartments-scraper, adinutzyc21/apartments-scraper, johnludwigm/PyApartments, shilongdai/Apartment_Scraper, davidhuang620). All five are functionally broken since 2021 (bot-blocked) but their URL builders and CSS selectors are still authoritative for the path-based search surface and schema.org microdata extraction.
3. **Documented community URLs** (Apify scrapers and apartments.com renter help) confirm path filters: `min-N-bedrooms`, `under-PRICE`, `MIN-to-MAX`, `pet-friendly[-cat-or-dog|-dog]`, property-type prefixes (`/houses/`, `/condos/`, `/townhomes/`).

## Runtime decision

- Transport: **Surf with Chrome TLS fingerprint** (`http_transport: browser-chrome`).
- No clearance cookie capture, no resident browser. The probe shows Surf alone clears the protection at runtime — that's the entire production runtime.
- Auth: **none** for v1. All in-scope features work anonymously.

## URL surface (path-based)

```
https://www.apartments.com/{city}-{state}/[{filter1}-{filter2}-...]/[{page}/]
```

Filter fragments (compose with hyphens):

| Fragment | Meaning |
|----------|---------|
| `studio` | Studio apartments |
| `1-bedrooms`, `2-bedrooms`, ..., `5-bedrooms` | Exact bedrooms |
| `min-N-bedrooms` | At least N bedrooms |
| `1-bathrooms`, `min-N-bathrooms` | Bathroom counts |
| `under-PRICE` | Max rent (no comma in PRICE) |
| `MIN-to-MAX` | Rent range |
| `pet-friendly` | Allows any pets |
| `pet-friendly-dog` | Allows dogs |
| `pet-friendly-cat-or-dog` | Allows both |

Property-type prefixes (mutually exclusive with default `/`):
- `/houses/{city}-{state}/...`
- `/condos/{city}-{state}/...`
- `/townhomes/{city}-{state}/...`

Pagination: trailing `/{page}/` (1-indexed; omitted on page 1).

ZIP variant: `/{zipcode}/...` instead of `/{city}-{state}/...`.

## Listing detail surface

Each listing has a canonical URL of the form `https://www.apartments.com/{property-slug}/`. The page is server-rendered HTML containing:

- `<meta itemprop="streetAddress" content="...">`
- `<meta itemprop="addressLocality" content="...">`
- `<meta itemprop="addressRegion" content="...">`
- `<meta itemprop="postalCode" content="...">`
- `data-beds`, `data-baths`, `data-maxrent` attributes on placard markers
- Availability table: `table.availabilityTable.basic.oneRental`
- Amenity bullet lists, photo gallery URLs, contact phone, floor-plan section

These are all stable, schema.org-aligned extraction targets.

## Replayability

**Surface qualifies under Cardinal Rule 5 of `references/browser-sniff-capture.md`:** "structured HTML/SSR/RSS/JSON-LD extraction targets" are a shippable surface. The printed CLI uses Surf direct HTTP — no resident browser, no clearance cookie capture.

## Notes for the generator

- `http_transport: browser-chrome` MUST be set in the spec (drives `UsesBrowserHTTPTransport: true`).
- All endpoints use `response_format: html` with `html_extract.mode: page`.
- Most user-facing functionality is hand-built in Phase 3 (synthetic CLI; spec is the path-surface scaffold, transcendence features are local-store/SQL).
- Per-source rate limiter: apartments.com hasn't documented limits but Akamai layers may throttle; default conservative pacing (1s) with adaptive backoff in `cliutil.AdaptiveLimiter`.
