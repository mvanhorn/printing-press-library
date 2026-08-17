# Zameen Browser-Sniff / Discovery Report

## User Goal Flow
- **Goal:** Search Zameen for properties (buy/rent, by city/area/price/beds) and read listing data.
- **Steps completed:** Homepage load → search-results page load (Homes/Islamabad) → paginate (page 2) → cross-purpose (Rentals, Plots, Commercial) → cross-city (Lahore, Karachi, Rawalpindi, Faisalabad, Multan) → property-detail link inspection.
- **Coverage:** Full for the primary search workflow. Detail/agency/new-project pages inspected but not deeply mapped (secondary).

## Discovery Method & Configuration
- **Backend:** Direct HTTP discovery (not browser automation). `browser-use` present but installed as the 3.0 skill interface (`js()`/`goto_url()`), not the 2.0 `open`/`eval` CLI the flow scripts; per skill guidance ("do not burn budget debugging tool integration"), and because `probe-reachability` returned **`standard_http`**, discovery used `curl` + Python extraction of the server-rendered `window.state` blob. This is the exact surface the printed CLI ships against, so direct-HTTP discovery is representative.
- **Reachability:** `cli-printing-press probe-reachability https://www.zameen.com/` → `mode: standard_http`, confidence 0.95, `needs_browser_capture: false`, `needs_clearance_cookie: false`. Both stdlib and surf-chrome probes returned HTTP 200.
- **Pacing:** ~1 req/s during discovery; no 429s encountered.
- **Proxy pattern:** none (path-based server-rendered pages).

## Endpoints Discovered (replayable surface)
| Method | Path | Status | Content-Type | Notes |
|--------|------|--------|--------------|-------|
| GET | `/{Category}/{location-slug}-{page}.html` | 200 | text/html | Category ∈ Homes/Rentals/Plots/Commercial; listings in `window.state.algolia.content.hits[]`; 25/page; `nbHits`,`nbPages` present |
| GET | `/Property/{slug}-{externalID}-{locationID}-{seq}.html` | 200 | text/html | Property detail page (constructable from a hit) |
| GET | `/new-projects/` | 200 | text/html | New developments / off-plan (secondary) |
| GET | `/agents/{City}/{Agency-Slug-ID}/` | 200 | text/html | Agency directory + inventory (secondary) |

## Traffic Analysis
- **Protocol:** `ssr_embedded_data` — server-rendered HTML with a `window.state = {…}` Redux JSON blob. High confidence.
- **Auth signals:** none required for public search/listing data. `state.user.loggedIn=false` on anonymous fetch; favorites/alerts require login (out of scope for v1).
- **Runtime shape:** **standard HTTP** with a browser User-Agent header. No resident browser, no clearance cookie.
- **Backend note:** the page also exposes a raw AWS Elasticsearch endpoint and an embedded search credential. **Deliberately NOT used** — shipping that credential would leak a secret, and the credential-free public HTML surface fully covers search. No credentials are stored in any artifact.
- **Filters:** encoded in the URL path (long-tail), not query strings (`?beds_in`/`?price_min`/`?sort` confirmed ignored). CLI applies filters/sort client-side over scanned pages.

## Listing Schema (from `algolia.content.hits[]`)
`id`/`externalID`, `title`, `price` (PKR int), `area` (float), `rooms` (beds), `baths`, `purpose`, `category`+subtype, `location` (hierarchical), `agency`, `contactName`, `phoneNumber`, `coverPhoto`, `photoCount`, `videoCount`, `geography` (lat/lng), `isVerified`, `product` (ad tier), `createdAt`/`updatedAt` (unix), `slug`, `shortDescription`, `installments`, `state`.

## Coverage Analysis
- **Exercised:** listing search across all 4 categories, both purposes, 6 cities, pagination.
- **Confirmed pagination math:** e.g. Islamabad Homes = 14,329 hits / 574 pages @ 25/page.
- **Likely missed / secondary:** area price-trend series, agency detail schema, new-project payment-plan schema. These are additional `window.state` slots hand-buildable later.

## Rate Limiting Events
None. ~15 requests at ~1 req/s, all HTTP 200.

## Authentication Context
No authenticated session used. Public search/listing data needs no auth. Favorites/saved-alerts (login-gated) are out of scope for v1. No session state written to any artifact.

## Replayability Verdict
**PASS** — the surface replays through plain standard HTTP (`curl`/Go `net/http`) with a Chrome User-Agent. No browser sidecar, no clearance cookie, no credential. The printed CLI ships `http_transport: standard`.
