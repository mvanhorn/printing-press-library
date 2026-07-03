# forMaps CLI Brief

## API Identity
- Domain: Italian cadastral cartography (catasto) ↔ WGS84 GPS coordinates
- Users: real estate professionals, geomatic surveyors (geometri), civil engineers, GIS analysts, agentic tools that need to resolve cadastral references
- Data profile: ~85M parcels (particelle) across ~8000 comuni, indexed by codice belfiore + foglio + particella (optional sezione)

## Reachability Risk
- **Low**. AdE WMS/ajax endpoints have been publicly available for years and are documented in the official FAQ. ondata/dati_catastali Parquet files are mirrored on GitHub (no auth, no rate limits). The community has used both surfaces stably since 2024–2025.

## Top Workflows
1. **GPS → cadastral**: given lat/lon, find which parcel a point falls inside (comune, foglio, particella). Real estate due diligence, "what's on this point".
2. **Cadastral → GPS**: given comune/foglio/particella, return the WGS84 centroid of that parcel. Mapping a paper title onto a digital map.
3. **Batch reverse-geocode**: process a CSV/TSV of coordinate pairs to enrich with cadastral refs. Bulk title research, surveying datasets.
4. **Resolve human comune name → codice belfiore**: "Milano" → "F205". Required as input to the cadastral→GPS path.
5. **Inspect / preview a parcel**: get its boundary polygon (GeoJSON) and metadata for embedding in maps.

## Data Layer
- Primary entities:
  - `parcels` — id (comune+sezione+foglio+particella), comune_code, comune_name, sezione, foglio, particella, lon, lat (centroid), region, source_file, last_synced
  - `comuni` — codice_belfiore, name, province_code, region (~8000 rows, bundled)
  - `lookups` — cached results of forward/reverse calls (the API result for a given input), for offline replay
- Sync cursor: per-region Parquet ETag / last-modified from GitHub (so `sync` only re-downloads changed regions)
- FTS/search: FTS5 over comune name + province for fuzzy comune lookup; SQL queries over parcel attributes for offline reverse lookups once synced.

## Codebase Intelligence
- Source: research on `ondata/dati_catastali`, `enricofer/catasto`, `pigreco/workshop-estate-gis-2021`, Andrea Borruso's Medium write-up
- Auth: **none** — both surfaces are anonymous public
- Data model:
  - AdE ajax: `?op=getDatiOggetto&lon=X&lat=Y` returns JSON object with `comune`, `foglio`, `particella`, `sezione`, raw identifier string like `IT.AGE.PLA.G273_003400.1298`
  - ondata Parquet: one file per Italian region (e.g. `19_Sicilia.parquet`, `12_Lazio.parquet`); columns include `comune` (codice belfiore), `sezione`, `foglio`, `particella`, `x`, `y` (integer microdegrees, divide by 1_000_000 → decimal degrees in EPSG:4258 ≈ WGS84)
  - `index.parquet` maps codice_belfiore → region file
- Rate limiting: not documented; community projects throttle politely (~10 req/s). We default to 5 req/s to be a good citizen.
- Architecture: ondata regenerates Parquet files quarterly from AdE WFS bulk dumps. We treat them as a CDN of pre-computed centroids.

## User Vision
The user runs a real estate / surveying practice and needs a scriptable bridge between paper cadastral references and digital maps. Both directions matter equally. Offline-first preferred (sync once, query forever). Single-binary distribution so it can be embedded in tooling that runs on field laptops.

## Product Thesis
- **Name**: `catasto-pp-cli` (binary), namespace "formaps"
- **Why it should exist**: The only Italian cadastral CLI that does both directions, works offline after sync, returns agent-native JSON, and ships as a single Go binary with no Python/QGIS dependency. Existing tooling is either commercial SaaS (catastomappe.it, openapi.com, forMaps.it/STIMATRIX) or requires a Python+DuckDB+QGIS stack. Free, scriptable, agent-friendly.

## Build Priorities
1. **Foundation**: SQLite store + comune lookup table (bundled CSV → embedded Go data) + Parquet reader for ondata files
2. **Core commands**: `gps` (forward), `cadastral` (reverse), `comune` (name↔code resolver), `sync` (cache Parquet files locally)
3. **Transcendence**: batch reverse-geocode, parcel preview with GeoJSON polygon, FTS comune search, offline-replayable cache
4. **MCP exposure**: every command read-only by default (`mcp:read-only`), since these are pure lookups
