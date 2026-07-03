# forMaps CLI — Absorb Manifest

## Source ecosystem (what exists today)

| Tool | Surface | Purpose |
|---|---|---|
| forMaps.it / STIMATRIX | Commercial SaaS | Cadastral search + property data reports (paid) |
| catastomappe.it | Commercial REST API | GPS↔cadastral with GeoJSON polygons (paid, per-call) |
| openapi.com (Italian Cadastral) | Commercial async REST | Bulk cadastral lookups (paid) |
| ondata/dati_catastali | DuckDB+Parquet recipes | Free attribute lookup via SQL-on-Parquet (Python/DuckDB required) |
| pigreco/workshop-estate-gis-2021 | QGIS workflow | WMS GetFeatureInfo for GPS→cadastral (manual, QGIS-bound) |
| enricofer/catasto | QGIS plugin | Interactive AdE WMS browser (QGIS-only) |
| Andrea Borruso / tantotanto | bash+curl pipeline | GPS→cadastral via WMS GetFeatureInfo HTML scrape |

Nobody ships a single self-contained binary that does both directions, works offline after sync, and emits agent-native JSON. That is our slot.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | GPS → cadastral (single point) | Borruso WMS-scrape | Direct call to AdE ajax JSON endpoint | One HTTP call, structured JSON, no HTML scrape |
| 2 | Cadastral → GPS (single parcel) | ondata DuckDB recipe | Pure-Go Parquet reader over HTTP range | No DuckDB / Python install; single static binary |
| 3 | Comune name → codice belfiore | catastomappe.it (paid) | Bundled ISTAT lookup table (~8000 comuni) | Free, embedded, offline-first |
| 4 | Codice belfiore → comune name | ondata index.parquet | Same bundled table | Works without network |
| 5 | Parcel polygon (GeoJSON) | catastomappe.it (paid) | AdE WFS GetFeature with tiny bbox + GML→GeoJSON parse | Free; renders directly in any map UI |
| 6 | Batch CSV reverse-geocode | Borruso bash pipeline | Native `--stdin` CSV/JSONL streaming with `--rate` throttle | Concurrency, progress, retry, typed errors |
| 7 | List all comuni in a province | None (forced manual lookup) | `comune list --province RM` | Quick discovery for new users |
| 8 | Local sync of all parcel centroids | ondata Parquet (manual download) | `sync` command with per-region ETag cache | Offline mode + automatic refresh detection |
| 9 | SQL over local parcel cache | None for cadastral | Inherited from PP framework (`sql` command, FTS5) | Power-user composability |
| 10 | Doctor (auth, reachability) | None | `doctor` checks AdE + ondata reachability + cache state | Confidence before relying on output |
| 11 | Agent context / MCP server | None | Inherited PP MCP server, every command `mcp:read-only` | Drop-in for Claude Desktop, Cursor, etc. |

## Transcendence (only possible with our approach)

Theme groups: **Local state that compounds**, **Field-work ergonomics**, **Agent-native composability**.

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 1 | Bulk reverse-geocode a route/trace | `formaps gps --stdin < route.csv --json` | Streaming pipeline with adaptive rate-limiter; commercial APIs charge per call and rate-limit harder | 9/10 |
| 2 | Find adjacent / neighbour parcels | `formaps neighbours --comune H501 --foglio 508 --particella B` | Requires the centroid (ours) + WFS bbox (theirs); compose locally. No commercial tool exposes this | 8/10 |
| 3 | Cadastral coverage report for a region | `formaps coverage --region Sicilia --json` | Counts parcels per comune from local Parquet, surfaces missing data (TAA) honestly | 7/10 |
| 4 | Diff a cadastral reference over time | `formaps drift --comune H501 --foglio 508 --particella B --since 2024-Q1` | Compares cached centroid against current AdE; detects parcel splits/renumbering. Nobody else stores history | 7/10 |
| 5 | "What's near this point" | `formaps around --lat 41.89 --lon 12.49 --radius 50m --json` | Calls AdE ajax once, then WFS bbox once, returns all parcels in radius with cadastral refs + polygons | 8/10 |
| 6 | Offline parcel search by partial code | `formaps search "H501 508"` | FTS5 over the local parcel cache; agents can resolve sloppy human input | 7/10 |
| 7 | Validate a cadastral reference syntax | `formaps validate --comune H501 --foglio 508 --particella B` | Parse-only; explains shape rules; useful for forms/imports without burning an API call | 6/10 |

All seven score ≥6/10. The novel-features subagent (Step 1.5c.5) would brainstorm more, but for a 2-endpoint surface the design space is bounded; over-inventing produces noise. I'm calling the seven above the working novel set.

## Stubs

None planned. Every row above is shipping scope.

## Known Gaps (will be disclosed in README)

- **Trentino-Alto-Adige**: AdE does not publish cadastral data for TAA (Trento and Bolzano provinces run autonomous cadastral systems). Lookups for TAA codici belfiore will return a typed `RegionalDataUnavailableError`.
- **Building footprints (`fabbricati`)**: out of scope for v1. The AdE ajax endpoint returns parcel-level info; building queries need WMS GetFeatureInfo on a different layer.
- **Historical data**: ondata Parquet is a snapshot. `drift` works only from the moment the user starts caching.
