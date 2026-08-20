# PVGIS CLI Brief

## API Identity
- **Domain:** EU Joint Research Centre solar resource and PV-performance data service.
- **Users:** PV designers sizing residential/commercial systems, energy auditors, climatologists, agentic workflows that need ground-truth solar yield without subscribing to commercial APIs (e.g., Solargis, Meteonorm).
- **Data profile:** Hourly and monthly solar radiation, hourly PV production simulations, 8760-hour TMY weather, DEM-derived horizon profiles. Coverage: ~60S to 65N globally, multi-year (typically 2005-2023). No auth, no rate limit published but courtesy throttling expected.

## Reachability Risk
**None.** Live probes against `https://re.jrc.ec.europa.eu/api/v5_3/` returned 200 with the documented shapes for all five endpoints (MRcalc, PVcalc, seriescalc, tmy, printhorizon). Public API, no auth required, no Cloudflare/WAF observed.

## Top Workflows
1. **Site assessment** — given (lat, lon), estimate annual yield for a 1-kWp reference system and the optimal tilt. Decision: viable / not viable, what tilt to recommend.
2. **Hourly production simulation** — 8760-hour P timeseries for sizing batteries, inverters, self-consumption modeling.
3. **TMY climate download** — get the canonical typical-year weather to feed external tools (PVSyst, SAM).
4. **Horizon / shading check** — pull the SRTM-derived horizon profile to estimate terrain shading losses.
5. **Optimal tilt search** — sweep tilt angles to find the production peak for the target azimuth.

## Table Stakes (the existing pvgis-mcp ground truth)
The reference Python MCP at `/Volumes/Studio Storage/.../pvgis-mcp/server.py` exposes 7 tools. The CLI must match each:
- `get_irradiance` → MRcalc with horizontal+tilted+optimal+temperature in one call.
- `get_production_timeseries` → PVcalc (monthly) and seriescalc (hourly) with year range.
- `get_solar_factsheet` → three MRcalc requests merged (horizontal, 35° south, optimal).
- `get_weather_data` → tmy with monthly-aggregated T2m/WS10m/G(h) summary.
- `calculate_shading` → printhorizon + a local heuristic for obstacle-based shading.
- `get_pvgis_coverage_area` → static reference info on which database covers which region.
- `pv_module_optimal_tilt` → PVcalc sweep 0-60° step 5°, max E_y.

The Python server adds value beyond raw HTTP: file-cache with content-addressed keys, robust field-alias handling (H(h)_m vs H(h) vs H_h_m), monthly aggregation of hourly TMY data. The Go CLI must inherit these.

## Data Layer
- **Primary entities:** Sites (lat, lon, elevation, db), MonthlyRadiation, MonthlyProduction, HourlySeries, TMYHour, HorizonPoint.
- **Sync cursor:** None — every response is parameterized by (lat, lon, system params); cache keyed on the canonical parameter tuple.
- **FTS/search:** Site notes/labels are user-supplied; entity-FTS on site labels and on the radiation-database choice.

## Codebase Intelligence
- **Source 1 (ground truth):** Local `/Volumes/Studio Storage/.../pvgis-mcp/server.py` — 770 lines, single file, FastMCP + httpx. Field-alias logic is in `_first_number` / `_month_values`; cache in `_cache_path` / `_cached_get`. The CLI port should preserve this tolerance because PVGIS field names shift between v5_2 and v5_3.
- **Source 2:** Live probes show v5_3 uses `outputs.monthly.fixed[]` (not bare `outputs.monthly[]`) for MRcalc with `selectrad=1`. The CLI parser must handle both shapes.
- **Auth pattern:** None. The Python server supports an optional `apikey` query param via `PVGIS_API_KEY` env var, but PVGIS public endpoints don't actually require it. The CLI offers `--api-key` as a future-proofing courtesy flag.
- **Rate limiting:** Not documented. Conservative 1 req/sec courtesy limiter in `cliutil.AdaptiveLimiter`.

## User Vision
Ground-truth reference implementation is the existing Python pvgis-mcp server. The CLI must match its 7 tools at minimum, then transcend with offline cache, multi-site batch, and local interpolation that don't require repeat API hits.

## Product Thesis
- **Name:** `pvgis-pp-cli`
- **Why it should exist:** Every PVGIS tool in the wild is either (a) a website behind a JS-heavy UI that an agent can't drive cleanly, (b) a Python notebook the user has to set up, or (c) a paid wrapper. There is no single-binary, agent-native CLI for the JRC API. With the offline SQLite cache plus batch and sweep features, the CLI becomes the fastest way to ask "how much yield will I get at this site?" — including from an LLM agent.

## Build Priorities
1. **Match server.py's 7 tools 1:1**, with the same field-alias robustness.
2. **Local SQLite cache** keyed on (endpoint, canonical-param-tuple, db_choice) so repeated calls for the same site are instant and the user can `pvgis sql` ad-hoc.
3. **Transcendence: multi-site rank** — feed a CSV of sites and rank by yield/kWp.
4. **Transcendence: tilt/azimuth heatmap** — compute the 2D grid once, cache, interpolate locally.
5. **Transcendence: db comparator** — same site under SARAH3, NSRDB, ERA5; surface which database thinks what.
