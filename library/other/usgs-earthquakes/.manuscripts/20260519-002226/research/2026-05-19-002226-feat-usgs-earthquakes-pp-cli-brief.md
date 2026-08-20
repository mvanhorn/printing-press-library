# USGS Earthquakes CLI Brief

## API Identity
- **Domain**: USGS Earthquake Hazards Program — `earthquake.usgs.gov`
- **Surface**: Two surfaces composed into one CLI
  - **FDSN Event Service** (`/fdsnws/event/1/`) — rich, parameterized event catalog
  - **GeoJSON summary feeds** (`/earthquakes/feed/v1.0/summary/{level}_{period}.geojson`) — 20 pre-built feeds updated every minute
- **Users**: Seismologists & researchers, journalists & news desks, emergency managers, citizen scientists, developers building quake-aware apps, outdoor planners checking pre-trip activity, educators
- **Data profile**: Real-time + historical seismic event catalog with rich per-event products (ShakeMap, PAGER, DYFI, focal mechanisms)

## Scope Decision
- **In**: FDSN Event service (search, count, get-event, catalogs, contributors, application metadata) + 20 GeoJSON summary feeds
- **Out**: Water, elevation, GNIS, volcano, ScienceBase, TNM Access — narrower scope = better product

## Reachability Risk
- **Low** — no auth, no documented per-IP rate limits, public endpoints
- **Hard limit**: FDSN `limit≤20000` per query — paginate with `offset` for larger result sets
- **USGS recommends** GeoJSON summary feeds over `fdsnws/event/query` for repeated polling — first-class in this CLI as a feed-snapshot fast path
- **Polite-client convention**: USGS does not formally require User-Agent contact info, but FDSN-compliant clients customarily include `User-Agent: usgs-earthquakes-pp-cli/<ver>`

## Top Workflows
1. **Live quake watch** — `usgs-earthquakes recent --min-magnitude 4.5 --within 24h` or `usgs-earthquakes recent --near "37.77,-122.42" --radius-km 500 --since 24h` → FDSN Event with circle filter, or summary-feed snapshot for global polling
2. **Get a specific event** — `usgs-earthquakes event us7000abcd` → returns the GeoJSON Feature with all products (ShakeMap, PAGER, DYFI, focal mechanisms)
3. **Historical search** — `usgs-earthquakes search --min-magnitude 6 --start 2020-01-01 --end 2024-12-31 --order-by magnitude` → bounded historical query
4. **Region monitoring** — `usgs-earthquakes search --bbox -125,32,-114,42 --since 7d` → California rectangular region for the past week
5. **Watch / stream** — `usgs-earthquakes watch --min-magnitude 5 --notify` → long-running poll of summary feed with deduplication against local store; pluggable notifier hook
6. **Counts** — `usgs-earthquakes count --min-magnitude 5 --since 30d` → FDSN `/count`, fast precheck before a full search
7. **Feed snapshot** — `usgs-earthquakes feed significant-week --json` → direct pull of a named summary feed without crafting query params

## Table Stakes
- Search by time / magnitude / region (rectangular bbox OR circle) — matches blake365/usgs-quakes-mcp, every FDSN client
- Get-event by ID — matches every existing tool
- 20 summary feeds reachable as named feeds (significant, M4.5, M2.5, M1.0, all × hour/day/week/month) — most tools fragment these into separate functions
- `--json`, `--csv`, `--select`, `--compact`, `--limit` for agent use
- `doctor` covering FDSN base URL + summary feed reachability + version
- `sync` populating local SQLite with rolling earthquake catalog for offline `sql`/`search`
- All event-product types reachable (`shakemap`, `losspager`, `dyfi`, `focal-mechanism`, `moment-tensor`)
- PAGER alert level filter, MMI / CDI filters, depth filters, review-status filter, catalog/contributor filters, event-type filter

## Data Layer (SQLite)
- **Primary entities**:
  - `earthquakes` — last 30 days M2.5+ + last 7 days all-magnitude (~30-80K events steady state), refreshed by `sync`
  - `event_products` — per-event product index (ShakeMap, PAGER, DYFI, etc.) stored as joined rows for fast "events near X with ShakeMap" queries
  - `catalogs`, `contributors`, `event_types` — static reference dictionaries (~30-80 rows each) from FDSN `/catalogs`, `/contributors`, `application.json`
- **Sync cursor**: `updatedafter` watermark per sync run — FDSN supports this natively, very cheap incremental sync
- **FTS5/search**: FTS5 on `place` (the human-readable description string like "10km SSW of San Francisco, CA"), plus indexes on `(time DESC, mag DESC)` and `(latitude, longitude)` for spatial queries

## Codebase Intelligence
- **Competing MCP**: `blake365/usgs-quakes-mcp` — only 2 tools (`find-earthquakes`, `find-earthquake-details`). Thin wrapper, no caching, no MMI/CDI/product filters exposed.
- **npm**: `usgs-earthquake-api` (doojin) — old NodeJS wrapper of FDSN Event; covers basic params, no novel features.
- **Python**: `obspy` — heavyweight seismology suite; reads FDSN events; overkill for non-researchers. `aio-geojson-usgs-earthquakes` — async GeoJSON-feed client (Home Assistant integration).
- **R**: `eq.uscensus` / various wrappers — academic, not user-friendly CLIs.
- **Auth**: none required.
- **Data model**: each event has a USGS event ID (`us7000abcd` shape), magnitude (with type — `mb`, `mw`, `ml`, etc.), origin time, depth, location (lat/lon), `place` description string, alert level (PAGER green/yellow/orange/red), `sig` (significance score 0-1000), `mmi` (Modified Mercalli max), `cdi` (Community DYFI), `tsunami` flag, `felt` count, and a `products` map with ShakeMap, PAGER, DYFI, etc.
- **Endpoint shape**: GeoJSON Feature/FeatureCollection. Each feature's `properties` is the event metadata; `geometry.coordinates` is `[lon, lat, depth_km]`.
- **Rate limiting**: no documented per-IP limits but USGS asks repeat consumers to use summary feeds.
- **Architecture**: FDSN Event is itself a federation pulling from ANSS contributors; the CLI exposes contributor + catalog filters so users can scope to e.g. the Northern California network.

## Source Strategy
- **Spec**: author internal YAML covering FDSN Event endpoints + the 20 summary feeds as named feed-paths
- **FDSN application.json** at `https://earthquake.usgs.gov/fdsnws/event/1/application.json` exposes enumerated valid values for `alertlevel`, `eventtype`, `catalog`, `contributor`, `producttype`, `orderby`, `reviewstatus`, `format` — use these for enum constraints in the spec
- **Auth**: `auth.type: none`

## Product Thesis
- **Slug**: `usgs-earthquakes`, binary `usgs-earthquakes-pp-cli`, display name `USGS Earthquakes`
- **One-liner**: "Every USGS earthquake feed and event query in one terminal — with offline SQLite cache, agent-native output, and a live watch mode."
- **Why it should exist**:
  - **The competing MCP exposes only 2 tools** out of FDSN's ~25-param surface. PAGER alert, MMI, CDI, product-type, review-status, catalog/contributor, depth, time-windows — all underexposed.
  - **No tool unifies the 20 summary feeds** with the FDSN Event API. Users pick one or the other; this CLI presents both as first-class.
  - **No offline SQLite caching** in any existing tool — every query is a live hit. With a 30-day rolling cache, `sql`/`search`/`near` work instantly without the network.
  - **No watch/stream mode** — current tools are request/response. A long-running watch with dedup-against-local-store is genuinely novel.
  - **Smart cross-surface routing** — `usgs-earthquakes feed significant-week` and `usgs-earthquakes search --min-magnitude 6.5 --since 7d` produce the same data via different paths; the CLI picks the cheaper one based on params.

## Build Priorities
1. **P0 (foundation)** — internal YAML spec; SQLite store for earthquakes + event_products + catalogs/contributors/event_types; sync command with `updatedafter` cursor; FTS5 search on `place`.
2. **P1 (absorb)** — every parameter FDSN exposes as a flag; get-event; count; catalogs list; contributors list; the 20 feeds as named feed-shortcuts; all output modes.
3. **P2 (transcend)** — `watch`, `near` (single command for "what's recent near this coord"), `region-history` (M+ counts per N years for a place), `swarm-detect` (clusters of events in time/space), `compare` (two regions or two periods side-by-side), `decode-id` (parse a USGS event ID into network/sequence), `migration-check` (deprecated catalog/contributor names).
