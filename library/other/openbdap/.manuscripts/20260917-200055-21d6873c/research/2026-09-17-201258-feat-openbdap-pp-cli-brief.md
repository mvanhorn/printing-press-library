# OpenBDAP CLI Brief

## API Identity
- Domain: catalogo Open Data della Banca Dati delle Amministrazioni Pubbliche (BDAP), Ragioneria Generale dello Stato (RGS, MEF). Host `bdap-opendata.rgs.mef.gov.it`.
- Users: giornalisti di dati, civic tech (monitoraggio PNRR/PNC, opere pubbliche), ricercatori di finanza pubblica, analisti di enti locali.
- Data profile: 3857 dataset, 13 gruppi tematici (bilancio dello Stato, bilanci enti PA, SIOPE/cassa, debito, pubblico impiego, sanità, opere pubbliche MOP, tesoreria, UE, rendiconto), 76 tag. Molte serie annuali/mensili ("2024 - …", "2025/08 - …") e regionali ("… - Sicilia"). Licenza CC-BY. Dataset grandi: "Progetti Opere Pubbliche MOP - Totale" ~560k righe, CSV 428 MB.
- Auth: nessuna.

## Surfaces (verified live 2026-09-17)
- CKAN-like facade (Java/Tomcat, not real CKAN) `/SpodCkanApi/api/3/action/`:
  - `package_list` → 3857 UUID (0.4 s).
  - `package_show?id=<uuid>` → ~0.14 s. **Only by UUID; by name returns non-JSON help/garbage.**
  - `package_search?q=&rows=&start=` → q works, start/rows paging works, **`count` is wrong (= rows returned), `fq` ignored** (groups filter returns unrelated). Slow: rows=50 ~6.6 s.
  - `group_list` (13 names), `group_show?id=` (includes `packages` UUID list, 128 for opere pubbliche), `tag_list` (76), `organization_list` (3), `organization_show`.
  - 404: `status_show`, `resource_show`, `current_package_list_with_resources`, `recently_changed_packages_activity`.
  - Resources per dataset: CSV dump `https://…/SpodCkanApi/api/3/datastore/dump/<pkg-uuid>.csv` (`;`-separated, works over https only; http fails), PDF metadata doc, OData resource (`resource_type: "OData"`) whose **resource id differs from the package id** and is the OData key.
  - Extras: issued, modified, frequency, theme, search/view/download_count.
- OData v2 proxy `/ODataProxy/MdData('<odata-id>@rgs')`, `Accept: application/json` (`d` envelope):
  - entity: lastUpdate ("04/09/2026 11:06:24"), isReady, links.
  - `/DataColumns`: physicalName (`ccodice_cup`), logicalName ("Codice CUP"), colUniqueId (`Cccodice_cup_1267962549`, deterministic: same physicalName → same id across datasets), dbType STRING/NUMERIC, cardinality, maxLength, `values` (JSON-string of distinct values).
  - `/DataMeasures` (alias, decimalPlaces), `/DataRows` supports `$top $skip $filter (eq, and, substringof) $select $orderby`. `$inlinecount`/`__count` always "0"; `$metadata` empty; no MdData listing. `DistinctCountRows`, `DataDimensions` empty; `RawDataRows` empty.
  - Performance: filter by CUP on 560k-row Totale 1.7–2 s; `$top=5000&$skip=100000` = 64 s / 12 MB; `$top=50000` times out >120 s. Unknown id → HTTP 500.

## Reachability Risk
- Low. No auth, no WAF observed, no 403. Risks: slow large pages (timeouts), 500 on bad id, CKAN facade quirks (count/fq broken, show-by-name broken).

## Top Workflows
1. Sfogliare/cercare il catalogo intero offline: per testo, gruppo, tag, anno, regione, formato; capire cosa c'è e dove.
2. Capire un dataset prima di scaricarlo: colonne con nomi leggibili, tipo, cardinalità, valori distinti.
3. Estrarre righe filtrate via OData usando nomi di colonna leggibili (non `Cccodice_cup_1267962549`), in JSON/CSV, con paginazione automatica.
4. Ricerca CUP machine-to-machine (caso d'uso pubblicato su pnrr.datibenecomune.it): progetto dal Totale + pagamenti, gare, partecipanti, piano dei costi, soggetti titolari dai dataset MOP regionali. Anche per CIG e per codice fiscale del titolare (opere di un ente).
5. Serie storiche: trovare tutte le annualità/mensilità di una stessa serie e le novità dopo l'ultimo aggiornamento.

## Table Stakes
- package list/show/search, group list/show, tag list (CKAN parity, as in generic CKAN tools e.g. ckan-mcp).
- OData rows with $filter/$select/$top/$skip; JSON output.
- CSV full download.

## Data Layer
- Primary entities: dataset (uuid, name, title, notes, tags, groups, modified, issued, frequency, odata_id, csv_url, pdf_url, counts), resource, group, tag. Optional cache of DataColumns per odata_id.
- Sync cursor: package_list diff + metadata_modified; full refresh ~3857 package_show at bounded concurrency.
- FTS/search: FTS5 on title, notes, tags, groups. Derived fields: anno/periodo parsed from title prefix, regione parsed from title suffix, serie = title without year/region.

## Codebase Intelligence
- Italian-Builders-Org/DoveVannoINostriSoldi (318★, web app) uses MdData metadata + DataColumns + DataRows on MOP Totale for CUP lookup and Legge di Bilancio snapshots; confirms schema contract approach (resolve column by physicalName). No CLI exists for OpenBDAP; generic CKAN tools break on this facade (count/fq/show-by-name).

## User Vision
- Sfogliare tutto il catalogo. La ricerca per CUP è solo un esempio: valutare cosa è utile esporre. Comandi e output in italiano (stile ars-sicilia). Slug openbdap, repo aborruso/openbdap-pp-cli, poi publish su printing-press-library.

## Product Thesis
- Name: openbdap-pp-cli
- Why it should exist: il catalogo RGS ha un'API CKAN rotta (count, fq, show per nome) e un OData con nomi di colonna offuscati e senza listing. Una CLI con store locale del catalogo, ricerca offline, traduzione automatica dei nomi colonna e una ricerca CUP/CIG/CF trasversale ai dataset MOP rende usabile machine-to-machine ciò che oggi richiede di aprire il sito, cliccare "ottieni URL OData" e indovinare i campi.

## Build Priorities
1. `sync` catalogo → SQLite + FTS; `cerca`, `dataset`, `gruppi`, `tag`, `serie`, `novita` offline.
2. `colonne`, `valori`, `righe` (filtri per nome leggibile, paginazione, CSV/JSON), `scarica` CSV.
3. `cup`, `cig`, `opere --cf` trasversali MOP.
