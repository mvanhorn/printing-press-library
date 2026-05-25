# PDOK Location CLI — Absorb Manifest

Combo CLI absorbing PDOK Locatieserver (Solr-based) and Kadaster Location API
(OGC API Features). Primary source: Locatieserver. Secondary: Location API.
Both free, no auth, Netherlands-only.

Status legend: shipping rows are unmarked; rows that ship as honest stubs are
explicitly tagged `Status: (stub)`.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Free-text geocoding | Locatieserver `/free` | `geocode <query>` with `--type`, `--bron`, `--rows`, `--start`, `--sort`, `--min-score` | `--json`/`--csv`/`--select`, score threshold, ergonomic flags around Solr knobs |
| 2 | Autocomplete suggestions | Locatieserver `/suggest` | `suggest <prefix>` with `--type`, `--rows`, `--lat/--lon` for distance bias | `--json`, highlight tags stripped by default, `--keep-highlights` to retain |
| 3 | Lookup by ID | Locatieserver `/lookup` | `lookup <id>` | Caches lookup in local SQLite; auto-converts WKT geometry to GeoJSON with `--geojson` |
| 4 | Reverse geocoding (WGS84) | Locatieserver `/reverse` | `reverse --lat --lon` with `--type`, `--distance`, `--rows` | `--json`/`--csv`, `afstand` field surfaced as `distance_m` |
| 5 | Reverse geocoding (RD coords) | Locatieserver `/reverse` | `reverse --rd-x --rd-y` (same command, RD input) | Convenience for Dutch coord system |
| 6 | Solr filter query passthrough | Locatieserver | `--fq` flag accepted on every endpoint command | Power users keep Solr access |
| 7 | Field list selection | Locatieserver | `--fl <list>` plus `--all-fields` shortcut | Plus standard `--select` for JSON output |
| 8 | Boost-query syntax | Locatieserver | `--boost-type adres=2` / `--boost-field straatnaam=3` | Convenience parser, no Solr literal required |
| 9 | Default search field control | Locatieserver `df` | `--df <field>` | Passthrough |
| 10 | List OGC collections | Location API `/collections` | `collections list` | Table view, `--json` for structured output |
| 11 | Describe one collection | Location API `/collections/{id}` | `collections describe <name>` | Schema, link summary, item count if available |
| 12 | Get a feature by ID | Location API `/collections/{c}/items/{id}` | `features get <collection> <id> [--crs <name>]` | Friendly CRS names (`wgs84`, `rd`, `webmerc`, `etrs89`) → URIs |
| 13 | List features in a collection | Location API `/collections/{c}/items` | `features list <collection>` with `--bbox`, `--limit`, `--crs`, `--filter` | Pagination loop, `--csv` flatten |
| 14 | Cross-collection text search | Location API `/search` | `ogc-search <query>` with `--collections`, `--bbox`, `--bbox-crs`, `--limit` | GeoJSON / JSON-FG passthrough |
| 15 | Format selection | Location API | `--format json\|jsonfg\|html` on OGC commands | Sensible defaults |
| 16 | API conformance probe | Location API `/conformance` | `doctor` includes a conformance check | Health probe surfaces both APIs |
| 17 | CRS friendly-name resolver | Location API | Internal mapping table `wgs84`→`OGC:CRS84` etc., used by every OGC flag | Hides the URI surface |
| 18 | Postcode lookup helper | Locatieserver `/free?fq=type:postcode` | `postcode lookup <pc>` | Validates shape (4 digits + 2 letters), canonicalises spacing |
| 19 | Adres type-scoped list | Locatieserver `/free?fq=type:adres` | `adres list --postcode <pc>` / `adres list --gemeente <name>` | Type-scoped helpers under one parent command |
| 20 | Weg type-scoped commands | Locatieserver `/free?fq=type:weg` | `weg list --gemeente <name>` | Type-scoped roads helper |
| 21 | Perceel type-scoped commands | Locatieserver `/free?fq=type:perceel` | `perceel list --gemeente <name>` | Type-scoped parcels helper |
| 22 | Woonplaats list | Locatieserver `/free?fq=type:woonplaats` | `woonplaats list [--gemeente <name>]` | Type-scoped |
| 23 | Score threshold filtering | Locatieserver `score` field | `--min-score <n>` on every search command | Pipeline-friendly precision control |
| 24 | Pagination (rows/start) | Locatieserver | `--rows N --start M` | Standard, with `--rows` clamp to 50 per docs |
| 25 | Sort control | Locatieserver `sort` | `--sort "<expr>"` (default Solr expression) | Passthrough |
| 26 | WKT centroid auto-parsing | Locatieserver | Every `--json` output exposes `centroide_ll: {lon,lat}` and `centroide_rd: {x,y}` numbers | Raw WKT also kept under `_wkt` |
| 27 | Highlighting strip | Locatieserver suggest | Strip `<b>...</b>` markup unless `--keep-highlights` | Default sensible |
| 28 | Wrap-it-to-Postcode-NL pattern (gemeente of-postcode) | Locatieserver | `gemeente of-postcode <pc>` | Tiny shortcut for forms |
| 29 | Per-call rate limiting | (none documented) | `cliutil.AdaptiveLimiter` for all hand-written client code; default 5 RPS, retries 429 | Defensive |
| 30 | Health probe | Both APIs | `doctor` reaches both base URLs + conformance | Standard |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Batch CSV geocode | `batch geocode <file.csv> --address-col street [--out result.csv]` | 10/10 | hand-code | Reads CSV, hits Locatieserver `/free` per row with `cliutil.AdaptiveLimiter`, writes `geocode_cache` to SQLite, emits new CSV with `lat,lon,rd_x,rd_y,score,match_type,match_id,error` columns | Brief Top Workflow #1; competitors (foarsitter, nlgeocoder, QGIS) lack batch |
| 2 | Suggest→lookup one-shot | `resolve <text> [--geojson]` | 10/10 | hand-code | Calls `/suggest` top-1, feeds id into `/lookup`, parses WKT into GeoJSON, caches lookup in SQLite | Brief Top Workflow #2; Codebase Intelligence; Persona Sven |
| 3 | Cross-source nearest | `nearest --lat <> --lon <>` (or `--rd-x --rd-y`) | 10/10 | hand-code | Fans out `/reverse` across types `adres,perceel,hectometerpaal,weg`, joins with offline `gemeenten`/`provincies`, returns one record with all four neighbours + admin context | Brief Top Workflow #3; Persona Marieke |
| 4 | RD ↔ WGS84 convert | `convert rd-to-ll <x> <y>` / `convert ll-to-rd <lon> <lat>` | 9/10 | hand-code | Pure-math EPSG:28992 ↔ EPSG:4326 transform in Go (no API call); supports stdin pairs for bulk | Brief Codebase Intelligence; Personas Lotte + Marieke |
| 5 | WKT ↔ GeoJSON convert | `convert wkt-to-geojson <wkt>` / `convert geojson-to-wkt <stdin>` | 9/10 | hand-code | Parse WKT POINT/POLYGON/MULTIPOLYGON/MULTILINESTRING offline → emit GeoJSON, or reverse | Brief Codebase Intelligence; used internally by `resolve`/`lookup` |
| 6 | Offline gemeente/provincie gazetteer | `gemeente get <name>` / `gemeente list --provincie <name>` / `provincie list` | 9/10 | hand-code | One-time `sync` seeds 342 gemeenten + 12 provincies; subsequent calls hit local SQLite, FTS-backed fuzzy name match | Brief Data Layer; Personas Lotte, Joris |
| 7 | Local FTS over cached results | `search <text> [--type adres,gemeente,...]` | 8/10 | hand-code | FTS5 over `addresses_fts` + `gemeenten` + `lookups`; returns local hits with provenance; `--online` falls back to `/free` if zero hits | Brief Data Layer; Personas Lotte, Sven |
| 8 | Bbox multi-collection CSV dump | `features in-bbox --bbox <x1,y1,x2,y2> --collections adres,perceel [--csv]` | 8/10 | hand-code | Iterates Location API `/collections/{c}/items?bbox=...` per requested collection, paginates, flattens to wide CSV with stable column order | Brief Top Workflow #6 (bbox is Location API only); Persona Joris |
| 9 | Confidence-gated best match | `top <query> --min-score <n> [--require-type adres]` | 7/10 | hand-code | `/free` top-1, exits non-zero if `score<min` or type filter unmet; stdout is the single match | Persona Lotte (CI / pipelines); used internally by batch |
| 10 | Perceel by kadastrale aanduiding | `perceel lookup --aanduiding "AMR03 N 1234"` | 8/10 | hand-code | Parser splits aanduiding into kadastrale gemeente / sectie / perceelnummer, queries Location API `/collections/perceel/items` with those fields, returns GeoJSON parcel | Brief Data Layer (BRK/DKK); Persona Joris |
| 11 | Gemeente containing point | `gemeente of-point --lat <> --lon <>` (or `--rd-x --rd-y`) | 8/10 | hand-code | Cached `gemeenten` centroids + `/reverse?type=gemeente` fallback; returns gemeente + provincie holding the point | Persona Marieke |

**Total transcendence rows: 11 (all `hand-code`).**
**Total absorbed rows: 30.**
**Total: 41 features.**

No stubs. Everything ships fully implemented.
