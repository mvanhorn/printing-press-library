# Global Fishing Watch (GFW) CLI Brief

## API Identity
- Domain: Ocean / vessel-activity intelligence. AIS-derived vessel identity, behavioral events (encounters, loitering, port visits, fishing), risk insights, and gridded fishing-effort data.
- API: v3 (CURRENT; v2 deprecated). Base `https://gateway.api.globalfishingwatch.org`, paths under `/v3/...`. Auth: Bearer JWT via `GFW_TOKEN` (free registration). Spec resolved from the official v3 Postman collection (213 requests → 15 clean DD endpoints).
- Users: maritime due-diligence analysts, NGOs, journalists, fisheries enforcement, sanctions/compliance researchers.
- Data profile: vessels (identity + history), events (typed behavioral records with positions/times/partners), insights (risk indicators), 4wings (fishing-effort grids).

## Reachability Risk
- None. Documented, stable, officially-maintained API with two official SDKs. Bearer auth; unauthenticated requests return 401 (expected). Token available + ambient in env.

## Top Workflows (DD chain)
1. **Identify** a vessel: search by name/MMSI/IMO/callsign → resolve to GFW vessel ID + identity history.
2. **Behavior**: pull that vessel's events — encounters (who it met at sea), loitering, port visits, fishing — with positions and counterpart vessels.
3. **Risk**: pull GFW Insights risk indicators (e.g. gaps in AIS, vessel-identity issues, MOU/IUU flags).
4. **Effort context**: 4wings fishing-effort by region/time to see where activity concentrated.
5. **Bulk**: large region/time pulls via bulk-reports (async create → query → download).

## Table Stakes (from competitors)
- Official **gfw-api-python-client** (Vessels, Events, Insights, 4Wings) and **gfwr** (R; vessels, events) — vessel search/get, events list/get/stats, insights, 4wings report.
- Community **samapriya/gfw** CLI — auth, data-list, file-list, download (Datasets + bulk download surface).
- We match all of it AND add: offline SQLite cache, `--json`/`--select`/`--compact`, typed exit codes, `--dry-run`, and DD-compound transcendence commands no SDK/CLI offers.

## Data Layer
- Primary entities: `vessel`, `event`, `insight` (per-vessel), plus cached `effort` summaries.
- Sync cursor: events by vessel + date range; vessels by query.
- FTS/search: vessel name/owner/flag; event type/counterpart.
- Why it compounds: caching vessels+events+insights locally enables cross-entity DD queries (encounter networks, risk rollups, watchlist drift) that require a local join the API can't do in one call.

## User Vision
Phase 1b of the Vessel-MCP project. `gfw-pp-cli` is the **behavior + risk** source; `gisis-pp-cli` (Phase 1a, shipped) is the **identity/registry** source. Phase 3 orchestrator will shell out to both. Prioritize the due-diligence surfaces: Vessels + Events + Insights (P1), 4Wings + Datasets data (P2), Bulk Download (P3). Map-tile/PNG rendering intentionally out of scope (headless DD tool).

## Product Thesis
- Name: `gfw-pp-cli` — "Every GFW vessel-behavior and risk surface on the command line, plus a local cache that turns one-shot lookups into a compounding due-diligence index."
- Why it should exist: the official SDKs are Python/R libraries (not CLIs); the one community CLI only downloads datasets. There is no agent-native, offline-caching, DD-oriented GFW CLI. It pairs with `gisis-pp-cli` to give the Vessel-MCP orchestrator both identity and behavior on day one.

## Build Priorities
1. **P0 data layer**: vessels/events/insights tables + sync + search + SQL.
2. **P1 absorbed**: vessel search/get/list, events list/get/stats, insights vessel, 4wings report/stats/last, datasets effort, bulk-reports create/get/query/download.
3. **P2 transcendence (DD-compound)**: `vessel dossier` (identity+events+insights merged), `vessel risk`, `encounters network`, `port-pattern`, watchlist (`pin`/`refresh`/`since`), AIS-gap/dark-activity flag.
