# Pointhound Browser-Sniff Discovery Report

## User Goal Flow
- **Goal:** Find award flight redemptions for a specific route + date (canonical Pointhound search workflow).
- **Steps completed:**
  1. Load `/search` (search builder page) — confirmed authenticated session via `loggedIn=true` heuristic on DOM.
  2. Type "SFO" into origin combobox → `scout.pointhound.com/places/search` autocomplete fired.
  3. Type "LIS" into destination combobox → second autocomplete request; selected Humberto Delgado.
  4. Open date picker → calendar UI; selected June 15, 2026.
  5. Click "Fetch deals" → form submit redirected to `/flights?q=<base64-protobuf>` URL, page loaded with search results.
  6. Page polled `/api/offers?searchId=ofs_xxxxxxxxxx` with multiple filter combinations and sort options (points, duration).
- **Steps skipped:** Authenticated alert creation, digest preferences, points-program management — Supabase RPC requests fired but bodies couldn't be captured cleanly through chrome-MCP's redaction filter.
- **Secondary flows attempted:** `/alerts` (marketing page, not management UI) and `/account/points-and-miles` (Supabase RPC `get_user_with_subscription` fired but redacted).
- **Coverage:** 6 of ~9 planned steps completed; auth-gated CRUD endpoints partially discovered via Supabase URL inspection only.

## Pages & Interactions
1. `/` (homepage) — interceptor installed; Supabase `get_user_with_subscription` fired on initial load.
2. `/search` (search builder) — typed SFO into origin, clicked first autocomplete result; typed LIS into destination, clicked first autocomplete result; clicked date input; clicked June 15 in calendar; clicked Select; clicked Fetch deals.
3. `/flights?q=<base64-protobuf>` (results page) — observed page loading deals via `/api/offers` with various filter combinations; tested anonymous replayability of the same `searchId`.
4. `/alerts` (marketing) — confirmed not the management UI; no /api/alerts endpoint exists.
5. `/account/points-and-miles` (auth-gated) — Supabase RPC `get_user_with_subscription` fired, response shape not capturable.

## Browser-Sniff Configuration
- **Backend:** chrome-MCP (browser extension drives a fresh tab in the user's already-running Chrome session).
- **Pacing:** Default 1s between agent-initiated calls; no 429s observed.
- **Proxy pattern detection:** Not a proxy-envelope pattern. URLs are conventional REST routes on `/api/offers/*`.

## Endpoints Discovered

| Method | Path | Status | Content-Type | Auth | Replayable Anonymously? |
|--------|------|--------|--------------|------|--------------------------|
| GET | `/api/offers` | 200 | application/json | none | yes (verified by curl 200) |
| GET | `/api/offers/filter-options` | 200 | application/json | none | yes (verified by curl 200) |
| POST | `/flights?q=<base64-protobuf>` | 200 (via Chrome) / 403 (plain HTTP & Surf) | text/html | cf_clearance + ph_session cookie | **no** — requires browser-clearance cookie |
| GET | `https://scout.pointhound.com/places/search` | 200 | application/json | none | yes (separate base) |
| POST | `https://db.pointhound.com/rest/v1/rpc/get_user_with_subscription` | 200 | application/json | Supabase JWT | requires JWT from logged-in session |
| GET | `/cards/<slug>` (SSR page) | 200 | text/html | none | yes (page scrape) |
| GET | `/points101` (SSR page) | 200 | text/html | none | yes (page scrape) |
| GET | `/cards` (SSR page) | 200 | text/html | none | yes (page scrape) |

Probed-and-confirmed-404: `/api/alerts`, `/api/search`, `/api/searches`, `/api/me`, `/api/user`, `/api/digest`, `/api/airports`, `/api/airlines`, `/api/programs`, `/api/health`. Pointhound's frontend uses Supabase RPCs at `db.pointhound.com` for user-data CRUD rather than Next.js API routes.

`/api/cards` returned 400 with no params; no working schema discovered. Treat cards data as page-scrape, not REST.

## Traffic Analysis Summary
- **Protocol observed:** `rest_json` (confidence ~0.95). Conventional GET requests with query-string params for `/api/offers/*`; no GraphQL, no protobuf body envelope on read endpoints.
- **One protobuf surface:** The `q` URL parameter for `/flights?q=...` is URL-safe base64 of a small protobuf message:
  - Field 1 (Origin): `{ code, name }`
  - Field 2 (Destination): `{ code, name }`
  - Field 3 (Date): ISO 8601 date string
  - Field 4 (Cabin): int (1=economy, 2=premium, 3=business, 4=first inferred)
  - Field 5 (Passengers): int
- **Auth signals:** Three distinct auth surfaces:
  1. Anonymous read for `/api/offers*` and `scout.pointhound.com/places/search` (no headers).
  2. Browser-clearance cookie (`cf_clearance` from Cloudflare + `ph_session` or similar) for `/flights` POST (search creation).
  3. Supabase JWT for `db.pointhound.com/rest/v1/rpc/*` (extractable from `ph_app` localStorage key in browser).
- **Parameter-name evidence:**
  - `/api/offers` query params (verified via repeated requests): `searchId`, `take`, `offset`, `sortOrder` (`asc`/`desc`), `sortBy` (`points`/`duration`/`departsAt`), `cabins` (`economy`/`premium_economy`/`business`/`first`), `passengers` (int), `airlines` (CSV of `aln_*` ids), and implicit support for `cardPrograms` / `airlinePrograms` / `stops` based on filter-options facets.
  - `scout.pointhound.com/places/search` query params: `q`, `limit`, `metro`, `bound`, `live`, `v`.
- **Protection signals:**
  - Cloudflare 403 with bot-protection HTML on `/flights` POST. `probe-reachability` returned `mode: browser_clearance_http, confidence: 0.6` on stdlib + Surf both 403.
  - `/api/offers/*` and `scout.pointhound.com` return clean 200 on plain stdlib HTTP — runtime `standard_http`.
- **Generation hints:**
  - Multi-base-URL spec: primary `https://www.pointhound.com`, secondary `https://scout.pointhound.com`, tertiary `https://db.pointhound.com`.
  - Reusable cookie-based auth via `auth login --chrome` so the CLI can perform search creation. The read-heavy commands work without auth.
  - The protobuf encoder for `q` will need to be implemented as a small hand-written Go helper.

- **Candidate commands worth considering:**
  - `offers <searchId>` — paginated offers list with filters.
  - `filter-options <searchId>` — fetch facets for a given search.
  - `airports <query>` — autocomplete via scout.
  - `search <origin> <dest> <date>` — composite: encode protobuf → POST /flights with clearance cookie → parse HTML for searchId → return it for use with `offers`.
  - `points-and-miles` — Supabase RPC `get_user_with_subscription` (auth-gated, JWT-bearing).
  - `cards`, `card <slug>`, `points101` — page scrapes.

- **Warnings:**
  - Search creation is **the only command that requires clearance + session cookies**. Without `auth login --chrome`, the CLI is still useful (the user can paste a searchId from the website URL) but cannot initiate searches programmatically.
  - The protobuf shape was reverse-engineered from one observed sample. The cabin enum (1/2/3/4) is inferred; if Pointhound changes the proto, the encoder will need updating.
  - `db.pointhound.com` is a public Supabase project; the apikey for anonymous queries is publicly bundled in the Next.js JS, but user-scoped RPCs need a per-session JWT. Treat the Supabase surface as auth-gated for v1.

## Coverage Analysis
- **Exercised:** offers (with multiple filter/sort combos), places/airports autocomplete, points-program catalog (via filter-options), credit-card page existence (sitemap-confirmed).
- **Likely missed:**
  - Authenticated alerts CRUD (the management UI was not visible; create/list/delete probably go through Supabase RPCs we did not name).
  - Authenticated digest preferences (same — Supabase RPCs).
  - Wallet endpoints (`/account/wallet` exists per sitemap; URL of the data RPC is not directly visible).
  - The "Top Deals" search workflow (the brief mentioned 6×6 origin/destination matrix; only Standard Search was exercised).
- **Brief alignment check:** Brief listed 5 top workflows; brower-sniff hit 1 (Standard Search) end-to-end, confirmed 3 surface paths (alerts/digest/cards), missed Top Deals matrix entirely.

## Response Samples
### `/api/offers/filter-options?searchId=ofs_xxxxxxxxxx` (200, application/json, 1221 bytes)
```json
{
  "cardPrograms": [
    {"id":"pep_ZRGnAepMcY","name":"Amex Membership Rewards"},
    {"id":"pep_a9ue0M8Jmz","name":"Bilt Rewards"},
    {"id":"pep_NPBPpD0mdW","name":"British Airways Avios"},
    {"id":"pep_3HN9iHCTDI","name":"Capital One Rewards"},
    {"id":"pep_LJ3oxvytYb","name":"Chase Ultimate Rewards"},
    {"id":"pep_LzwFfWwlgs","name":"Citi ThankYou Points"},
    {"id":"pep_d6rwDXBocV","name":"IHG One Rewards"},
    {"id":"pep_oKAeOffKQ7","name":"Marriott Bonvoy"},
    {"id":"pep_aDWLZhPwam","name":"Rove Miles"},
    {"id":"pep_6c9D0HaDje","name":"World of Hyatt Loyalty Program"}
  ],
  "airlinePrograms": [
    {"id":"prp_20MH5wRp83","name":"Atmos Rewards"},
    {"id":"prp_5EyDDagMq1","name":"Delta SkyMiles"},
    {"id":"prp_w7axoHTMrg","name":"Qatar Airways Privilege Club"},
    {"id":"prp_TcZLnN6OgF","name":"United MileagePlus"}
  ],
  "airlines": [
    {"id":"aln_Z978AdLy1s","name":"Aer Lingus"},
    /* ...8 more truncated */
  ]
}
```

### `https://scout.pointhound.com/places/search?q=SFO&limit=10&metro=true&bound=false&live=true&v=2` (200, application/json, 304 bytes)
```json
{
  "results": [
    {
      "rank": 0.1,
      "code": "SFO",
      "type": "airport",
      "name": "San Francisco Intl Airport",
      "city": "San Francisco",
      "stateCode": "CA",
      "stateName": "California",
      "regionName": null,
      "countryCode": "US",
      "countryName": "United States",
      "dealRating": "high",
      "sortPriority": null,
      "isTracked": true
    }
  ],
  "searchStatus": "found"
}
```

### `/api/offers?searchId=ofs_xxxxxxxxxx&take=2&offset=0&sortOrder=asc&sortBy=points&cabins=economy&passengers=1` (200, application/json, 26101 bytes)
Top-level: `{ data: [...], count: N }`. Offer entry shape (keys observed):
```
id, departsAt, arrivesAt, cabinClass, originCode, totalStops, pricePoints,
airlinesList, totalDuration, flightNumbers, arrivesAtHour, departsAtHour,
pricePerPoint, priceRetailTax, priceRetailBase, destinationCode,
remoteUpdatedAt, remoteCreatedAt, priceRetailTotal, sourceIdentifier,
quantityRemaining, priceRetailCurrency, source, bestPricePoints,
offerFlightSegments[], <5 redacted-by-chrome-mcp deeplink fields>
```

Sample first offer (subset, non-PII fields):
```json
{
  "airlinesList": "UA, UA",
  "arrivesAt": "2026-06-16T10:30:00.000Z",
  "cabinClass": "economy",
  "bestPricePoints": 70000,
  "departsAt": "2026-06-15T12:35:00.000Z",
  "destinationCode": "LIS",
  "originCode": "SFO",
  "pricePoints": 70000,
  "quantityRemaining": 9,
  "priceRetailCurrency": "USD",
  "priceRetailTotal": "974",
  "totalDuration": 835,
  "totalStops": 1,
  "sourceIdentifier": "united",
  "source": {
    "id": "src_YwtplePYEH",
    "identifier": "united",
    "name": "United MileagePlus",
    "pointRedeemProgramId": "prp_TcZLnN6OgF",
    "redeemProgram": {
      "id": "prp_TcZLnN6OgF",
      "name": "United MileagePlus",
      "transferOptions": [
        { "earnProgramId": "pep_oKAeOffKQ7", "transferRatio": "0.333", "totalTransferRatio": "0.333", "transferTime": "up_to_72" },
        { "earnProgramId": "pep_LJ3oxvytYb", "transferRatio": "1", "totalTransferRatio": "1", "transferTime": "instant" },
        { "earnProgramId": "pep_a9ue0M8Jmz", "transferRatio": "1", "totalTransferRatio": "1", "transferTime": "instant" }
      ]
    }
  },
  "offerFlightSegments": [/* 2 segments with airline, originAirport, destinationAirport nested */]
}
```

Segment shape (keys):
```
id (seg_*), order, offerId (off_*), duration, departsAt, arrivesAt,
originCode, cabinClass, airlineCode, aircraftName, flightNumber,
departureDate, destinationCode, airline {id, name, logoUrl, iataCode},
originAirport {id, name, timezone, iataCode, identifier, municipality, availabilityName},
destinationAirport {ditto}
```

## ID Prefix Conventions (Pointhound's internal scheme)
| Prefix | Entity |
|--------|--------|
| `pep_` | Points Earn Program (transferable points: Chase UR, Amex MR, Bilt, etc.) |
| `prp_` | Points Redeem Program (airline frequent flyer: United MileagePlus, etc.) |
| `pto_` | Points Transfer Option (the actual ratio between earn and redeem) |
| `aln_` | Airline |
| `apt_` | Airport |
| `src_` | Source (booking provider/deal source) |
| `seg_` | Offer flight segment |
| `off_` | Offer |
| `ofs_` | Offers Search (the search session) |

These prefixes are stable across all observed responses; safe to embed in the spec as `pattern: "^pep_[A-Za-z0-9]+$"` etc.

## Auth Strategy Decision
- **`auth.type: cookie`** for the spec.
- **Two required cookies for search-create:** `cf_clearance` (Cloudflare token, captured during browser login) and a Pointhound session cookie (name TBD by the import step). All other commands run anonymously.
- **CLI flow:**
  - `pointhound-pp-cli auth login --chrome` — imports cookies from Chrome.
  - `pointhound-pp-cli auth status` — reports whether cookies are present and likely fresh.
  - All read commands ignore cookies; the `search` create command checks cookies and fails fast with a helpful error if missing.

## Cookie Replay Validation (Step 2d)
The cookie-replay test was not executed against the live site because the chrome-MCP capture redacted cookie header strings. Defer the validation to the printed CLI's first run of `pointhound-pp-cli search`: if it succeeds, cookies replay; if it fails, the CLI reports the failure clearly and the user re-runs `auth login --chrome`.

## Recommended Generation Path
1. Internal YAML spec covering the four anonymous endpoints (`/api/offers`, `/api/offers/filter-options`, `scout/places/search`, `cards-page-scrape`) — primary path.
2. Hand-written novel command for `search` (`POST /flights?q=<protobuf>`) — the only cookie-required command, treated as a transcendence command since it requires Go-side protobuf encoding.
3. Skip Supabase RPCs in v1 — note them as a v0.2 stub for `pointhound-pp-cli account` if user demand surfaces.
