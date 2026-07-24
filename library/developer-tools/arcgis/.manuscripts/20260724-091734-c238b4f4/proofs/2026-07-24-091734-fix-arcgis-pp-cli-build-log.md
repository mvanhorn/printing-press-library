Manifest transcendence rows: 6 planned, 6 built. Phase 3 complete.

## Built
- Core: arcgis protocol client (internal/arcgis) — arbitrary-URL service/layer metadata, count, /query, returnIdsOnly.
- Extraction engine: resultOffset paging (exceededTransferLimit loop, deterministic orderByFields), OID-array chunking fallback, recursive bbox tiling, --limit. GeoJSON (esri->GeoJSON incl. multi-ring polygon grouping), CSV, JSONL, raw JSON.
- Commands: discover, fields, count, query, locate (point-in-polygon + CSV batch), audit (RE field signals), diff (change detection vs synced baseline), stats (outStatistics), sync (URL->SQLite via `features` view), sql (read-only offline SQL).
- Robustness: HTTP-200 {"error":{}} bodies now surfaced (ArcGIS returns 200 on invalid layer/token/param).
- Optional ARCGIS_TOKEN passthrough on every request.

## Verified live (FEMA NFHL + spot checks)
fields/count/query(csv,geojson polygon)/discover/stats(group-by)/locate(matched containing zone)/sync/sql(group-by view)/diff(honest 0-change)/audit — all correct.

## Removed generated base-URL demo commands
tail, analytics, workflow, api, catalog(promoted), features(promoted) — they targeted the nominal sample base_url and don't fit an arbitrary-URL protocol tool. Replaced generated sync with URL-taking sync.

## Generator bug (retro)
auth.type:none + env_vars emitted undefined `authConfigured` in doctor.go and routed the optional token to authEnvRequiredMissing (ERROR). Fixed in place.
