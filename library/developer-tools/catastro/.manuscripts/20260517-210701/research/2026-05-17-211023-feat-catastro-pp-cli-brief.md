# Catastro (OVC) CLI Brief

## API Identity

- **Domain:** Spanish national cadastral registry. Property/parcel data for ~80 million parcels across the 47 mainland provinces, Balearics, Ceuta, Melilla (Basque provinces – Álava, Bizkaia, Gipuzkoa – and Navarra are managed separately).
- **Operator:** Dirección General del Catastro, Ministerio de Hacienda (`catastro.hacienda.gob.es`). Services hosted at `ovc.catastro.meh.es` (Oficina Virtual del Catastro).
- **Auth:** None for public consultation. All "Webservices Libres" are open with simple GET/SOAP. Recommended rate ~5 req/s per IP; bulk scraping is discouraged.
- **Surface families:**
  1. **OVC Callejero** (`/OVCServWeb/OVCWcfCallejero/COVCCallejero.svc/json/*`): provincia → municipio → vía → numero hierarchy + RC lookups. REST JSON works.
  2. **OVC Coordenadas** (`/ovcservweb/OVCSWLocalizacionRC/OVCCoordenadas.asmx/*`): RC ↔ X,Y in any SRS. Returns XML (REST JSON path returns 404; ASMX GET returns text/xml that parses cleanly).
  3. **INSPIRE WMS** (`/cartografia/INSPIRE/spadgcwms.aspx`): parcels, addresses, buildings, building-parts, zoning. Daily updated.
  4. **INSPIRE WFS** (`/INSPIRE/wfsCP.aspx`, `/INSPIRE/wfsAD.aspx`, `/INSPIRE/wfsBU.aspx`): parcels, addresses, buildings. Bbox / RC / postal-code filtering.
  5. **ATOM downloads** (`catastro.hacienda.gob.es/INSPIRE/{buildings,Addresses,parcels}/ES.SDGC.*.atom.xml`): bulk per-municipality GML, updated 2x/year.

## Reachability Risk

- **Low.** Live probes during research:
  - SOAP WSDL `OVCCoordenadas.asmx?wsdl` → HTTP 200, 12KB XML.
  - REST `ObtenerProvincias` → HTTP 200, JSON with all 48 provinces.
  - REST `Consulta_DNPLOC` for Calle Alcalá 1 Madrid → HTTP 200, 801 B JSON with full RC + property data.
  - WMS `GetCapabilities` → HTTP 200, 6 main layers (CP.CadastralParcel, AD.Address, BU.Building, etc.).
- Service is stable government infrastructure (operated since 2008). No 403/429 issues reported in community repos.
- The only nuance: Coordenadas service responds with XML on the ASMX GET endpoint (not JSON); we handle that in the client.

## Users (concrete personas drawn from real workflows)

1. **Property manager of a residential urbanization** (the user of this repo, Río Cedena Gestión, manages 602 parcels). Keeps a local DB of parcels with potentially-stale Catastro data (street mappings flagged unreliable in CLAUDE.md). Periodically needs to verify each parcel's official surface, use, year, and address against the Catastro source of truth; today they check parcels one-by-one in the web viewer.
2. **GIS analyst / consultant** (geomática firm, e.g. geomatico.es). Builds maps for ayuntamientos, real-estate firms, civil-engineering studies. Today uses QGIS plugin `Spanish_Inspire_Catastral_Downloader` or `cidownloader` Python script to download parcel GeoPackages by municipality. Pain: per-municipality only, no easy bbox or polygon download; manual workflow.
3. **Real-estate scout / property researcher** (housing market analysts, due-diligence consultants). Wants to look up a property's official record (built year, use, surface, building parts) from an address or coordinates. Today: opens `sedecatastro.gob.es`, types address into a slow ASP.NET form, clicks through 4 pages. Pain: no bulk lookups; no easy diff against prior month.
4. **Civil-engineering / cadastral expediente preparer** (technical bureaus that prepare expedientes for licencias, segregaciones, agrupaciones). Needs: RC ↔ coordinates conversion, building-part details (uses, surfaces by floor), polygon export to DXF/GeoJSON for CAD work. Today: SOAP calls one at a time, or pycatastro scripts. Pain: cross-RC joins are impossible (each call returns one parcel); no offline cache.

## Top Workflows

1. **Address → property card.** "Calle Alcalá 1, Madrid" → RC + use + surface + year + building-parts. (`Consulta_DNPLOC`)
2. **RC → property card.** "0545206VK4704F0001RE" → full DNPRC response with all the property's sub-parts. (`Consulta_DNPRC`)
3. **Coordinates → nearby RCs.** Reverse-geocode a lat/lon to all parcels within 50 m. (`Consulta_RCCOOR_Distancia`)
4. **RC → coordinates.** Map an RC to its X,Y centroid in ETRS89 / WGS84 / EPSG:25830. (`Consulta_CPMRC`)
5. **Bulk municipal download.** Fetch every parcel polygon (or building, or address) for a municipality as GeoJSON/GeoPackage. (ATOM + WFS)
6. **Polygon export.** "Give me every parcel inside this bbox/polygon" for QGIS or web map. (WFS bbox query)
7. **Map render.** Render parcels for a given bbox at zoom Z as PNG, with a parcel border or label overlay. (WMS)
8. **Verify-and-diff.** Compare a local parcel table against fresh Catastro and report changed surface/use/year/address. (No incumbent does this.)
9. **Province / municipality / vía / number hierarchy.** Browse the official cadastral street directory. (`ObtenerProvincias`/`ObtenerMunicipios`/`ObtenerCallejero`/`ObtenerNumerero`)
10. **Polígono+Parcela rural lookup.** Rural land: lookup by polygon + parcela code (no street address). (`Consulta_DNPPP`)

## Table Stakes (must match what exists)

| Tool | Coverage | Gaps |
|---|---|---|
| **gisce/pycatastro** (Python, SOAP) | All 16 Callejero + Coordenadas SOAP operations | No store, no bulk, no INSPIRE, returns dicts only |
| **rOpenSpain/CatastRo** (R) | OVC + WFS bbox/RC/postal + WMS layer rendering + ATOM bulk municipal | R-only, returns sf/tibble; no offline join; no diff |
| **MrCabss69/Python-Catastro** | Thin pycatastro wrapper | Same gaps as pycatastro |
| **sperea/catastro-lib-python** | Returns JSON, smaller subset | Same gaps |
| **geomatico/cidownloader** (Python CLI) | INSPIRE ATOM downloads → GeoPackage | Per-municipality only, no bbox |
| **sigdeletras/Spanish_Inspire_Catastral_Downloader** (QGIS plugin) | INSPIRE downloads inside QGIS | UI-only, not scriptable |
| **ibonkonesa/api-catastro** (Node.js) | Basic SOAP wrap | Very small surface |

**No existing CLI is "GOAT"**:
- No tool offers an offline SQLite store with FTS over parcels.
- No tool supports bulk-RC enrichment piped through stdin.
- No tool detects changes between syncs.
- No tool reconciles an external table against Catastro.
- No tool combines OVC consult + INSPIRE bulk in one command surface.
- No MCP server exists for Catastro at all (green field).

## Data Layer (what to persist locally)

Primary entities (synced from REST/WFS/ATOM):

- `provinces` (cpine, name) — 48 rows, ~stable.
- `municipalities` (codmunicipio, cpine, name) — ~8,100 rows, ~stable.
- `streets` (codigovia, sigla, nombre, codmunicipio) — populated on demand.
- `parcels` (rc, cpine, cm, cpos, np, nm, locint, surface, use, built_year, geom_wkt) — main entity.
- `parcel_parts` (rc, subparcela, use, surface, floor, coef_propiedad) — DNPRC sub-parts.
- `parcel_history` (rc, sync_at, surface, use, built_year, raw_json) — change tracking.

Sync cursor: per-municipality timestamp (ATOM is updated twice/year; daily WMS refresh).
FTS: parcels (rc, address, use, municipality) + streets (nombre, municipality) for free-text lookups.

## Codebase Intelligence

(No DeepWiki query — Catastro has no canonical SDK repo with a wiki; the canonical implementations are `gisce/pycatastro` and `rOpenSpain/CatastRo` which are small modules without internal abstractions worth documenting separately. Service shape is fully understood from WSDL + REST probes + wrapper analysis above.)

## Source Priority

Single source: OVC oficial (Callejero + Coordenadas + INSPIRE WMS/WFS/ATOM). The cartography viewer URL (`mapa.aspx`) is the user's entry point but the CLI builds on the oficial API surface. No multi-source priority gate.

## Product Thesis

- **Name (slug):** `catastro` — published binary `catastro-pp-cli`; canonical prose: "Catastro" (Sede Electrónica del Catastro español).
- **Headline:** Every Catastro feature, plus offline SQLite, bulk RC enrichment, polygon export, and change detection no other Catastro tool offers.
- **Why it should exist:**
  1. Spanish-government cadastral data is huge (~80M parcels) but the official UI is a slow ASP.NET form; no first-class CLI exists; the few wrappers (Python R) are library-only with no store.
  2. Power users (property managers like the urbanización this repo serves, GIS analysts, due-diligence firms) need batch workflows that the web viewer cannot deliver.
  3. AI agents need an MCP surface for cadastral lookups; none exists today.
- **Differentiation:**
  - Unified OVC + INSPIRE surface in one binary.
  - Offline FTS5 SQLite store synced from ATOM + WFS.
  - Reconciliation against external parcel tables (the killer feature for this repo).
  - Pipe-friendly bulk RC/address enrichment (`cat refs.txt | catastro enrich --json`).
  - WMS render to PNG and WFS export to GeoJSON without QGIS.

## Build Priorities

1. **Foundation:** internal YAML spec covering REST Callejero + ASMX Coordenadas; D1 store schema for provinces, municipalities, streets, parcels, parcel_parts, parcel_history; sync command.
2. **Absorbed (table-stakes):** all OVC operations from the WSDLs (16 total), WFS bbox/RC queries, WMS layer render, ATOM bulk municipal download.
3. **Transcendence (decided by the novel-features subagent in Step 1.5c.5).** Likely candidates: `reconcile`, `enrich --stdin`, `watch`/`diff`, `polygon-export`, `locate`, `analyze-area`.
4. **Polish:** README cookbook with realistic Madrid + Móstoles examples, agent-friendly `--select`, typed exit codes for "RC not found" vs "rate-limited".
