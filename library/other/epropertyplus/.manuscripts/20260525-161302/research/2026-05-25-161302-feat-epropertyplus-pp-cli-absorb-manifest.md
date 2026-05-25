# ePropertyPlus CLI — Absorb Manifest

No competing CLI/library/scraper exists for ePropertyPlus (confirmed via search). "Absorbed"
features therefore come from the API's own surface + property-CLI table stakes; transcendence
features are what a multi-tenant, locally-backed CLI unlocks that the per-instance web UI cannot.

## Absorbed (API surface + table stakes — match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List published properties | API `searchSummaryPublicMapQuery` | `list` (id+lat/lng index) | `--json`/`--csv`, `--limit`, offline from store |
| 2 | Property detail | API `getPublishedProperty?propertyId=` | `get <id>` (full returnVal) | field `--select`, JSON/CSV, cached in store |
| 3 | Custom field config | API `getCustomFieldConfigs` | `custom-fields` (decodes s_custom_*/n_custom_*) | unwraps the JS-wrapped JSON; maps cryptic keys to names |
| 4 | Property image | API `viewImage/{id}/{file}` | `image <id>` (resolve + `--download`) | print URL by default, download on opt-in (verify-safe) |
| 5 | Search inventory | (none) | `search <term>` FTS over address/neighborhood/class/use | offline, regex, SQL-composable |
| 6 | Sync to local store | (none) | `sync` enumerate→hydrate→SQLite | offline analysis, repeatable, keyed by (instance,id) |
| 7 | Health check | (none) | `doctor` (instance reachable, endpoints OK) | agent-native preflight |
| 8 | Tabular/GIS output | (none) | `--json`, `--csv`, GeoJSON | pipes to jq / GIS / pandas |

## Transcendence (only possible with our multi-tenant + local-store approach)
| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|-------------------------|
| 1 | Multi-instance registry | `instances` (list/add known land-bank slugs; `--instance <slug>` on any cmd) | The web UI is one land bank at a time; we make every ePropertyPlus land bank one uniform surface |
| 2 | Structure vs lot classifier | `list --kind structure|lot` | Requires hydrating detail + applying the propertyClass/structureType rule the UI never exposes as a filter |
| 3 | Bulk hydrate-all | `sync` / `hydrate-all` | Index→detail fan-out across the whole instance into one local dataset; the UI paginates one property at a time |
| 4 | GeoJSON land export | `export --format geojson` | Joins lat/lng + parcel geometry into GIS-ready features for the Land Designer; no UI export of this shape |
| 5 | Parcel/condition join hook | `export --select parcelNumber,...` + `enrich --parcels` | parcelNumber is the join key to county/Socrata condition + market data; only a local dataset can be joined |
| 6 | Land Designer feed | `land-export` (lots + geometry + zoning + potentialUse) | Filters to vacant-land parcels with the exact fields the Land Designer needs; UI has no such projection |
| 7 | Cross-instance compare | `compare --instances kclb,<slug>` | Requires two synced local datasets side by side; impossible in any single-tenant UI |

## research.json novelty
- novel_features scored: registry(8), structure/lot filter(8), GeoJSON export(7), bulk hydrate(7), parcel-join/enrich(6), land-export(7), cross-instance compare(6). All >=5 → ship.

## Stubs
- `enrich --parcels` ships as a **documented hook** (emits parcelNumber + join instructions) rather than a built-in county/Socrata fetch — the join target is external and instance-specific. Marked `(stub — external join, hook only)`.
