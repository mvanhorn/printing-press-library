# EIA CLI Research Brief

**Run:** 20260520-221754
**API:** U.S. Energy Information Administration (EIA) APIv2
**Base URL:** https://api.eia.gov/v2/
**Docs:** https://www.eia.gov/opendata/
**Spec source:** docs-derived (no official OpenAPI; route discovery via GET /v2/{route})

## NOI (Non-Obvious Insight)

EIA isn't just a government data portal. It's a real-time energy market intelligence
terminal. Every price series and generation mix update is a signal about where the grid
is stressed and where margins are moving. The CLI should treat EIA like Bloomberg for
energy: latest tick, time series, ranked filters, offline cache.

## Source Priority

Single source: EIA APIv2 (apiVersion 2.1.12 at run time). No combo gate.

## What's Already Out There

- **`eia-python`** (PyPI) — thin wrapper, v1-era, no v2 facet support, abandoned 2022.
- **`pyEIA`** — community wrapper, generic series fetch, no novel commands.
- **R packages (`EIAdata`)** — academic/research focus, no offline cache.
- **No MCP server, no CLI** for EIAv2 that I can find. Greenfield.

Gap: no tool offers (a) discoverable routes from the shell, (b) BA fuel-mix snapshot,
(c) Henry Hub spot vs futures comparison, (d) STEO forecast vs latest realized, or
(e) an FTS-indexed local mirror of series metadata.

## API Shape

- **Route discovery:** `GET /v2/{path}/` returns `routes[]`, `frequency[]`, `facets[]`,
  `data` columns (alias + units), `startPeriod`, `endPeriod`, `defaultFrequency`.
- **Data fetch:** `GET /v2/{path}/data/?frequency=...&data[]=value&facets[fueltype][]=NG`
  with `start`, `end`, `sort[0][column]`, `sort[0][direction]`, `offset`, `length`.
- **All values returned as strings** since v2.1.6 — must parse client-side.
- **Pagination:** offset + length, max 5000 rows per JSON page.
- **Auth:** `api_key` query parameter on every request.
- **Rate limits:** undocumented but generous; site recommends caching.

## Priority Routes

| Route | Frequency | Facets | Key data columns |
|---|---|---|---|
| electricity/retail-sales | M/Q/A | stateid, sectorid | price, sales, revenue, customers |
| electricity/rto/fuel-type-data | H/LH | respondent (BA), fueltype | value (MWh) |
| electricity/rto/region-data | H/LH | respondent, type (D,DF,NG,TI) | value (MWh) |
| electricity/electric-power-operational-data | M/Q/A | location, sectorid, fueltypeid | generation, consumption, stocks, receipts |
| natural-gas/pri/sum | M/A | duoarea, product, process, series | value (USD/MMBtu) |
| natural-gas/pri/fut | D/W/M/A | duoarea, product, process, series | value (futures price) |
| petroleum/pri/spt | D/W/M/A | duoarea, product, process, series | value (USD/bbl) |
| steo | M/Q/A | seriesId | value (forecast) |
| seds | A | seriesId, stateId | value (CO2 mt, energy consumption, etc.) |

## Auth & Config

- Env var: `PRINTING_PRESS_EIA_API_KEY` (matches generator default for `<API>_API_KEY`).
- Config file: `~/.config/eia-pp-cli/config.yaml`.
- Key value provided for live smoke (held in shell env, not committed to any artifact).

## Novel Features (Phase 3)

These are not endpoint mirrors; they orchestrate multiple endpoints + the local store.

1. **`electricity retail-price <state> [--sector] [--latest]`** — latest retail price for
   a state, optionally pinned to a sector (residential/commercial/industrial).
2. **`electricity rto <ba> --fuel-mix`** — current fuel mix snapshot for a BA (ERCO, PJM,
   MISO, CISO, NYIS, ISNE, etc.).
3. **`electricity generation <state> [--fuel-type]`** — monthly net generation by state,
   optionally filtered to one fuel type.
4. **`natgas price --series henry-hub [--last 30d]`** — Henry Hub spot history.
5. **`natgas price spot [--state tx]`** — citygate / state-level natural gas spot.
6. **`petroleum price crude wti [--frequency]`** — WTI crude oil price series.
7. **`steo --series natgas|oil|electricity [--months 6]`** — STEO forecast window.
8. **`co2 <state> [--sector electric-power] [--annual]`** — state CO2 emissions (now
   routed through SEDS since the dedicated co2-emissions route is deprecated).
9. **`sync`** — mirror key series into local SQLite.
10. **`search <query>`** — FTS5 across synced series IDs, names, and descriptions.

## Local SQLite Mirror Strategy

- DB at `~/.local/share/eia-pp-cli/store.db` (XDG).
- Tables: `series_meta` (id, name, route, frequency, units, default_freq, start_period,
  end_period, last_synced_at), `series_data` (series_id, period, value, units), and
  `series_fts` (FTS5 virtual table on id, name, description).
- `sync` walks the priority routes, populates metadata + the most recent N points for
  each.

## Risks / Edge Cases

- Values are JSON strings, not numbers — convert in the client.
- Facet names are inconsistent across routes (`stateid` vs `location` vs `stateId`).
- `co2-emissions` route is deprecated; SEDS is the replacement.
- BA respondent codes use FERC codes (ERCO, MISO, CISO) — document the common ones.
