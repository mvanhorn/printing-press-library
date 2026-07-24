# ArcGIS REST CLI — Absorb Manifest

## Absorbed (match or beat every incumbent)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Dump a layer URL to GeoJSON | pyesridump esri2geojson | arcgis-pp-cli query <url> --format geojson | Also CSV/JSONL/table, --json, offline store |
| 2 | Automatic resultOffset pagination | pyesridump | (behavior in arcgis-pp-cli query) loops resultOffset until exceededTransferLimit=false | Deterministic orderByFields, correct end-of-page detection |
| 3 | OID-array chunking fallback | pyesridump | (behavior in arcgis-pp-cli query) returnIdsOnly then batch by OBJECTID when server lacks pagination | Auto-selected; --pager flag to force |
| 4 | Newline-delimited JSONL streaming | pyesridump --jsonlines | (behavior in arcgis-pp-cli query --format jsonl) | Streams for piping into loaders |
| 5 | where= attribute filter | all incumbents | (behavior in arcgis-pp-cli query --where) | |
| 6 | outFields selection | all incumbents | (behavior in arcgis-pp-cli query --fields) | Defaults to *; --fields narrows |
| 7 | Spatial bbox / envelope filter | ogr2ogr, raw API | arcgis-pp-cli query <url> --bbox xmin,ymin,xmax,ymax | esriGeometryEnvelope + intersects wired |
| 8 | Read layer metadata (fields/geom) | ogrinfo, raw API | arcgis-pp-cli fields <url> | Clean field table + types + maxRecordCount |
| 9 | Enumerate services in a directory | gis_scraper (recursive) | arcgis-pp-cli discover <services-dir-url> | Recursive walk to layers, clean JSON/table |
| 10 | outSR reprojection on pull | raw API, ogr2ogr | (behavior in arcgis-pp-cli query --out-sr) | Default 4326 lon/lat for loaders |
| 11 | Count-only without pulling rows | raw API | arcgis-pp-cli count <url> --where ... | returnCountOnly passthrough |
| 12 | Layer -> local geodata store | esridumpgdf (GeoDataFrame) | arcgis-pp-cli sync <url> | SQLite mirror, not an in-memory frame |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Point-in-polygon signal binding | locate --point lon,lat <layer-url> | hand-code | Binds a coordinate to its containing parcel via a single intersects query — the RE signal-to-parcel bind that fuzzy address matching gets wrong. Accepts a CSV of points for batch binding. | Use to attach a lat/lng distress signal (obit, court record) to the parcel polygon that contains it. Do NOT use for attribute filtering; use 'query --where'. |
| 2 | Field audit across a service | audit <service-url> | hand-code | Pulls every layer's schema and flags high-value fields present/absent (owner, mailing address, homestead/exemption, last-sale) — the "what are we dropping" audit, impossible without walking + comparing schemas locally. | Use to see which distress-signal fields a county's layers actually expose before writing a load. |
| 3 | Owner/attribute change detection | diff <layer-url> --key OBJECTID --track OWNER | hand-code | Re-queries a layer and diffs a tracked field against the last local sync — surfaces ownership changes (recent transfers = motivation) with no deed scraper. Requires the local SQLite baseline. | Use to detect changed owners/values between two pulls. Requires a prior 'sync'. |
| 4 | Bbox tiling for capped layers | query <url> --tile | hand-code | Auto-subdivides the query envelope into quadrants when a layer exceeds transfer limit AND lacks pagination — pulls layers the incumbents silently truncate. | Use on large layers that return exceededTransferLimit but do not support resultOffset paging. |
| 5 | Stats passthrough (no full pull) | stats <url> --group-by <field> --out count,sum:<f> | hand-code | Server-side outStatistics/groupByFieldsForStatistics — get counts/sums per category without downloading every row. | Use for aggregates (parcels per land-use code, acreage by owner) without pulling the whole layer. |
| 6 | Offline SQL over a synced layer | sql "SELECT ..." | spec-emits | Query a synced layer's attributes with SQLite/FTS after one pull — no repeated API hits, composable with jq. | Use to slice a synced layer locally. Requires a prior 'sync'. |

## Stubs
- None. Optional ArcGIS token (`--token` / ARCGIS_TOKEN) is a thin passthrough on every request, wired but defaulting to none; not a stub.
