# Open-Meteo CLI — Absorb Manifest

## Source tools surveyed

| Tool | URL | Type | Tools/commands |
|---|---|---|---|
| cmer81/open-meteo-mcp | github.com/cmer81/open-meteo-mcp | TS MCP | 17 (most comprehensive) |
| isdaniel/mcp_weather_server | github.com/isdaniel/mcp_weather_server | Python MCP | 3 |
| jeremymorgan/weather-mcp-server | github.com/jeremymorgan/weather-mcp-server | Python MCP | 3 (with WMO-code humanization) |
| nprousalidis/Open-Meteo-MCP-Server | github.com/nprousalidis/Open-Meteo-MCP-Server | TS MCP | ~5 |
| gbrigandi/mcp-server-openmeteo | LobeHub | MCP | ~6 |
| chuk-mcp-open-meteo | pypi.org/project/chuk-mcp-open-meteo | Python MCP | ~7 |
| @openmeteo/sdk | npmjs.com/package/@openmeteo/sdk | Official npm SDK | per-endpoint |
| openmeteo (npm) | npmjs.com/package/openmeteo | TS client | per-endpoint |
| @atombrenner/openmeteo | npmjs.com/package/@atombrenner/openmeteo | typesafe TS | per-endpoint |
| openmeteo-requests | pypi.org/project/openmeteo-requests | Official Python lib | per-endpoint |
| openmeteopy | github.com/m0rp43us/openmeteopy | Python wrapper | per-endpoint |
| frenck/python-open-meteo | github.com/frenck/python-open-meteo | async Python | per-endpoint |
| johnallen3d/conditions | github.com/johnallen3d/conditions | Rust CLI | current-conditions only, IP-geo |
| R366Y/weather_cli | github.com/R366Y/weather_cli | Python CLI | forecast only |
| Hitori-Laura/weather-fetch | github.com/Hitori-Laura/weather-fetch | Python CLI | forecast + IP-geo |

## Absorbed — match or beat everything that exists

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | 7-day weather forecast | cmer81 weather_forecast | `forecast` command, hourly+daily+current selection | All variables, units, model selection, --json/--select/--csv |
| 2 | Current conditions | jeremymorgan get_current_weather | `current` command | WMO code humanized, fast, agent-native |
| 3 | Weather summary (current + daily min/max) | jeremymorgan get_weather_summary | `summary` command | Compact mode, --select fields |
| 4 | Forecast for arbitrary days | cmer81 weather_forecast | `forecast --forecast-days N` | Up to 16 days |
| 5 | Past-days context for forecast | cmer81 (via past_days param) | `forecast --past-days N` | Includes prior-N-days hourly data |
| 6 | Variable selection (hourly) | all libs | `--hourly` flag, comma-separated list | All ~50 variables documented |
| 7 | Variable selection (daily) | all libs | `--daily` flag | All ~30 daily aggregates |
| 8 | Variable selection (current) | cmer81, openmeteo-requests | `--current` flag | All current vars |
| 9 | Unit selection (temp) | all libs | `--temperature-unit C/F` | Default C, F via flag |
| 10 | Unit selection (wind) | all libs | `--wind-speed-unit km/h, mph, m/s, kn` | All 4 |
| 11 | Unit selection (precip) | all libs | `--precipitation-unit mm/inch` | Both |
| 12 | Timezone handling | all libs | `--timezone auto, UTC, named` | Auto-resolve via geocoding when --place used |
| 13 | Time format selection | openmeteo-py | `--timeformat iso8601/unixtime` | Both |
| 14 | Historical ERA5 archive | cmer81 weather_archive | `archive` command | 1940-now, daily/hourly |
| 15 | Marine forecast (waves, SST) | cmer81 marine_weather | `marine` command | Wave height/period/direction, SST |
| 16 | Air quality (PM/O3/AQI/UV) | cmer81 air_quality | `air-quality` command | All pollutants, EU+US AQI, pollen, UV |
| 17 | Pollen forecast | cmer81 air_quality | bundled into `air-quality --pollen` | Alder/birch/grass/mugwort/olive/ragweed |
| 18 | Flood / river discharge | cmer81 flood_forecast | `flood` command | GloFAS daily |
| 19 | Climate projections (CMIP6) | cmer81 climate_projection | `climate` command | All models, SSP scenarios |
| 20 | Ensemble forecast | cmer81 ensemble_forecast | `ensemble` command | All members |
| 21 | Seasonal forecast (9 months) | cmer81 seasonal_forecast | `seasonal` command | NCEP CFSv2 |
| 22 | Model-specific forecast (DWD ICON) | cmer81 dwd_icon_forecast | `forecast --models dwd_icon` | All variants |
| 23 | Model-specific forecast (GFS) | cmer81 gfs_forecast | `forecast --models gfs_seamless` | + GFS variants |
| 24 | Model-specific forecast (MeteoFrance) | cmer81 meteofrance_forecast | `forecast --models meteofrance_arpege_world` | AROME + ARPEGE |
| 25 | Model-specific forecast (ECMWF) | cmer81 ecmwf_forecast | `forecast --models ecmwf_ifs04` | All ECMWF |
| 26 | Model-specific forecast (JMA) | cmer81 jma_forecast | `forecast --models jma_seamless` | All JMA |
| 27 | Model-specific forecast (MetNo) | cmer81 metno_forecast | `forecast --models metno_seamless` | All MetNo |
| 28 | Model-specific forecast (GEM Canada) | cmer81 gem_forecast | `forecast --models gem_seamless` | All GEM |
| 29 | Geocoding by name | cmer81 geocoding | `geocode search "Seattle"` | Multi-result, language-aware |
| 30 | Reverse geocoding by ID | open-meteo geocoding-api | `geocode get <id>` | ID → location |
| 31 | Elevation lookup (single) | cmer81 elevation | `elevation` command | Single lat/lon |
| 32 | Elevation lookup (batch) | open-meteo /v1/elevation native | `elevation --batch lat1,lat2 --batch lon1,lon2` | Comma-separated batch (native API support) |
| 33 | IP-based geolocation default | johnallen3d/conditions | `current` (no args) → geolocate via IP, then forecast | One-command UX |
| 34 | WMO weather-code humanization | jeremymorgan | Built-in `cliutil.WeatherCode(N)` helper, default-on for human output | Full WMO 4501 table (28 codes) |
| 35 | Async/non-blocking client | frenck/python-open-meteo | Go's natural goroutine concurrency in fan-out | Better than Python async |
| 36 | Multi-location batch (single call) | Open-Meteo native (CSV coords) | `forecast --place Seattle,Berlin,Tokyo` → batched | Server-side aggregation |
| 37 | Customer-tier routing | none of the surveyed tools | `OPEN_METEO_API_KEY` env var → swap host + apikey | First CLI to do this cleanly |
| 38 | Doctor / health check | (PP standard) | `doctor` command | Auth, reachability, env |
| 39 | Local SQLite cache | (no existing tool) | Auto-cache responses; `sync` to refresh | Generic PP store layer |
| 40 | SQL composability over cached data | (no existing tool) | `sql` command | SELECT-only query over local store |
| 41 | FTS search across cached locations | (no existing tool) | `search "seattle"` | Local FTS5 |

## Transcendence — only possible with our approach

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|---|
| T1 | **Forecast diff** — what changed since the last forecast pull for this location | `forecast diff --place Seattle` | Requires local snapshot history; the API gives "now" only | 9 |
| T2 | **Climate-vs-now comparison** — how does today's weather compare to the 30-year normal for this date | `compare --place Seattle --metric temperature_2m_mean` | Joins archive (1940+) + current forecast in one query; no API gives "anomaly" | 9 |
| T3 | **Activity verdict (free version)** — "Is it good for surfing/skiing/hiking right now?" with thresholds | `is-good-for surfing --place "Pipeline, HI"` | Combines marine + forecast + UV/AQI; no single API call gives a verdict | 8 |
| T4 | **Climate normal** — multi-decade average for any (location, date, variable) | `normals --place Seattle --month 7` | Requires aggregating archive over 30+ years locally | 8 |
| T5 | **Forecast accuracy back-test** — for a past date, did the model predict what actually happened? | `accuracy --place Seattle --date 2025-12-25 --variable temperature_2m_max` | Needs both archive (truth) and a snapshot of past forecast (cache); no API call | 7 |
| T6 | **Multi-location summary** — one-command panel for N locations | `panel --place Seattle,Berlin,Tokyo --metric temperature_2m,precipitation` | Open-Meteo natively supports CSV coords; we add table layout + jq-friendly array | 7 |
| T7 | **Wake-of-storm air-quality check** — after a forecasted storm, what does AQI do? | `forecast-aq --place Seattle` | Cross-API joins forecast + air-quality | 6 |
| T8 | **Marine + tide-aware surf rating** | `surf-report --place "Mavericks, CA"` | Marine wave height + forecast wind + period thresholds | 6 |
| T9 | **WMO code distribution over time window** | `weather-mix --place Seattle --past-days 30` | Aggregates archive WMO codes into "% rain / % clear / % storms" | 6 |

**Score legend (1.5c.5 dimensions):** breadth (cross-API), persistence (cache-only), composition (local + remote), persona-fit (>=2 personas).

### Stubs / honest gaps

None planned. All 9 transcendence features are buildable in-session with the data the API exposes for free.

---

## Counts

- **Absorbed**: 41 features
- **Transcendence**: 9 features
- **Total shipping scope**: 50 features

This significantly exceeds the most comprehensive existing Open-Meteo MCP (cmer81 with 17 tools).
