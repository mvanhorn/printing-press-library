# Wanderlog CLI Research Brief

## API Identity

Wanderlog is a travel-planning product from Travelchime Inc. Its public web surface combines destination discovery, public guide browsing, shared itineraries, place cards, category lists, routing/distance metadata, hotel deals, budgets, checklists, comments, likes, and session preferences. No official public API or OpenAPI spec was found; the usable contract is the web app's replayable JSON/HTML surface.

## Users

1. **Trip-planning power user**: keeps multiple destination tabs open, compares public guides, then turns candidate places into a day-by-day itinerary.
2. **Travel agent or group planner**: needs to pull a shared Wanderlog itinerary into structured formats for clients, collaborators, or downstream documents.
3. **Guide miner / travel blogger**: regularly scans destination guides, high-signal place lists, and place metadata to find reusable recommendations.
4. **Agentic workflow builder**: wants a scriptable bridge from natural-language planning into Wanderlog-compatible geos, guides, places, and optional cookie-backed trip state.
5. **Maps/offline-export traveler**: wants to carry a Wanderlog trip into KML/CSV/Markdown for Google Maps, Maps.me, Organic Maps, GIS, or offline review.

## Top Workflows

1. **Destination guide ritual**: search a destination, resolve its geo id, list public guides, fetch a guide by key, and extract days, sections, places, notes, metadata, and source URLs.
2. **Shared itinerary export ritual**: open a shared Wanderlog view, recover the embedded trip plan/resources, then emit JSON, Markdown, CSV, or KML with per-day splits.
3. **Place research ritual**: search place suggestions, fetch place details/card data, and inspect category lists for top attractions/restaurants in a destination.
4. **Authenticated personal trip ritual**: use a `connect.sid` session cookie to list personal trips, fetch a trip with resources, create a trip, and optionally edit via ShareDB websocket operations.
5. **Import reconciliation ritual**: take a Google Maps saved-place export or another place list, match names/addresses against Wanderlog places, and produce a matched/missing/name-mismatch audit before adding anything.

## API Surface

### Public, replayable HTTP

- `GET /api/sessionStore` returns anonymous session preferences and locale/currency defaults.
- `GET /api/geo/autocomplete/{query}` returns candidate geos; verified with `Paris` returning geo id `9614`.
- `GET /api/geo/geosWithGoodGuides` returns guide-rich destinations according to MCP source.
- `GET /api/tripPlans/browse/guides/{geoId}` returns public guides for a destination according to MCP source.
- `GET /api/tripPlans/{viewKey}?clientSchemaVersion=2` returns public guide/shared trip data; verified against `uzyvvtuwtc`.
- `GET /api/placesAPI/autocomplete/v2?request=<json>` searches places according to MCP source.
- `GET /api/placesAPI/getPlaceDetails/v2?placeId=...&language=en` returns place details; verified against a Paris place id.
- `GET /api/placesAPI/getPlaceDetailsAndCardData?placeId=...&language=en` returns rich card data; observed in browser-sniff.
- `GET /api/placesList/geoCategory/{geoCategoryId}?includeRelatedPagesData=true` returns a large category/list payload; observed with id `104643`.
- `GET /api/tripPlans/{tripKey}/comments` and `/distinction` return public/social sidecar metadata for shared trips.
- HTML pages `/explore/{geoId}/{slug}`, `/list/geoCategory/{id}/{slug}`, and `/view/{key}/{slug}/shared` include SSR `window.__MOBX_STATE__` / config data and are useful fallback extraction surfaces.

### Auth-gated HTTP / websocket

The MCP source confirms `connect.sid` cookie auth via `WANDERLOG_COOKIE`. Read endpoints include `GET /api/user`, `GET /api/tripPlans/home`, and `GET /api/tripPlans/{tripKey}?clientSchemaVersion=2&registerView=true`. Creation uses `POST /api/tripPlans`. Deletion uses `DELETE /api/tripPlans/{tripKey}`. Editing notes, places, hotels, checklists, expenses, dates, and day names uses ShareDB JSON0 operations over `wss://wanderlog.com/api/tripPlans/wsOverall/{tripKey}?clientSchemaVersion=2`.

## Reachability Gate

- Decision: PASS.
- Evidence: `probe-reachability https://wanderlog.com --json` returned `mode: standard_http`, confidence `0.95`, HTTP 200 for stdlib and Surf probes.
- Browser-sniff traffic analysis saw PerimeterX markers in HTML and classified runtime as `browser_http`, but replayed API JSON endpoints were reachable with standard HTTP during enrichment.
- No clearance cookie was needed for public discovery in this run.
- Risk: private/user trip endpoints require a real `connect.sid` cookie; anonymous capture did not validate mutations.

## Browser-Sniff Result

User approved browser-sniff. Anonymous capture visited `/plan/create`, `/explore/9614/paris`, and a public shared guide. The analyzer emitted 12 endpoints across 9 resources. Manual curation is required because some analyzer paths are sample-hardcoded (`/api/geo/autocomplete/Paris`, `/api/tripPlans/uzyvvtuwtc/comments`) or over-normalized (`/api/placesAPI/{placesapi_id}`). The curated spec corrects these to parameterized paths backed by live probes and MCP source.

## Crowd-Sniff Result

User approved crowd-sniff. `cli-printing-press crowd-sniff --api wanderlog --base-url https://wanderlog.com` failed with `downloads API returned status 400` and `no endpoints discovered for "wanderlog"`. Manual community research found better signals:

- `shaikhspeare/wanderlog-mcp` / npm `wanderlog-mcp` v0.3.1: TypeScript MCP server with cookie auth, public guide search, place search/details, trip list/get/create, and ShareDB-backed mutations.
- `@zaw_ye/wanderlog_mcp`: similar npm package/repo discovered in npm search.
- `danilden1/Wanderlog-to-KML`: Python exporter from saved Wanderlog HTML to combined/per-date KML.
- `devsuhh/wanderlog_importer`: Chrome extension importing Google Maps saved places into a Wanderlog section with notes and reconciliation audit.

## Table Stakes To Absorb

- From Wanderlog MCP: list trips, get trip, get trip URL, search places, search guides, get guide, create trip, add place, add note, edit note, remove note, add hotel, add checklist, add expense, annotate place, remove place, update trip dates, rename day.
- From Wanderlog-to-KML: parse saved Wanderlog HTML, extract all locations, preserve names/coordinates, split KML by travel date, clean KML organization.
- From Wanderlog importer: parse Google Maps saved-place exports, import place names/addresses/notes into a Wanderlog section, provide matched/missing/name-mismatch/extra audit, support CSV/report export.
- From Wanderlog web app: public destination explore pages, place category lists, shared guide/itinerary pages, comments/likes/distinction metadata, session preferences.

## Data Layer

Core entities: geos, guide summaries, trip plans, sections/days, blocks/stops, places, place metadata, geo category lists, comments, distinctions, likes, session preferences, distances/routes, hotel deals, budgets, checklist items, expenses, and import/audit rows.

## Product Thesis

`wanderlog-pp-cli` should be a terminal bridge for Wanderlog public guide mining, itinerary export, and agent-ready trip planning. It should make anonymous public surfaces useful immediately, persist data locally for search/export/audit, and expose cookie-backed private trip reads/creation when the user supplies `WANDERLOG_COOKIE`. Full ShareDB editing is valuable but high-risk; ship only the parts that can be verified or explicitly approve honest stubs for deferred websocket mutations.

## Clone/Fill Priority Addendum

The user clarified on 2026-06-21 that the most important workflow is filling a new Wanderlog plan from an existing shared/public plan. Example source URL: `https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared`.

Live verification against the example shows that `GET /api/tripPlans/naertjcoixqrgrfc?clientSchemaVersion=2` returns structured `tripPlan.itinerary` data, including title, sections, date-bearing dayPlan sections, budget, journal, and resources. This confirms the read/template side.

The write/fill side is not a REST endpoint. It requires cookie-backed trip creation plus ShareDB websocket JSON0 operations. Therefore, `plan preview`, `plan clone`, and `plan fill` are now the primary hand-written shipping scope. Lower-priority local-analysis commands may be trimmed before these are dropped.
