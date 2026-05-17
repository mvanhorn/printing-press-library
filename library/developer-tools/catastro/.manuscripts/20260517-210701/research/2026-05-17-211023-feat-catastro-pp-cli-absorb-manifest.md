# Absorb Manifest — catastro

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List provinces | pycatastro.ConsultaProvincia / OVC ObtenerProvincias | `catastro provinces list` | `--json` `--select`; cached locally; FTS-searchable |
| 2 | List municipalities of a province | pycatastro.ConsultaMunicipio | `catastro municipalities list --provincia MADRID` | Cache; INE codes; `--like` regex filter |
| 3 | List streets of a municipality | pycatastro.ConsultaVia / ObtenerCallejero | `catastro streets list --provincia MADRID --municipio MADRID` | FTS5 over `streets.nombre` |
| 4 | List numbers on a street | pycatastro.ConsultaNumero / ObtenerNumerero | `catastro numbers list --provincia ... --municipio ... --via ...` | Sortable, offline |
| 5 | Property by address (DNPLOC) | pycatastro.Consulta_DNPLOC | `catastro property by-address --provincia MADRID --municipio MADRID --calle ALCALA --numero 1` | Pretty card + `--json` + cache |
| 6 | Property by cadastral reference (DNPRC) | pycatastro.Consulta_DNPRC | `catastro property show <RC>` | Adds `--parts` flag for sub-parts table |
| 7 | Property by polygon+plot (rural) | pycatastro.Consulta_DNPPP | `catastro rural show --provincia X --municipio Y --poligono N --parcela M` | `--json` + cache |
| 8 | Coordinates → cadastral reference | pycatastro.Consulta_RCCOOR | `catastro geocode reverse --lon X --lat Y` | EPSG selectable (`--srs`) |
| 9 | Coordinates → nearby RCs (within ~50 m) | pycatastro.Consulta_RCCOOR_Distancia | `catastro geocode nearby --lon X --lat Y` | Sorted by distance |
| 10 | RC → coordinates | pycatastro.Consulta_CPMRC | `catastro geocode forward <RC>` | Multi-SRS output |
| 11 | INE-codes variants of #2/#5/#6/#7 | pycatastro.*_Codigos | Single `--codes` flag toggles INE input | Unified UX |
| 12 | Bulk parcels of a municipality (ATOM) | catr_atom_get_parcels (R) / cidownloader | `catastro sync --municipio <code> --kind parcels` | Ingests to SQLite, FTS-ready |
| 13 | Bulk addresses of a municipality | catr_atom_get_address (R) | `catastro sync --municipio <code> --kind addresses` | Same store |
| 14 | Bulk buildings of a municipality | catr_atom_get_buildings (R) | `catastro sync --municipio <code> --kind buildings` | Same store |
| 15 | WFS parcels by bbox | catr_wfs_get_parcels_bbox (R) | `catastro export parcels --bbox X1,Y1,X2,Y2 --format geojson` | Direct GeoJSON, no QGIS |
| 16 | WFS parcels by RC | catr_wfs_get_parcels_parcel (R) | `catastro export parcels --rc <RC> --format geojson` | GeoJSON / GeoPackage |
| 17 | WFS parcels by neighbor | catr_wfs_get_parcels_neigh_parcel | `catastro export parcels --rc <RC> --include-neighbors` | Single command |
| 18 | WFS parcels by zoning | catr_wfs_get_parcels_zoning | `catastro export parcels --zoning <code>` | |
| 19 | WFS addresses (bbox / RC / codvia / postal) | catr_wfs_get_address_* | `catastro export addresses --bbox ... / --rc ... / --codvia ... / --postalcode ...` | Unified flags |
| 20 | WFS buildings (bbox / RC) | catr_wfs_get_buildings_* | `catastro export buildings --bbox ... / --rc ...` | |
| 21 | WMS map render | catr_wms_get_layer (R) | `catastro map render --bbox X1,Y1,X2,Y2 --layer CP.CadastralParcel --out map.png` | Per-layer, PNG |
| 22 | Coordinates → municipality code | catr_get_code_from_coords (R) | `catastro geocode municipality --lon X --lat Y` | Helper |
| 23 | Local cache management | catr_set_cache_dir / catr_clear_cache (R) | `catastro cache path` / `catastro cache clear` | SQLite path + size info |
| 24 | INE municipality code lookup by name | catr_ovc_get_cod_munic (R) | `catastro municipalities show <name>` | |
| 25 | SRS / EPSG catalog | catr_srs_values (R) | `catastro srs list` | Names + extents |
| 26 | INSPIRE WMS GetCapabilities introspection | (none) | `catastro wms layers` | Lists 6 available WMS layers |

Every row in this table is a feature we MUST build. Each works offline where possible, supports `--json --select --dry-run`, returns typed exit codes, and is exposed through the local SQLite store when the data is persistable.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Reconcile local parcel table against Catastro | `catastro reconcile <file> --rc-col rc --fields surface,use,year,address` | 9/10 | Reads local CSV/SQLite, for each RC calls Consulta_DNPRC, diffs declared columns, writes typed diff report + non-zero exit on drift | Brief: "No tool reconciles an external table against Catastro"; user's CLAUDE.md flags street mappings as unreliable; persona Rosa |
| 2 | Detect parcel changes between syncs | `catastro watch --kind parcels --municipio <code> --since <date>` | 8/10 | Re-syncs municipality, joins `parcels` vs `parcel_history`, emits per-RC delta of surface/use/built_year/address | Brief: "No tool detects changes between syncs"; data layer declares `parcel_history` |
| 3 | Polygon-clipped parcel export | `catastro export parcels --polygon ./geom.geojson --to ./out.gpkg --epsg 25830` | 8/10 | Point-in-polygon over local `parcels.geom_wkt`, reprojects, writes GeoJSON/GeoPackage/SHP | Brief: cidownloader/QGIS only per-municipality; persona Pablo |
| 4 | Pipe-friendly bulk enrichment | `cat refs.txt \| catastro enrich --json` / `catastro enrich --addresses < addrs.csv` | 9/10 | Stdin lines (RCs or addresses), throttled DNPRC/DNPLOC fan-out with on-disk cache, JSONL out | Brief: "No tool supports bulk-RC enrichment piped through stdin"; persona Laura |
| 5 | Expediente bundle for one RC | `catastro report <RC> --neighbors --to ./bundle/` | 7/10 | DNPRC + RCCOOR + RCCOOR_Distancia + WFS bbox + WMS GetMap composed into directory (data.json, neighbors.geojson, map.png, parts.csv) | Persona Iván workflow crosses 4 tools |
| 6 | Stale local data planning | `catastro stale --older-than 180d` | 6/10 | SQL over `parcel_history.sync_at`, lists RCs/municipalities past TTL | Brief: data-layer sync cursor; Rosa weekly |
| 7 | Nearest-neighbor parcels for an RC | `catastro neighbors <RC> --radius 50 --out gpkg` | 7/10 | RC → centroid via CPMRC, RCCOOR_Distancia for 50m radius, joined with DNPRC, exported | Persona Iván vecinas every expediente |
| 8 | FTS over local store | `catastro search "calle alcalá" --municipio 28079 --json` | 7/10 | SQLite FTS5 over `parcels(rc, address, use)` + `streets(nombre, municipality)` | Brief Data Layer FTS; no incumbent has it |
| 9 | Local sync coverage overview | `catastro coverage --provincia 28` | 5/10 | Aggregates `municipalities` left-joined to `*_history` for last_sync + counts | Persona Pablo weekly check |
| 10 | Area aggregate analysis | `catastro analyze-area --polygon ./geom.geojson` | 7/10 | Point-in-polygon + SQL aggregate: count, total ha, use histogram, mean year, distinct RCs | Brief: cross-parcel aggregations not feasible against live API |
| 11 | MCP server for agents | `catastro mcp serve` | 8/10 | Mirrors top commands as MCP tools (the Cobra-tree walker), JSON, deterministic exits, stdio | Brief: "No MCP server exists for Catastro at all (green field)" |

Total: **26 absorbed + 11 transcendence = 37 features.**

## Stub list

No features are planned as stubs. All 37 ship as fully implemented.
