# Novel features brainstorm — catastro

## Customer model

### Persona 1: Rosa, gerente de la urbanización Río Cedena (residential property manager)

**Today (without this CLI):** Rosa heredó una hoja de cálculo con 602 parcelas con dirección, número, calle y un campo "RC" que se rellenó a mano hace años. Cuando la Junta le pide "el padrón actualizado", abre 20 pestañas de sedecatastro.gob.es, mete una RC, espera el formulario ASP.NET, copia superficie/uso/año/dirección, cierra. Su CLAUDE.md ya advierte que los mapeos calle↔parcela "no son fiables". No sabe qué parcelas tienen RC mal, ni qué propietario reformó (cambió superficie/uso) sin avisar.

**Weekly ritual:** Un viernes al mes verifica un lote de ~20 parcelas contra Catastro. Dos horas, siempre se interrumpe porque no hay forma de marcar "ya verificadas esta semana".

**Frustration:** No puede preguntar "¿qué parcelas de mi tabla han cambiado en Catastro desde la última sincro?" — cada lookup es atómico, no hay diff.

### Persona 2: Pablo, GIS analyst en una geomática que prepara mapas para ayuntamientos

**Today (without this CLI):** Para "todas las parcelas del polígono X" abre QGIS con `Spanish_Inspire_Catastral_Downloader`, descarga el municipio entero como GeoPackage (10-200 MB), lo importa, lo recorta, lo exporta. Si cambian el bbox, repite todo. Para varios municipios encadena `cidownloader` desde la terminal con `ogr2ogr`.

**Weekly ritual:** 2-3 veces por semana descarga un municipio nuevo o re-recorta uno anterior. Mantiene un `~/catastro/` con GeoPackages caducados (ATOM se refresca 2x/año).

**Frustration:** Recortar por bbox/polígono requiere descargar todo el municipio aunque solo necesite 50 parcelas; no hay un solo binario que combine bulk + WFS + recorte + reproyección.

### Persona 3: Laura, due-diligence scout en una asesora inmobiliaria

**Today (without this CLI):** Recibe 80-300 direcciones (Excel) y debe devolver para cada una: RC, superficie, uso, año, building parts. Abre cada dirección en sedecatastro, copia a Excel. Si solo hay coords (catastros con pleitos, fincas rurales), pestaña de cartografía y clic.

**Weekly ritual:** 1-2 lotes (50-300 direcciones) semanales para due-diligence o tasaciones. Cuello de botella: enriquecimiento, no análisis.

**Frustration:** No existe `cat addresses.csv | enrich > out.jsonl` que devuelva RC + property card por fila. A mano o becario con pycatastro de un solo uso.

### Persona 4: Iván, técnico de expedientes de segregación/agrupación

**Today (without this CLI):** Para un expediente necesita: RC origen, RCs vecinas, geometrías EPSG:25830 (CAD), subparcelas (uso/superficie/coef_propiedad por planta), PNG rotulado para informe. Hoy: pycatastro DNPRC + R CatastRo WFS + QGIS reproject + screenshot del visor.

**Weekly ritual:** 3-5 expedientes/semana, cada uno una RC central + 4-15 vecinas + subparcelas. 30-45 min de fontanería antes del trabajo técnico real.

**Frustration:** No hay un solo comando que dado un RC devuelva parcela + vecinas + building-parts + PNG en una pasada.

## Candidates (pre-cut)

(a) Persona-driven: C1 reconcile, C2 watch, C3 export polygon, C4 enrich stdin, C5 report bundle.
(b) Service-specific: C6 parts (DNPRC sub-parts), C7 rural locate (KILL — wrapper), C8 tile RC, C9 valuation (KILL — not in API), C16 mcp serve.
(c) Cross-entity: C10 stale, C11 municipality summary (KILL — dup of C14), C12 neighbors, C13 search FTS, C14 coverage, C15 analyze-area.

## Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Reconcile local parcel table against Catastro | `catastro reconcile <file> --rc-col rc --fields surface,use,year,address` | 9/10 | Reads local CSV/SQLite, for each RC calls Consulta_DNPRC, diffs declared columns, writes typed diff report + non-zero exit on drift | Brief: "No tool reconciles an external table against Catastro"; user's CLAUDE.md flags street mappings as unreliable; persona Rosa |
| 2 | Detect parcel changes between syncs | `catastro watch --kind parcels --municipio <code> --since <date>` | 8/10 | Re-syncs municipality, joins `parcels` vs `parcel_history`, emits per-RC delta of surface/use/built_year/address | Brief: "No tool detects changes between syncs"; data layer declares `parcel_history` |
| 3 | Polygon-clipped parcel export | `catastro export parcels --polygon ./geom.geojson --to ./out.gpkg --epsg 25830` | 8/10 | Point-in-polygon over local `parcels.geom_wkt`, reprojects, writes GeoJSON/GeoPackage/SHP | Brief: cidownloader/QGIS only do per-municipality; persona Pablo |
| 4 | Pipe-friendly bulk enrichment | `cat refs.txt \| catastro enrich --json` | 9/10 | Stdin lines (RCs/addresses), throttled DNPRC/DNPLOC fan-out with on-disk cache, JSONL out | Brief: "No tool supports bulk-RC enrichment piped through stdin"; persona Laura |
| 5 | Expediente bundle for one RC | `catastro report <RC> --neighbors --to ./bundle/` | 7/10 | DNPRC + RCCOOR + RCCOOR_Distancia + WFS bbox + WMS GetMap composed into directory (data.json, neighbors.geojson, map.png, parts.csv) | Persona Iván workflow crosses 4 tools; no incumbent bundles them |
| 6 | Stale local data planning | `catastro stale --older-than 180d` | 6/10 | SQL over `parcel_history.sync_at`, lists RCs/municipalities past TTL | Brief models `parcel_history`; Rosa weekly planning |
| 7 | Nearest-neighbor parcels for an RC | `catastro neighbors <RC> --radius 50 --out gpkg` | 7/10 | RC → centroid via CPMRC, then RCCOOR_Distancia for 50m radius, joined with DNPRC, exported | Persona Iván vecinas every expediente; wrappers force scripting |
| 8 | FTS over local store | `catastro search "calle alcalá" --municipio 28079 --json` | 7/10 | SQLite FTS5 over `parcels(rc, address, use)` + `streets(nombre, municipality)` | Brief Data Layer lists FTS; pycatastro/CatastRo no local search |
| 9 | Local sync coverage overview | `catastro coverage --provincia 28` | 5/10 | Aggregates `municipalities` left-joined to per-kind `*_history` for last_sync + counts | Persona Pablo weekly check |
| 10 | Area aggregate analysis | `catastro analyze-area --polygon ./geom.geojson` | 7/10 | Point-in-polygon + SQL aggregate over hits: count, total ha, use histogram, mean year, distinct RCs | Brief: cross-parcel aggregations not feasible against live API |
| 11 | MCP server for agents | `catastro mcp serve` | 8/10 | Wraps top ~15 commands as MCP tools, JSON, deterministic exits, stdio | Brief: "No MCP server exists for Catastro at all (green field)" |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|---------------------------|
| C7 `rural locate` | Pure wrapper rename of Consulta_DNPPP already in absorb manifest | — (absorbed) |
| C8 `tile <RC>` WMS multi-layer | Subsumed by C5 `report` bundle | C5 `report` |
| C9 `valuation` | Valor catastral fiscal data not in Webservices Libres | — (out of API) |
| C6 `parts <RC>` standalone | One-flag projection of DNPRC; better as `property show <RC> --parts` | absorbed #6 |
| C11 `municipality summary` | Duplicate of C14 `coverage` at finer grain | C9 survivor `coverage` |
