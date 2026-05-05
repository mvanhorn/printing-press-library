---
name: pp-open-meteo
description: "Every Open-Meteo endpoint family in one CLI — forecast, archive, marine, air quality, flood, climate, ensemble, seasonal, geocoding, elevation. Trigger phrases: `what's the weather in`, `forecast for`, `is it going to rain`, `marine forecast`, `air quality in`, `historical weather`, `climate normal`, `use open-meteo`, `run open-meteo`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - open-meteo-pp-cli
    install:
      - kind: go
        bins: [open-meteo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-cli
---

# Open-Meteo — Printing Press CLI

Comprehensive coverage of Open-Meteo's free, no-API-key tier across all 11 endpoint families. City-name input via integrated geocoding, WMO code humanization, and a local SQLite cache that powers commands no upstream tool can: forecast diff, climate-vs-now compare, activity verdicts, climate normals.

## When to Use This CLI

Use this CLI for any weather, climate, or air-quality question that does not require a paid commercial agreement. It covers forecasts (16 days, multiple models), historicals (ERA5 from 1940), marine waves, air quality with pollen, river flood discharge, CMIP6 climate projections, and ensemble runs. Reach for the novel commands (compare, normals, weather-mix, panel, is-good-for) when the user is asking a decision or anomaly question rather than a raw-data question.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**air-quality** — Air quality and pollen: PM2.5, PM10, ozone, NO2, SO2, CO, dust, pollens, EU/US AQI, UV.

- `open-meteo-pp-cli air-quality` — Air quality forecast and historical: pollutants, AQI, pollen, UV.

**archive** — Historical weather data (ERA5 reanalysis, 1940 to ~5 days ago). Same variable surface as forecast.

- `open-meteo-pp-cli archive` — Historical hourly/daily weather observations from ERA5 (1940 to ~5 days ago).

**climate** — CMIP6 climate change projections (1950-2050) under SSP scenarios.

- `open-meteo-pp-cli climate` — Daily CMIP6 climate projections for any (lat, lon) under SSP scenarios. Models include EC_Earth3P_HR, CMCC_CM2_VHR4,...

**elevation** — Digital elevation model lookup. Supports up to 100 coordinates per call.

- `open-meteo-pp-cli elevation` — Lookup elevation in meters for one or more coordinate pairs (up to 100).

**ensemble** — Ensemble forecasts (all model members) for uncertainty quantification.

- `open-meteo-pp-cli ensemble` — Hourly ensemble forecasts; returns all ensemble members so you can quantify uncertainty.

**flood** — River discharge / flood forecasts (GloFAS).

- `open-meteo-pp-cli flood` — Daily river discharge forecasts and historicals from GloFAS.

**forecast** — Weather forecast (up to 16 days). Free, no API key, hourly + daily + current resolution. Default model is best-available; pass --models for specific models (gfs_seamless, ecmwf_ifs025, dwd_icon, jma_seamless, metno_seamless, gem_seamless, meteofrance_arpege_world).

- `open-meteo-pp-cli forecast` — 7-16 day weather forecast for one or more coordinates. Pass comma-separated latitude/longitude for batch fetch.

**geocode** — Geocoding: search locations by name/postcode, or look up a location by ID.

- `open-meteo-pp-cli geocode get` — Look up a single location by its Open-Meteo geocoding ID.
- `open-meteo-pp-cli geocode search` — Search locations worldwide by name ('Berlin', 'Springfield, IL') or postcode.

**marine** — Marine weather: wave height, period, direction, swell components, sea-surface temperature.

- `open-meteo-pp-cli marine` — Marine forecast/historical: waves, swells, sea-surface temperature.

**seasonal** — Seasonal forecasts (up to 9 months) using NCEP CFSv2.

- `open-meteo-pp-cli seasonal` — 9-month seasonal forecast at six-hourly resolution. Default model is NCEP CFSv2.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `OPEN_METEO_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
open-meteo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pick a slim forecast slice for an agent

```bash
open-meteo-pp-cli forecast --latitude 47.6062 --longitude -122.3321 --hourly temperature_2m,precipitation,weather_code --forecast-days 3 --json --select hourly.time,hourly.temperature_2m,hourly.weather_code
```

Use --select with dotted paths to narrow deeply nested time-series arrays. Open-Meteo responses can be tens of KB; this trims them to the columns you actually need.

### Diff what changed in tomorrow's forecast

```bash
open-meteo-pp-cli forecast diff --place Seattle --json
```

Compares the current forecast against the cached prior pull for this place and emits only the changed hours.

### Anomaly check: is this temperature unusual?

```bash
open-meteo-pp-cli compare --place Seattle --metric temperature_2m_mean --years 30 --json
```

Joins the 30-year ERA5 archive normal for the date and place against the current forecast.

### Side-by-side panel for travel planning

```bash
open-meteo-pp-cli panel --place "Seattle,Berlin,Tokyo" --current temperature_2m,weather_code --json
```

Open-Meteo accepts comma-separated coords natively; the CLI batches the call and labels each result by place.

### How often does it rain in October in Seattle?

```bash
open-meteo-pp-cli weather-mix --place Seattle --start-date 2024-10-01 --end-date 2024-10-31 --json
```

Aggregates archive hourly WMO codes into a category histogram (% clear / % rain / % storms / etc.).

## Auth Setup

No authentication required.

Run `open-meteo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  open-meteo-pp-cli air-quality --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
open-meteo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
open-meteo-pp-cli feedback --stdin < notes.txt
open-meteo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.open-meteo-pp-cli/feedback.jsonl`. They are never POSTed unless `OPEN_METEO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPEN_METEO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
open-meteo-pp-cli profile save briefing --json
open-meteo-pp-cli --profile briefing air-quality
open-meteo-pp-cli profile list --json
open-meteo-pp-cli profile show briefing
open-meteo-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `open-meteo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-cli@main
   ```
3. Verify: `open-meteo-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/other/open-meteo/cmd/open-meteo-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add open-meteo-pp-mcp -- open-meteo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which open-meteo-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   open-meteo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `open-meteo-pp-cli <command> --help`.
