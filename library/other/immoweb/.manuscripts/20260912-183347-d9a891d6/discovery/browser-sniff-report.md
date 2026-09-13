# Immoweb browser-sniff report

## 1. User Goal Flow
- Goal: find listings matching precise criteria in a commune, open one listing's full details, and track changes.
- Steps completed: (1) search page for houses for sale → search-results JSON; (2) listing detail page (logged-in session) → classified JSON, statistics summary, saved-searches, can-be-contacted; (3) favourites page (logged in) → server-rendered, no XHR; (4) autocomplete/count/map/similar via page-config URLs.
- Steps skipped: toggling a favourite heart (write action on the user's real account — not performed).
- Coverage: 4 of 5 planned steps.

## 2. Pages & Interactions
- https://www.immoweb.be/fr/recherche/maison/a-vendre (curl) — extracted base64 endpoint config (`searchResultsJsonUrl`, `searchResultsJsonCountUrl`, `searchMapResultsJsonUrl`, `searchAutocompleteUrl`, `classifiedJsonUrl`, `searchRenderedSimilarResultsJsonUrl`).
- https://www.immoweb.be/fr/annonce/21828249 (user's Chrome via Claude-in-Chrome, new tab, logged in) — scrolled to bottom; Performance API collected XHR/fetch.
- https://www.immoweb.be/fr/profil/favoris (same tab) — page loaded; 0 saved properties; no XHR.
- Tab closed after capture.

## 3. Browser-Sniff Configuration
- Backends: direct HTTP (curl, Chrome UA) for public endpoints; chrome-MCP (user's Chrome, fresh tab) for the authenticated pass.
- Pacing: 1 s between capture requests; ~1 req/s effective.
- Proxy-envelope pattern: not detected. GraphQL BFF: not detected. REST JSON (Laravel/PHP backend, Vue front-end).

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| GET | /{lang}/search-results | 200 | application/json | public |
| GET | /{lang}/search-results-count | 200 | application/json | public |
| GET | /{lang}/search-results-map | 200 | application/json | public |
| GET | /{lang}/classified/get-result/{id} | 200 | application/json | public |
| GET | /{lang}/search/autocomplete | 200 | application/json | public |
| GET | /{lang}/search-similar-results | 200 | application/json | public |
| GET | /{lang}/statistics/summary/{id} | 200 in browser / 404 via curl | application/json | session-bound (redundant: classified.statistics has viewCount/bookmarkCount) |
| GET | /{lang}/saved-searches/get-saved-searches | 200 (logged in) / login redirect | application/json | auth-required |
| GET | /{lang}/profil/favoris | 200 SSR (logged in) / 302 to /connexion | text/html | auth-required |
| GET | /{lang}/email/can-be-contacted/{id} | 200 | application/json | public (contact-form gating) |

## 5. Traffic Analysis
- Protocols: rest_json (0.75). Reachability: standard_http (probe-reachability 0.95).
- Auth signals: none required for public surface. Authenticated surface = Laravel session cookie (`immoweb_session`) + XSRF-TOKEN; only saved-searches (JSON) and favourites (SSR HTML).
- Protection signals: Cloudflare in front; DataDome tag (`dd.immoweb.be/tags.js`) on HTML pages; not enforced on JSON routes in our tests. Community reports 2026: DataDome blocks datacenter IPs on HTML pages; soft throttle = 200 with empty `results`.
- Parameter evidence: filter names taken from the search page's own criteria keys (minPrice, maxPrice, minBedroomCount, minSurface, minLandSurface, minConstructionYear, epcScores, hasGarden, hasSwimmingPool, isNewlyBuilt, isALifeAnnuitySale, isAPublicSale, postalCodes, provinces, districts, orderBy) and validated against search-results-count.
- Warnings: invalid enum values return a 200/4xx HTML error page instead of JSON; `totalItems` caps at 9969 (use count endpoint).

## 6. Coverage Analysis
- Exercised: search, count, map, detail, autocomplete, similar, saved searches (auth), favourites (auth).
- Not exercised: favourite add/remove (write), saved-search create (write), contact-agent email (write) — intentionally out of scope for a read-first CLI.

## 7. Response Samples
- search-results: `{"criteria":{"countries":"BE","propertyTypes":"HOUSE","transactionTypes":"FOR_SALE","page":"1"},"sort":"relevance","results":[{"id":21828249,"property":{"type":"HOUSE","bedroomCount":4,"location":{"locality":"Dilbeek","postalCode":"1700","latitude":50.84,"longitude":4.27},"netHabitableSurface":160,"landSurface":46},"transaction":{"type":"FOR_SALE","sale":{"price":449000}},"price":{"mainValue":449000},"publication":{"lastModificationDate":"2026-09-11T15:00:42Z"}}],"totalItems":9969}` (truncated)
- search-results-count: `{"classifiedsCount":203}`
- autocomplete: `[{"group":"Localité","results":[{"type":"LOCALITY_NAME","label":"Ixelles (1050)","queryParam":"postalCodes","queryValue":"BE-1050"}]}]`
- classified/get-result: `{"classified":{"id":21828249,"property":{...},"transaction":{"certificates":{"epcScore":"D","primaryEnergyConsumptionPerSqm":317},"sale":{"price":449000,"cadastralIncome":1249}},"publication":{"creationDate":"2026-09-09T16:15:34Z"},"statistics":{"viewCount":2073,"bookmarkCount":16}}}` (truncated)

## 8. Rate Limiting Events
- None. 10-request burst + 12 capture requests, all 200 at ~0.5 s latency.

## 9. Authentication Context
- Authenticated pass used the user's existing Chrome session via chrome-MCP (fresh tab, closed after). No cookie values were read or stored.
- Auth-only endpoints: saved searches (JSON), favourites (SSR HTML). The user's account had 0 of each.
- Decision: ship public-only. The auth surface is thin (two list views, one HTML-only), cookie import would require a macOS Keychain prompt plus a Cloudflare/DataDome-bound session that may not replay, and the CLI's own local saved searches + watch give the client the same outcome with zero login.

## 10. Bundle Extraction
- app.js (1.8 MB) analysed: endpoint URLs are injected as base64 strings in the page config, not in the bundle. Bundle confirmed filter vocabulary (orderBy: relevance/newest/cheapest/most_expensive/postal_code; epcScores; geoSearchPoint; provinces/districts).
