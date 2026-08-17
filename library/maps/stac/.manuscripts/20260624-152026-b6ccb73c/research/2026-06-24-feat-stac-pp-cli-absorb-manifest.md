# STAC CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List collections | pystac-client, rustac | `(generated endpoint) collections get` | Offline cache, --json/--select, table output |
| 2 | Get collection by id | pystac-client | `(generated endpoint) collections describe` | Extent/license/summaries rendered |
| 3 | Collection items (OGC Features) | stac-server | `(generated endpoint) collections items` | Paged, cached |
| 4 | Get item by id | go-stac-client | `(generated endpoint) collections item-get` | Asset table, --select |
| 5 | Item search GET | pystac-client `search` | `(generated endpoint) items get-search` | Typed flags |
| 6 | Item search POST (intersects/native) | pystac-client `--method POST` | `(generated endpoint) items post-search` | GeoJSON intersects |
| 7 | Rich item search (all filters) | pystac-client, eodag, sat-search | `stac-pp-cli workflow search` | bbox/datetime/intersects/collections/ids/limit/max-items + query-filter sugar + sortby + fields + auto-paginate + matched |
| 8 | Filter by query extension | sat-search `-q "eo:cloud_cover<10"`, pystac `--query` | `(behavior in stac-pp-cli workflow search)` `--max-cloud`/`--min-cloud`/`--query KEY OP VALUE` → `{"query":{...}}` | Compiles sugar to the extension Earth Search actually supports |
| 9 | sortby | pystac-client `--sortby` | `(behavior in stac-pp-cli workflow search)` `--sort field:asc` → object form | Emits the object form Earth Search requires (string form 400s) |
| 10 | fields include/exclude | pystac-client `--fields` | `(behavior in stac-pp-cli workflow search)` `--fields`/`--select` | Trims huge payloads for agents |
| 11 | Pagination (bounded) | pystac-client `--max-items` | `(behavior in stac-pp-cli workflow search)` auto-replay POST `next` cursor, `--max-items` cap | Safe bounded default (no runaway full-catalog walk) |
| 12 | Count-only / matched | pystac-client `--matched` | `(behavior in stac-pp-cli workflow search)` `--count` | numberMatched with limit=1 (cheap) |
| 13 | Conformance classes | pystac-client (none — gap) | `(generated endpoint) conformance` | Feature-detect support |
| 14 | Queryables per collection | stac-mcp `get_queryables` | `(generated endpoint) queryables` | Lists filterable fields (spec-enriched endpoint) |
| 15 | Aggregations | stac-mcp `get_aggregations` | `(generated endpoint) aggregate` | Real `/aggregate` endpoint (spec-enriched) |
| 16 | Provider switching | eodag `--provider` | `stac-pp-cli providers use/list/show` | Switch STAC endpoints; STAC_BASE_URL override |
| 17 | Local archive / sync | none (cache is the gap) | `stac-pp-cli sync`, `workflow archive`, `workflow status` | Offline SQLite, the killer differentiator |
| 18 | Local full-text search | none | `stac-pp-cli search` | FTS over synced item properties |
| 19 | Export / import | none | `stac-pp-cli export`/`import` | JSONL backup/migration |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Least-cloudy scene over AOI | `scenes best` | hand-code | query-filter + sortby asc with **client-side ranking fallback** when server sort is unsupported; returns single best scene + asset URLs | Use to pick the single clearest scene for an AOI+date range. Not for listing many scenes; use 'workflow search'. |
| 2 | Coverage / availability check | `coverage` | hand-code | matched-count (limit=1) + real `/aggregate` datetime_min/max/frequency over AOI+range | Use to answer "is there data here, and when". Backed by the aggregation endpoint. |
| 3 | Cloud-cover histogram | `clouds` | hand-code | real `/aggregate?aggregations=cloud_cover_frequency` over AOI+range | Use to see the cloud-cover distribution before committing to a download. |
| 4 | Scene timeline (change detection) | `timeline` | hand-code | one observation per date, deduped by sat:relative_orbit/grid tile, cloud per date — local join no single API call provides | Use to build a time series for change detection. Not a raw item list. |
| 5 | Temporal gaps | `gaps` | hand-code | enumerate scene dates over AOI+range, diff vs expected revisit — requires local time-windowed aggregation | Use to find missing acquisition windows over an AOI. |
| 6 | Asset / band URL resolution | `assets` | hand-code | provider-aware band-name mapping (red/nir/swir/visual), prefer COG over jp2, filter by band/role | Use to get download URLs for an item's bands. Provider-aware aliasing. |
| 7 | Compare collections | `compare` | hand-code | same AOI+range vs two collections: matched counts + cloud stats + resolution/revisit — local cross-source | Use to choose between e.g. Sentinel-2 vs Landsat for an AOI. |
| 8 | Watch new scenes | `watch` | hand-code | diff live search against local-store seen IDs since last run — requires the SQLite cache | Use to monitor an AOI for newly published scenes. Local state that compounds. |
| 9 | xarray stack snippet | `stack-snippet` | hand-code | compose matched item hrefs into a ready-to-paste stackstac/odc-stac Python snippet | Use to bridge CLI discovery into a Python analysis cube. |

Hand-code transcendence rows: **9 planned** (all hand-code; generator emits Priority 0/1).

## Stubs
None. All transcendence rows call real endpoints or the local store.
