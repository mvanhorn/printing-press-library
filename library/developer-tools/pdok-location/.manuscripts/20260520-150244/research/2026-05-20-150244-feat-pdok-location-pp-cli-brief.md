# PDOK Location CLI Brief

Combo CLI: PDOK Locatieserver (Solr-based geocoder, primary) + Kadaster Location
API (OGC API Features, secondary).

## API Identity

- **Domain:** Dutch government geocoding & location lookup. Open, free, no
  authentication. Data is BAG (addresses), BRK/DKK (cadastral parcels), NWB
  (national roads & hectometer markers), CBS (neighbourhoods, districts),
  HWH (water boards), Bestuurlijke Grenzen (administrative boundaries).
- **Users:** Dutch developers, public-sector teams, GIS analysts, civic-tech
  builders, anyone integrating Dutch addresses or geometries into apps, maps,
  forms, data pipelines.
- **Data profile:** ~9M BAG addresses, ~2400 woonplaatsen, 342 gemeenten,
  12 provincies, ~10M cadastral parcels, ~500K road segments + hectometer
  markers. The Locatieserver is a fast Solr index over all of it; the Location
  API exposes the same domain as OGC API Features collections with full
  geometries and CRS support.

## Reachability Risk

- **None.** Both APIs returned 200s on every probe; no auth, no rate-limit
  headers, no Cloudflare/WAF, no terms gating programmatic use. Documented as
  "open en gratis" (open and free) for everyone.
- Practical limits noted in docs: `rows<=50`, `start<=10000` on Locatieserver
  pagination. `distance` on `/reverse` is meters and required for tight
  matches.

## Top Workflows

1. **Geocode a CSV of addresses** — read 50K rows, call `/free` per row, write
   out lat/lon + RD/WGS84 + match score. This is the dominant batch workflow
   and nobody ships a good CLI for it today.
2. **Address autocomplete chain** — `suggest` returns IDs, then `lookup`
   resolves any chosen ID into the full geometry. The standard "Dutch form
   address autocomplete" pattern.
3. **Reverse geocode coordinates** — point (lat/lon or RD X/Y) → nearest
   address / parcel / road / hectometer marker. Used for map-click handlers,
   incident reports, mobile field data.
4. **Filter by administrative source** — find all gemeenten matching X, find
   the parcel by kadastrale aanduiding, look up provincie by name. Solr
   `fq=type:gemeente` / `fq=bron:BAG` filters power this.
5. **Pull full GeoJSON for a collection feature** — Location API
   `/collections/{adres|gebouw|perceel|woonplaats|gemeentegebied|...}/items/{id}`
   returns full GeoJSON geometry in any of four CRSes. Locatieserver returns
   geometry too but only via lookup, only in WKT, and not as proper GeoJSON.
6. **Bounding-box queries** — Location API supports `bbox` filtering on
   `/search` and per-collection `/items`. Locatieserver has no bbox; only
   `lat`/`lon` distance-sorted hints. This is a real Location API
   differentiator.

## Table Stakes (from competing tools)

- **foarsitter/locatieserver (Python):** thin client over all 4 Locatieserver
  endpoints. No CLI, no batch, no cache, no coord conversion.
- **Amsterdam/pdok-api-client (Python):** auto-generated OpenAPI client. Same
  shape as foarsitter, also no CLI.
- **nlgeocoder (R, CRAN):** `nl_free`, `nl_suggest`, `nl_lookup`,
  `nl_geocode`, plus a leaflet helper. Aimed at notebooks, not pipelines.
- **QGIS PDOK plugins:** GUI tools (Locator Filter, BAG Geocoder, Processing
  Tool). Inside QGIS only; no headless / scriptable surface outside it.
- **Angular implementation (oldgeogap/angular-locatieserver-geocoder):**
  in-page search bar component with all 4 endpoints. WKT → GeoJSON parsing.
- **Vanilla `curl` recipes / blog posts:** widely used for one-off lookups,
  but never as a workflow primitive.

## Data Layer (local store)

- **Primary entities (stored as `resource_type` rows in `resources`):**
  `adres`, `weg`, `gemeente`, `provincie`, `woonplaats`, `postcode`,
  `perceel`, `hectometerpaal`, `wijk`, `buurt`, `waterschapsgrens`,
  `appartementsrecht` (the Locatieserver `type:` values) — plus an
  `ogc_feature` bucket for raw Location API features with full GeoJSON.
- **Synthetic typed tables (high-gravity, queryable):**
  - `addresses_fts`: FTS5 over `weergavenaam`, `straatnaam`, `postcode`,
    `woonplaatsnaam`, `gemeentenaam`.
  - `gemeenten`: 342 rows, full local gazetteer — `gemeentecode`,
    `gemeentenaam`, `provinciecode`, `provincienaam`, `centroide_ll/_rd`.
  - `provincies`: 12 rows.
  - `lookups`: id → full lookup record (geometry, all fields). The lookup
    response is large; caching it locally is high value.
  - `geocode_cache`: query string → top result(s), keyed by normalized text +
    fq filter so repeated geocodes don't replay calls.
  - `reverse_cache`: lat/lon (or RD X/Y) rounded to ~1m → nearest match,
    keyed by rounded coords + type filter.
- **Sync cursor:** no upstream cursor — Locatieserver is query-driven, not
  feed-driven. `sync` is a no-op or pre-seeds `gemeenten`/`provincies` by
  paging `/free?fq=type:gemeente&rows=50&start=N` until 342 rows arrive.
- **FTS/search:** FTS5 over addresses + a join across all primary types so
  one offline `search "amsterdam"` returns the gemeente, the woonplaats, and
  any cached addresses. After 100 geocodes, the local DB beats the API on
  latency for repeat queries.

## Codebase Intelligence

- **Source:** Manual probing of both OpenAPI specs and the
  `foarsitter/locatieserver` Python client. No DeepWiki query attempted (the
  client repos are tiny and the spec is authoritative).
- **Auth:** None on either API. No env vars needed. `doctor` should still
  probe both base URLs for reachability.
- **Data model:**
  - Locatieserver responses follow Solr shape: `{response: {numFound, start,
    maxScore, docs: [...]}, highlighting?: {...}}`. Each `doc` is a
    flattened map of nullable fields (the 40+ field list shipped in the
    default `fl=`).
  - Coords come as WKT strings: `centroide_ll = "POINT(4.76 52.64)"`
    (lon-then-lat) and `centroide_rd = "POINT(112805.97 517459.55)"` (RD
    X/Y in meters). The CLI MUST parse and re-emit these in machine-friendly
    shape (`{lon, lat}` and `{x, y}` numbers) so downstream tools don't have
    to.
  - Geometries (from `/lookup`) are WKT MULTIPOLYGON / MULTILINESTRING /
    POLYGON. Worth converting to GeoJSON for the user — Locatieserver doesn't
    do this, neither does foarsitter.
  - Location API returns native GeoJSON / JSON-FG features. Direct passthrough
    is fine.
- **Rate limiting:** Not documented and not observed. Behave well anyway:
  default to 1 concurrent request, allow `--concurrency N` for batch, use
  `cliutil.AdaptiveLimiter` for any future sibling sources.
- **Architecture:** Locatieserver is Solr with a thin REST veneer. Most
  power-user features are Solr passthroughs (`fq`, `fl`, `bq`, `qf`, `sort`,
  `df`). The CLI's job is to expose these as ergonomic flags
  (`--type adres`, `--bron BAG`, `--boost-type gemeente=2`) without making
  users learn Solr syntax.

## Source Priority

- Primary: **locatieserver** (`https://api.pdok.nl/bzk/locatieserver/search/v3_1`)
  — official OpenAPI 3.0 spec, four endpoints (`/free`, `/suggest`, `/lookup`,
  `/reverse`). Free, no auth.
- Secondary: **kadaster-location-api**
  (`https://api.pdok.nl/kadaster/location-api/v1`) — official OpenAPI 3.0
  spec, OGC API Features with 14 collections plus a `/search` endpoint. Free,
  no auth.
- **Economics:** Both are free. No tier routing needed.
- **Inversion risk:** None — both have official specs of similar completeness.
  The primary is privileged on user intent (Locatieserver is the established
  workflow), not on spec quality. The Location API is genuinely complementary
  (full geometries, CRS, bbox), not a competing primary.

## Product Thesis

- **Name:** `pdok-location-pp-cli` (binary), `pdok-location` (library slug).
- **Why it should exist:** No headless CLI exists for either PDOK Locatieserver
  or Kadaster Location API. Existing clients (Python, R, JS) are notebook /
  library wrappers without batch, without offline cache, without coord
  conversion, without GeoJSON output, without combining the two services. A
  single Go binary that does all of this — and adds an offline gazetteer of
  Dutch gemeenten, RD↔WGS84 conversion built in, FTS over cached lookups,
  agent-native `--json`/`--select`/`--csv` output — beats every existing
  alternative on every dimension simultaneously.

## Build Priorities

1. **Foundation (Priority 0):** SQLite store with `addresses_fts`,
   `gemeenten`, `provincies`, `lookups` cache, `geocode_cache`,
   `reverse_cache`. Sync command seeds gemeenten + provincies (one-time, 342
   + 12 rows).
2. **Absorbed (Priority 1):** Every Locatieserver endpoint as an ergonomic
   command (`geocode`, `suggest`, `lookup`, `reverse`), every Solr knob
   (`--type`, `--bron`, `--fq`, `--fl`, `--rows`, `--start`, `--sort`,
   `--boost-type`, `--boost-field`, `--df`). Every Location API endpoint
   (`collections list`, `collections describe`, `features list`,
   `features get`, `search`). GeoJSON output via `--geojson`.
3. **Transcendence (Priority 2):** Batch geocoding from CSV/stdin; offline
   gazetteer (`gemeente get amsterdam`, `provincie list`); RD↔WGS84 coord
   conversion (`convert rd-to-ll 121200 488000`); cross-source `nearest`
   (geocode + lookup + reverse in one call); local FTS `search` over cached
   data; `validate-postcode` (Dutch postcode shape); `inside <bbox>` filter;
   `top` (find best single match for ambiguous queries with confidence
   threshold).
