# EIA CLI Absorb Manifest

**API:** EIA APIv2 | **Slug:** `eia` | **Binary:** `eia-pp-cli`

## Generated (typed endpoint mirrors from the spec)

These are produced from `eia-openapi.yaml` and become the canonical CRUD surface.

| Command | Endpoint | Notes |
|---|---|---|
| `discover list-routes` | GET /v2/ | Top-level route enumeration |
| `discover describe-route --route <p>` | GET /v2/{route}/ | Returns frequencies, facets, columns |
| `electricity get-retail-sales` | GET /v2/electricity/retail-sales/data/ | facets: stateid, sectorid |
| `electricity get-rto-fuel-type-data` | GET /v2/electricity/rto/fuel-type-data/data/ | facets: respondent, fueltype |
| `electricity get-rto-region-data` | GET /v2/electricity/rto/region-data/data/ | facets: respondent, type |
| `electricity get-electric-power-operational-data` | GET /v2/electricity/electric-power-operational-data/data/ | facets: location, sectorid, fueltypeid |
| `natural-gas get-price-summary` | GET /v2/natural-gas/pri/sum/data/ | facets: duoarea, product, process, series |
| `natural-gas get-futures-prices` | GET /v2/natural-gas/pri/fut/data/ | Henry Hub futures + spot |
| `petroleum get-spot-prices` | GET /v2/petroleum/pri/spt/data/ | WTI, Brent, RBOB, distillate |
| `steo get` | GET /v2/steo/data/ | Forecast series by seriesId |
| `seds get` | GET /v2/seds/data/ | Annual state energy + CO2 |

## Novel (hand-authored, pp:novel)

High-value commands that compose endpoints + the local store. Each must do real work
(client call OR store read) — no constants, no fake builders. Sourced directly from
the NOI: EIA is a market intelligence terminal, not a docs browser.

| Command | What it does | Wraps | pp:annotation |
|---|---|---|---|
| `electricity retail-price <state> [--sector] [--latest]` | Latest retail price for a state, optional sector pin | retail-sales | `pp:client-call` |
| `electricity rto <ba> --fuel-mix` | Current fuel mix snapshot for a BA | rto/fuel-type-data | `pp:client-call` |
| `electricity generation <state> [--fuel-type]` | Monthly net generation by state | electric-power-operational-data | `pp:client-call` |
| `natgas price --series henry-hub [--last 30d]` | Henry Hub price history | natural-gas/pri/fut | `pp:client-call` |
| `natgas price spot --state <st>` | State citygate / spot natural gas price | natural-gas/pri/sum | `pp:client-call` |
| `petroleum price crude wti [--frequency]` | WTI crude price series | petroleum/pri/spt | `pp:client-call` |
| `steo --series <s> [--months N]` | STEO forecast window for a series alias | steo | `pp:client-call` |
| `co2 <state> [--sector] [--annual]` | State CO2 emissions (routed through SEDS) | seds | `pp:client-call` |
| `sync` | Mirror priority series into local SQLite | multiple routes | `pp:client-call` |
| `search <query>` | FTS5 across synced series metadata | local store | (store read) |

## Local SQLite Store

Path: `~/.local/share/eia-pp-cli/store.db` (XDG)

Tables:
- `series_meta(id PK, route, name, description, frequency, units, default_freq, start_period, end_period, last_synced_at)`
- `series_data(series_id, period, value REAL, units, PRIMARY KEY(series_id, period))`
- `series_fts` — FTS5 virtual table over `(id, name, description, route)`

`sync` walks priority routes, calls each route's metadata endpoint, calls `/data/` for
the recent N points, and upserts into both tables + the FTS index.

## Kill Check

- No hardcoded series values, no canned market data, no fake fuel-mix builders.
- All novel commands call the real EIA client OR read from the local store.
- `search` reads `series_fts`; everything else hits the API or the cache populated by sync.

## Auth

- Query parameter `api_key`.
- Env var `PRINTING_PRESS_EIA_API_KEY` (generator default).
- Config file `~/.config/eia-pp-cli/config.yaml`.
- Get a key: https://www.eia.gov/opendata/register.php
- Quota: undocumented; the EIA site recommends client-side caching, which the local
  mirror provides.

## Out of Scope (v0.1)

- Bulk download endpoints (manifest.txt + ZIPs at https://api.eia.gov/bulk/).
- Excel Add-In specifics.
- AEO / IEO long-term forecast routes (steo covers the short term, which is what
  traders actually use).
- International route — large surface area, less common for U.S. desks.
