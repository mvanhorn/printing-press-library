# Open-Meteo CLI

**Every Open-Meteo endpoint family in one CLI — forecast, archive, marine, air quality, flood, climate, ensemble, seasonal, geocoding, elevation.**

Comprehensive coverage of Open-Meteo's free, no-API-key tier across all 11 endpoint families. City-name input via integrated geocoding, WMO code humanization, and a local SQLite cache that powers commands no upstream tool can: forecast diff, climate-vs-now compare, activity verdicts, climate normals.

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/open-meteo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-cli@latest
```

## Authentication

No API key required. The free tier supports up to ~10,000 requests per day for non-commercial use. If you have an Open-Meteo customer-tier subscription, set OPEN_METEO_API_KEY in the environment and the CLI auto-routes to customer-*.open-meteo.com with the apikey appended — same commands, same flags, higher limits, commercial use allowed.

## Quick Start

```bash
# Resolve a place name to coordinates (used by spec-derived commands).
# Novel commands (panel, compare, normals, weather-mix, forecast diff,
# accuracy, is-good-for) accept --place natively.
open-meteo-pp-cli geocode search --name Seattle --count 1 --json


# Current conditions for a city by name (panel takes --place natively).
open-meteo-pp-cli panel --place Seattle --current temperature_2m,weather_code --humanize


# 7-day daily forecast as JSON.
open-meteo-pp-cli forecast --latitude 47.6062 --longitude -122.3321 --daily temperature_2m_max,temperature_2m_min,precipitation_sum --forecast-days 7 --json


# Historical ERA5 data — works for any date back to 1940.
open-meteo-pp-cli archive --latitude 47.6062 --longitude -122.3321 --start-date 2024-01-01 --end-date 2024-01-31 --daily temperature_2m_max --json


# Climate-vs-now anomaly: how today compares to the 30-year normal (--place native).
open-meteo-pp-cli compare --place Seattle --metric temperature_2m_mean --years 30 --json


# Side-by-side multi-location panel in one batched call.
open-meteo-pp-cli panel --place Seattle,Berlin,Tokyo --current temperature_2m,weather_code --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`forecast diff`** — See exactly what changed since the last forecast pull for this location — temperature deltas, new precipitation hours, weather-code shifts.

  _Pick this when an agent needs to react to forecast changes (alerting, replanning) rather than to the current forecast itself._

  ```bash
  open-meteo-pp-cli forecast diff --place Seattle --json --select changes
  ```
- **`normals`** — Compute the 30-year (or user-specified) climate normal for any (location, day-of-year, variable) tuple.

  _Use to ground a 'normal vs current' answer. Also useful for travel/timing recommendations across decades._

  ```bash
  open-meteo-pp-cli normals --place Seattle --month 7 --variable temperature_2m_max --years 30 --json
  ```
- **`accuracy`** — For any past date, compare what the model predicted (cached snapshot) against what actually happened (archive ground truth).

  _Use to score forecast trust over time, or to debug a 'why did the forecast change?' question._

  ```bash
  open-meteo-pp-cli accuracy --place Seattle --date 2025-12-25 --variable temperature_2m_max --json
  ```
- **`weather-mix`** — Distribution of WMO weather conditions (% clear, % rain, % storms, etc.) over an arbitrary historical window for a place.

  _Use for travel planning, climatology summaries, or 'how often does it rain in November?' questions._

  ```bash
  open-meteo-pp-cli weather-mix --place Seattle --past-days 30 --json
  ```

### Cross-API joins
- **`compare`** — Compare today's weather (or any forecast hour) against the 30-year climate normal for that date and place.

  _Use when a user asks 'is this weather unusual?' or 'how does this compare to historical?' — a single API call cannot answer._

  ```bash
  open-meteo-pp-cli compare --place Seattle --metric temperature_2m_mean --years 30 --json
  ```
- **`is-good-for`** — Verdict (GO / CAUTION / STOP) for surfing, skiing, hiking, running, biking based on combined forecast + marine + air-quality thresholds.

  _Use when the user asks a decision question, not a data question. Saves the agent from synthesizing 3 endpoint calls into a verdict._

  ```bash
  open-meteo-pp-cli is-good-for surfing --place "Half Moon Bay, CA" --json
  ```

### Agent-native plumbing
- **`panel`** — One-command snapshot panel for N locations side-by-side — batched in a single Open-Meteo call when possible.

  _Use to compare conditions across locations in one shot; cheaper and faster than N separate calls._

  ```bash
  open-meteo-pp-cli panel --place Seattle,Berlin,Tokyo --current temperature_2m,wind_speed_10m,weather_code --json
  ```

## Usage

Run `open-meteo-pp-cli --help` for the full command reference and flag list.

## Commands

### air-quality

Air quality and pollen: PM2.5, PM10, ozone, NO2, SO2, CO, dust, pollens, EU/US AQI, UV.

- **`open-meteo-pp-cli air-quality get`** - Air quality forecast and historical: pollutants, AQI, pollen, UV.

### archive

Historical weather data (ERA5 reanalysis, 1940 to ~5 days ago). Same variable surface as forecast.

- **`open-meteo-pp-cli archive get`** - Historical hourly/daily weather observations from ERA5 (1940 to ~5 days ago).

### climate

CMIP6 climate change projections (1950-2050) under SSP scenarios.

- **`open-meteo-pp-cli climate get`** - Daily CMIP6 climate projections for any (lat, lon) under SSP scenarios. Models include EC_Earth3P_HR, CMCC_CM2_VHR4, MRI_AGCM3_2_S, MPI_ESM1_2_XR, FGOALS_f3_H, NICAM16_8S, HiRAM_SIT_HR.

### elevation

Digital elevation model lookup. Supports up to 100 coordinates per call.

- **`open-meteo-pp-cli elevation get`** - Lookup elevation in meters for one or more coordinate pairs (up to 100).

### ensemble

Ensemble forecasts (all model members) for uncertainty quantification.

- **`open-meteo-pp-cli ensemble get`** - Hourly ensemble forecasts; returns all ensemble members so you can quantify uncertainty.

### flood

River discharge / flood forecasts (GloFAS).

- **`open-meteo-pp-cli flood get`** - Daily river discharge forecasts and historicals from GloFAS.

### forecast

Weather forecast (up to 16 days). Free, no API key, hourly + daily + current resolution. Default model is best-available; pass --models for specific models (gfs_seamless, ecmwf_ifs025, dwd_icon, jma_seamless, metno_seamless, gem_seamless, meteofrance_arpege_world).

- **`open-meteo-pp-cli forecast get`** - 7-16 day weather forecast for one or more coordinates. Pass comma-separated latitude/longitude for batch fetch.

### geocode

Geocoding: search locations by name/postcode, or look up a location by ID.

- **`open-meteo-pp-cli geocode get`** - Look up a single location by its Open-Meteo geocoding ID.
- **`open-meteo-pp-cli geocode search`** - Search locations worldwide by name ("Berlin", "Springfield, IL") or postcode.

### marine

Marine weather: wave height, period, direction, swell components, sea-surface temperature.

- **`open-meteo-pp-cli marine get`** - Marine forecast/historical: waves, swells, sea-surface temperature.

### seasonal

Seasonal forecasts (up to 9 months) using NCEP CFSv2.

- **`open-meteo-pp-cli seasonal get`** - 9-month seasonal forecast at six-hourly resolution. Default model is NCEP CFSv2.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
open-meteo-pp-cli air-quality

# JSON for scripting and agents
open-meteo-pp-cli air-quality --json

# Filter to specific fields
open-meteo-pp-cli air-quality --json --select id,name,status

# Dry run — show the request without sending
open-meteo-pp-cli air-quality --dry-run

# Agent mode — JSON + compact + no prompts in one flag
open-meteo-pp-cli air-quality --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `OPEN_METEO_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-open-meteo -g
```

Then invoke `/pp-open-meteo <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-mcp@latest
```

Then register it:

```bash
claude mcp add open-meteo open-meteo-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/open-meteo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "open-meteo": {
      "command": "open-meteo-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
open-meteo-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/open-meteo-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **429 "Daily API request limit exceeded"** — Free tier caps ~10k/day, 5k/hr, 600/min. Wait, or set OPEN_METEO_API_KEY for the customer tier.
- **"reason":"Cannot initialize a Float from invalid Float value 'nan'"** — Some variables are missing for some hours (e.g., night-time UV). Drop the variable or filter: --select hourly.time,hourly.temperature_2m
- **Place name returns wrong location** — Disambiguate with comma-separated context: --place "Springfield, IL, US". Or look up by ID: open-meteo-pp-cli geocode search Springfield --count 10
- **Hourly arrays look misaligned** — All hourly/daily arrays index-align with .time. Use --select hourly.time,hourly.temperature_2m to keep them paired in agent output.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**cmer81/open-meteo-mcp**](https://github.com/cmer81/open-meteo-mcp) — TypeScript
- [**isdaniel/mcp_weather_server**](https://github.com/isdaniel/mcp_weather_server) — Python
- [**JeremyMorgan/Weather-MCP-Server**](https://github.com/jeremymorgan/weather-mcp-server) — Python
- [**open-meteo/python-requests**](https://github.com/open-meteo/python-requests) — Python
- [**frenck/python-open-meteo**](https://github.com/frenck/python-open-meteo) — Python
- [**m0rp43us/openmeteopy**](https://github.com/m0rp43us/openmeteopy) — Python
- [**johnallen3d/conditions**](https://github.com/johnallen3d/conditions) — Rust
- [**R366Y/weather_cli**](https://github.com/R366Y/weather_cli) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
