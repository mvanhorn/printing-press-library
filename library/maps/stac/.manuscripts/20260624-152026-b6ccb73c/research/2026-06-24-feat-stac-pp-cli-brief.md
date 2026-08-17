# STAC API CLI Brief

## API Identity
- Domain: SpatioTemporal Asset Catalog (STAC) — geospatial/earth-observation imagery catalog search.
- Default target: AWS Earth Search v1 (`https://earth-search.aws.element84.com/v1`), `stac-server` (STAC API 1.0.0). Override with `STAC_BASE_URL` / `providers use`.
- Users: remote-sensing analysts, data scientists, GIS engineers, ML/EO pipelines discovering Sentinel-2/Landsat/Sentinel-1/NAIP/Copernicus-DEM scenes.
- Data profile: large GeoJSON Feature items; rich `properties` (eo:cloud_cover, s2:* veg/water/snow %, view:sun_elevation, proj:epsg, grid:code, sat:orbit_state) and `assets` keyed by band name (red/green/blue/nir/swir/visual/thumbnail; COG + jp2 twins).

## Reachability Risk
- None. `GET /collections` → HTTP 200, 9 live collections, no auth.
- Probe-safe endpoint used: GET /collections, GET /aggregate, POST /search.

## Live API Capabilities (probed)
- Filtering: **`query` extension only** — `{"query":{"eo:cloud_cover":{"lt":20}}}` (ops lt/gt/lte/gte/eq/neq). **CQL2 is silently ignored on Earth Search** (no filter conformance class). Build `query`-based filter sugar; offer CQL2 only as feature-detected fallback for capable providers.
- Sortby: works, **object form only** `{field, direction}` (asc/desc). `+`/`-` string form 400s.
- Aggregations: real `GET /aggregate` endpoint — `cloud_cover_frequency`, `datetime_frequency`, `datetime_min/max`, `sun_elevation_frequency`, `total_count`. `GET /aggregations` lists available. **Call the endpoint; never compute locally.**
- Fields extension: works (include/exclude).
- Pagination: **POST-body cursor** — `next` field (`datetime,id,collection`) re-POSTed to `/search` with `merge:false`.
- `numberMatched`/`numberReturned` give free result counts (Context deprecated).
- Queryables: `GET /collections/{id}/queryables` works (JSON Schema; `additionalProperties:true`).

## Competitive Landscape
- **No Go CLI competitor for STAC search.** Full-featured: pystac-client (`stac-client`, Python — search+collections only, raw JSON, awkward output), eodag (Python, 17 providers), stactools (Python, static-catalog authoring), rustac (Rust, the only single-binary today). go-stac (Planet) is validate-only; go-stac-client is an SDK+TUI.
- Object models (no CLI): pystac, stac-js, stac-fields. Loaders: stackstac, odc-stac.
- MCP: essentially one open-source STAC MCP (`Wayfinder-Foundry/stac-mcp`, ~12★, 9 tools, no CQL2, no first-class cloud-cover). planetary-computer-mcp is download-only. **Open lane for a strong STAC MCP surface.**

## Top Workflows (from research)
1. Least-cloudy scene over an AOI in a date range (filter eo:cloud_cover, sort asc).
2. Time series of scenes for change detection (one obs/date, deduped by orbit/tile).
3. Resolve asset/band download URLs (provider-aware band aliasing, COG over jp2).
4. Coverage/availability check over AOI+range (matched count + temporal aggregation).
5. Cloud-cover distribution over AOI+range (real /aggregate).
6. Compare collections (S2 vs Landsat) over same AOI+range.
7. Temporal gaps vs expected revisit.
8. Monitor new scenes since last run (local diff).
9. Bridge to xarray (stackstac/odc-stac) analysis.

## Table Stakes (must match)
- Item search: bbox, datetime (instant/range/open `..`), intersects (POST), collections, ids, limit, max-items.
- `query` filter sugar (eo:cloud_cover, sat:orbit_state, etc.), sortby (object form), fields include/exclude.
- Auto-pagination (bounded), matched/count-only mode, GET+POST search.
- collections list/get/items, item get-by-id, conformance, queryables, aggregate.
- Human-readable table output + `--json`/`--select` (competitors are raw-JSON-only).

## Data Layer
- Primary entities: collections, items (scenes). Sync items per collection+AOI to SQLite for offline search, coverage, gaps, watch.
- Sync cursor: POST-body `next` token; dedupe by item id.
- FTS/search: local FTS over synced item properties.

## Pain Points (opportunity)
- Python-heavy, no Go single binary. Verbose raw JSON. No offline cache. `sortby` not guaranteed (need client-side ranking fallback). CQL2 complexity. Pagination footgun (unbounded). Inconsistent asset keys/collection IDs across providers.

## Product Thesis
- Name: **stac-pp-cli** (display: "STAC")
- Thesis: The first Go single-binary STAC CLI — match every search flag the Python tools have, add the subcommands they lack (queryables, aggregate, item get), and win on what no STAC tool offers: an offline SQLite cache with coverage/timeline/gaps/watch, provider-aware asset resolution, client-side cloud-cover ranking, and a strong agent-native MCP surface.

## Build Priorities
1. Foundation: SQLite store for collections+items, `sync` per collection/AOI, local FTS `search`.
2. Absorb: full item-search surface (all flags, query-filter sugar, sortby, fields, auto-paginate, matched), collections/item/conformance/queryables/aggregate endpoint commands, provider switching.
3. Transcend: best-scene ranking, coverage, timeline, gaps, clouds histogram, asset resolution, compare, watch, xarray snippet.
