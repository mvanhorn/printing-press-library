# Open-Meteo CLI Brief

## API Identity

- **Domain**: Free, open-source weather data — forecasts, historicals, marine, air quality, climate, ensembles, flood, geocoding, elevation, satellite radiation. CC-BY 4.0 licensed.
- **Users**: Hobbyist developers (status bars, scripts), researchers/data scientists (ERA5 historicals, climate projections), outdoor activity planners (surfers, farmers, pilots, hikers, skiers), public-health/urban planners (AQI, pollen, UV), hydrologists (river discharge / flood).
- **Data profile**: Free tier needs no API key. Customer tier same endpoints, just different host (`customer-*.open-meteo.com`) + `&apikey=` query param. Rate limits free tier: <10k/day, 5k/hr, 600/min. Non-commercial use only on free tier.
- **Multi-host architecture** — each endpoint family lives on its own subdomain:

| Resource | Free host | Customer host (when `OPEN_METEO_API_KEY` set) | Path |
|---|---|---|---|
| Forecast | `api.open-meteo.com` | `customer-api.open-meteo.com` | `/v1/forecast` |
| Archive (historical ERA5) | `archive-api.open-meteo.com` | `customer-archive-api.open-meteo.com` | `/v1/archive` |
| Marine | `marine-api.open-meteo.com` | `customer-marine-api.open-meteo.com` | `/v1/marine` |
| Air Quality | `air-quality-api.open-meteo.com` | `customer-air-quality-api.open-meteo.com` | `/v1/air-quality` |
| Flood | `flood-api.open-meteo.com` | `customer-flood-api.open-meteo.com` | `/v1/flood` |
| Geocoding (search) | `geocoding-api.open-meteo.com` | `customer-geocoding-api.open-meteo.com` | `/v1/search` |
| Geocoding (by ID) | `geocoding-api.open-meteo.com` | same | `/v1/get` |
| Climate (CMIP6) | `climate-api.open-meteo.com` | `customer-climate-api.open-meteo.com` | `/v1/climate` |
| Ensemble | `ensemble-api.open-meteo.com` | `customer-ensemble-api.open-meteo.com` | `/v1/ensemble` |
| Seasonal | `seasonal-api.open-meteo.com` | same | `/v1/seasonal` |
| Satellite radiation | `satellite-api.open-meteo.com` | same | `/v1/archive` |
| Elevation | `api.open-meteo.com` | same | `/v1/elevation` |

- **Spec status**: Open-Meteo's published `openapi.yml` covers ONLY `/v1/forecast`. To absorb the full surface, this CLI needs a hand-built internal YAML spec with per-resource `base_url` overrides.

## Reachability Risk
- **None.** Free tier is openly accessible without API key. Multiple GitHub MCPs and SDKs are working today. Live smoke testing in Phase 5 will hit the real API.

## Top Workflows
1. **"What's the weather in Seattle this week?"** — Geocode-by-name → forecast call → human or JSON output. The single most common ask.
2. **"What was the temperature on this date 30 years ago?"** — Archive endpoint (ERA5 from 1940). Highly differentiating for researchers and climate-curious users.
3. **"Should I go surfing tomorrow?"** — Marine endpoint: wave height, period, direction, sea-surface temperature.
4. **"Air quality and pollen forecast for my walk"** — Air quality endpoint: PM2.5/PM10, O3, NO2, pollen, US/EU AQI, UV index.
5. **"How much will the climate change here by 2050?"** — Climate endpoint: CMIP6 projections under SSP scenarios.
6. **"Will the river flood next week?"** — Flood endpoint: GloFAS river discharge.

## Table Stakes
- Lat/lon **and** city-name input (geocoding integrated)
- Hourly + daily + current data selection
- WMO weather code → human description ("Clear sky", "Heavy rain", "Thunderstorm with hail")
- Multiple unit systems (°C/°F, km/h vs mph, mm vs inch, hPa vs inHg)
- Timezone handling (auto-detect, UTC, named)
- Variable selection (`--hourly temperature_2m,precipitation,...`)
- `--json` for agents, table/pretty output for humans
- Customer-tier env var routing

## Data Layer
- **Primary entities** (worth caching in SQLite): `forecasts`, `historical_observations`, `air_quality_readings`, `marine_observations`, `flood_readings`, `locations` (geocoded), `weather_codes` (lookup table).
- **Sync cursor**: `(location_id, endpoint, start_date)` triple. Forecast endpoints overwrite within their forecast window; archive/historical is immutable so trivially append-only.
- **FTS/search**: locations table benefits most (search by city name, country, admin1).

## Codebase Intelligence
- Source: GitHub repo browse (open-meteo/open-meteo) + analysis of cmer81/open-meteo-mcp (17 tools — most comprehensive MCP) and jeremymorgan/weather-mcp-server (3 tools — typical).
- **Auth**: NONE for free tier. Optional `apikey` query param for customer tier. No bearer token, no OAuth.
- **Data model**: Time-series-shaped responses. Top-level keys: `latitude`, `longitude`, `elevation`, `timezone`, `hourly` (object of arrays), `daily` (object of arrays), `current`, `hourly_units`, `daily_units`, `current_units`. Always paired arrays — `hourly.time[i]` aligns with `hourly.temperature_2m[i]`.
- **Rate limiting**: Soft per-day/hour/minute caps. 429 returned when exceeded.
- **Architecture**: Each endpoint family is a separate microservice on its own subdomain. Some support `models=` for model selection (DWD ICON, GFS, ECMWF, JMA, MetNo, MétéoFrance, GEM). FlatBuffers binary protocol available as opt-in for big payloads, but JSON is default and easier for a CLI.

## User Vision
- User wants **free tier only by default**, with a clean upgrade path: if `OPEN_METEO_API_KEY` env var is set, the CLI auto-routes to `customer-*.open-meteo.com` and appends `apikey=$KEY` to every request. Same commands, same flags, same output.

## Product Thesis
- **Name**: `open-meteo-pp-cli`
- **Why it should exist**:
  - Nearly every existing Open-Meteo wrapper covers only `/v1/forecast`. Marine, archive, air quality, flood, climate, ensemble, satellite are perpetually under-tooled.
  - Existing CLIs return raw WMO codes (e.g., `61` instead of `Slight rain`) and force users to look up coordinates manually.
  - No CLI offers offline storage of forecasts/observations, so users can't ask "what changed since yesterday's forecast?" or "trend over the last 30 days."
  - The customer-tier routing pattern (free by default, env var unlocks paid) is a feature no current Open-Meteo tool offers cleanly.

## Build Priorities
1. **Internal YAML spec** with all 11 endpoint families and per-resource `base_url` overrides. Customer-tier routing wired into the generated client (env-var-driven host swap + apikey append).
2. **Geocoding-integrated commands** — every command that takes `--latitude/--longitude` should also accept `--place "City, Country"` which resolves via the geocoding endpoint first.
3. **WMO weather code lookup** — built-in table, `--humanize` flag and human-output default.
4. **Local SQLite cache** — store every fetched forecast/observation. Diff/trend/since commands work offline.
5. **Novel features (Phase 1.5)** — diff against yesterday's forecast, "since-last-snapshot" deltas, multi-location batch (Open-Meteo natively supports comma-separated coords), seasonal narrative, climate-vs-now comparison.

## Notes for Phase 1.5 Absorb
Top tools to absorb features from (full list in absorb manifest):
- cmer81/open-meteo-mcp (17 tools, most comprehensive — includes per-model selection)
- isdaniel/mcp_weather_server, JeremyMorgan/Weather-MCP-Server, nprousalidis/Open-Meteo-MCP-Server, gbrigandi/mcp-server-openmeteo
- chuk-mcp-open-meteo (PyPI), open-meteo-mcp (PyPI)
- @openmeteo/sdk, openmeteo, @atombrenner/openmeteo (npm)
- openmeteo-requests, openmeteo-py, openmeteopy, frenck/python-open-meteo (PyPI)
- johnallen3d/conditions (Rust CLI — IP-based geolocation pattern worth absorbing)
- R366Y/weather_cli, Hitori-Laura/weather-fetch (Python CLIs)
- AntoinePinto/weather-data
