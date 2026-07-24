# ArcGIS REST CLI Brief

## API Identity
- Domain: ArcGIS REST API — the uniform query protocol exposed by every Esri ArcGIS Server / ArcGIS Online FeatureServer & MapServer. Endpoints are arbitrary per-host URLs; the *operations* are uniform (service metadata, layer metadata, /query).
- Users: GIS analysts, data engineers, civic-data scrapers, and (our case) real-estate lead-gen pipelines pulling county parcel + distress layers.
- Data profile: layers of features (polygons/points/lines) with typed attribute tables + geometry. Paginated. maxRecordCount caps per-request rows (commonly 1000-2000).

## Reachability Risk
- None. Live probe against FEMA NFHL FeatureServer returned service metadata, layer fields (maxRecordCount 2000, supportsPagination true), count-only (66), and a paged feature with exceededTransferLimit=true. Public endpoints, no auth.

## Top Workflows
1. Point at any layer URL and pull ALL features to GeoJSON/CSV, correctly paginated (the esridump job).
2. Discover: point at a /rest/services directory or a service and enumerate services -> layers -> fields.
3. Introspect one layer's schema (fields, types, geometry, maxRecordCount, capabilities) before pulling.
4. Spatial query: pull only features intersecting a bbox or a point (point-in-polygon).
5. Sync a layer into a local SQLite store, then query/search it offline and re-pull to detect change.

## Table Stakes (from incumbents)
- Full-layer dump with automatic pagination (resultOffset/resultRecordCount) + OID-array chunking fallback (pyesridump, esri-dump).
- GeoJSON output + newline-delimited JSONL streaming (pyesridump --jsonlines).
- where= filter, outFields selection (all incumbents, ogr2ogr).
- Spatial filter: geometry + geometryType + spatialRel intersects (ogr2ogr, raw API).
- Robust against servers lacking pagination (fall back to OID chunking).

## Data Layer
- Primary entities: `service` (url, type, layer count), `layer` (url, name, geometryType, maxRecordCount, fields json), `feature` (layer_url, oid, attributes json, geometry json).
- Sync cursor: OBJECTID high-water mark per layer.
- FTS/search: over feature attributes json.

## Competitors / Absorb Sources
- openaddresses/pyesridump (Python, esri2geojson CLI) — THE incumbent. Pagination + OID fallback, --jsonlines. GitHub, widely used.
- stiles/ezesri (Python) — API+CLI+web, pagination + filtering.
- openaddresses/esri-dump (Node) — library.
- GDAL/OGR ESRIJSON driver (ogr2ogr/ogrinfo) — heavyweight, needs GDAL, awkward URL syntax.
- MatzFan/gis_scraper (Ruby) — recursive MapServer -> GeoJSON/PostGIS.
- wchatx/esridumpgdf (Python) — to GeoDataFrame.

## User Vision (Jake / RE Reset LeadForge)
Pull county parcel layers + code-enforcement/permit/tax-delinquent layers, and enrich parcels against overlay layers (FEMA flood). Uniform client so onboarding a new county is config, not a new scraper. Optional ArcGIS token param later; v1 is public endpoints only. Output must pipe straight into a data loader (GeoJSON + CSV).

## Product Thesis
- Name: arcgis-pp-cli
- Why it should exist: every incumbent dumps ONE layer to GeoJSON and stops. None discover a server, introspect schemas cleanly, output CSV for loaders, keep an offline SQLite mirror, or do spatial signal-binding (point -> containing parcel). This CLI turns "write a scraper per county" into "point the CLI at a URL," and adds the offline + agent-native + spatial-join layer the RE pipeline actually needs.

## Build Priorities
1. Core query engine: paginate correctly (resultOffset + exceededTransferLimit loop; OID-array chunking fallback; deterministic orderByFields), GeoJSON/CSV/JSONL out.
2. Discovery + schema introspection (service walk, layer fields).
3. Spatial: bbox + point-in-polygon intersects.
4. Offline SQLite store + sync + change-diff.
5. Stats passthrough (outStatistics count/sum/group-by).
