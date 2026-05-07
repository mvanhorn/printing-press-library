# Weather Goat CLI

The weather CLI that answers the questions you actually ask: Should I bike today? Do I need an umbrella for the walk home? Is there a tornado warning? Is this summer hotter than normal?

Powered by Open-Meteo (global forecasts, 80 years of history, air quality) and NWS (severe weather alerts). No API key required.

## Install

The recommended path installs both the `weather-goat-pp-cli` binary and the `pp-weather-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install weather-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install weather-goat --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/weather-goat/cmd/weather-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/weather-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-weather-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-weather-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-weather-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-weather-goat. The skill defines how its required CLI can be installed.
```

## Authentication

Open-Meteo is a free API — no API key required. NWS alerts also require no authentication.

## Quick Start

```bash
# 1. Set your default location
weather-goat-pp-cli config set-location "Seattle"

# 2. Check your setup
weather-goat-pp-cli doctor

# 3. Get a full weather brief (current conditions + forecast + alerts)
weather-goat-pp-cli

# 4. Should I bike today?
weather-goat-pp-cli go bike

# 5. Check air quality before going outside
weather-goat-pp-cli breathe
```

## What Makes This Different

Every weather CLI shows you temperature and precipitation. Weather Goat answers the questions that actually drive decisions.

**"Should I bike today?"** — `go bike` checks wind, rain, temperature, ice, and air quality. Returns GO, CAUTION, or STOP with specific reasons.

**"Do I need an umbrella for the walk home?"** — `go commute` checks the forecast at your departure AND return times. "Clear this morning, rain by 5pm — take an umbrella."

**"Is there a severe weather warning?"** — `alerts` pulls active NWS warnings (tornado, storm, flood, heat). `watch` polls every 60 seconds during active weather.

**"Is this summer actually hotter than normal?"** — `normal` compares today to the historical average for this date. "Today is 8°F above average for April 11."

**"Beach or mountains this weekend?"** — `compare "Malibu" "Big Bear"` shows side-by-side forecasts.

**"Is the air quality safe to run in?"** — `breathe` shows AQI, pollen, UV, and whether it's safe to exercise outdoors.

## Commands

### Weather & Forecasts

| Command | Description |
|---------|-------------|
| `weather-goat-pp-cli` | Morning brief — current conditions, today's forecast, active alerts |
| `forecast` | Current weather and daily forecast (temperature, precip, sunrise/sunset) |
| `forecast hourly` | Hourly forecast for the next 48 hours |
| `history` | Historical weather data for any date back to 1940 |
| `normal` | Compare today's temperature to the historical average |
| `compare` | Side-by-side weather comparison of two locations |

### Activity Verdicts

| Command | Description |
|---------|-------------|
| `go walk` | Walking prep advice — umbrella, layers, sunscreen |
| `go bike` | GO/CAUTION/STOP for wind, rain, temperature, AQI |
| `go hike` | GO/CAUTION/STOP for thunderstorms, hypothermia, UV, wind |
| `go commute` | Morning vs evening forecast with umbrella advice |
| `go drive` | GO/CAUTION/STOP for visibility, ice, wind, NWS warnings |

### Air Quality & Alerts

| Command | Description |
|---------|-------------|
| `air-quality` | AQI, PM2.5, PM10, pollen, UV index |
| `breathe` | Air quality + pollen + UV with exercise safety recommendation |
| `alerts` | Active NWS weather alerts for a location or state |
| `watch` | Continuously poll NWS alerts and print new ones |

### Location & Data

| Command | Description |
|---------|-------------|
| `geocoding` | Search for a location by name and get coordinates |
| `sync` | Sync API data to local SQLite for offline access |
| `analytics` | Run analytics queries on locally synced data |
| `export` | Export data to JSONL or JSON |
| `import` | Import data from JSONL file |

### Utilities

| Command | Description |
|---------|-------------|
| `config set-location` | Save your default location |
| `config set-commute` | Save commute departure and return times |
| `config show` | Display current configuration |
| `doctor` | Check CLI health and connectivity |
| `api` | Browse all API endpoints |
| `workflow` | Compound workflows combining multiple operations |
| `auth` | Manage authentication tokens |

## Output Formats

```bash
# Human-readable (default in terminal, JSON when piped)
weather-goat-pp-cli forecast --latitude 47.6 --longitude -122.3

# JSON for scripting and agents
weather-goat-pp-cli forecast --latitude 47.6 --longitude -122.3 --json

# Filter to specific fields
weather-goat-pp-cli forecast --latitude 47.6 --longitude -122.3 --json --select temperature_2m,wind_speed_10m

# CSV output
weather-goat-pp-cli history --latitude 47.6 --longitude -122.3 --start-date 2024-01-01 --end-date 2024-01-31 --csv

# Compact output (key fields only, for agents)
weather-goat-pp-cli geocoding "Seattle" --compact

# Dry run — show the request without sending
weather-goat-pp-cli forecast --latitude 47.6 --longitude -122.3 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
weather-goat-pp-cli go bike "Portland" --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** — never prompts, every input is a flag
- **Pipeable** — `--json` output to stdout, errors to stderr
- **Filterable** — `--select id,name` returns only fields you need
- **Previewable** — `--dry-run` shows the request without sending
- **Confirmable** — `--yes` for explicit confirmation of destructive actions
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** — no colors or formatting unless `--human-friendly` is set
- **Progress events** — paginated commands emit NDJSON events to stderr

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add weather weather-goat-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "weather": {
      "command": "weather-goat-pp-mcp"
    }
  }
}
```

## Cookbook

```bash
# Morning brief — what's the weather right now?
weather-goat-pp-cli

# Should I bike to work today?
weather-goat-pp-cli go bike

# Compare two cities for a weekend trip
weather-goat-pp-cli compare "San Francisco" "Los Angeles"

# Is this temperature unusual for this time of year?
weather-goat-pp-cli normal

# Check air quality before an outdoor run
weather-goat-pp-cli breathe

# Get NWS alerts for all of California
weather-goat-pp-cli alerts --state CA

# Watch for severe weather during a storm
weather-goat-pp-cli watch "Oklahoma City" --interval 30

# Historical weather on a specific date
weather-goat-pp-cli history --latitude 47.6 --longitude -122.3 --start-date 2024-07-04 --end-date 2024-07-04

# Commute forecast (set up times first)
weather-goat-pp-cli config set-commute 08:00 18:00
weather-goat-pp-cli go commute

# Hourly forecast as JSON for scripting
weather-goat-pp-cli forecast hourly --latitude 40.7 --longitude -74.0 --json

# Pipe activity verdict to another tool
weather-goat-pp-cli go drive --agent | jq '.verdict'

# Sync data locally for offline access
weather-goat-pp-cli sync

# Export synced data for analysis
weather-goat-pp-cli export --format jsonl > weather-backup.jsonl
```

## Health Check

```bash
weather-goat-pp-cli doctor
```

```
  OK Config: ok
  WARN Auth: not required
  OK API: reachable
  config_path: ~/.config/weather-goat-pp-cli/config.toml
  base_url: https://api.open-meteo.com/v1
```

## Configuration

Config file: `~/.config/weather-goat-pp-cli/config.toml`

| Setting | Description |
|---------|-------------|
| `WEATHER_CONFIG` | Override config file path |
| `WEATHER_BASE_URL` | Override API base URL (for self-hosted Open-Meteo) |

Config file fields:

| Field | Description |
|-------|-------------|
| `base_url` | API base URL (default: `https://api.open-meteo.com/v1`) |
| `latitude` | Default latitude for location-based commands |
| `longitude` | Default longitude for location-based commands |
| `location_name` | Display name for the saved location |
| `commute_depart_time` | Departure time for commute forecast (e.g., `08:00`) |
| `commute_return_time` | Return time for commute forecast (e.g., `18:00`) |

## Self-Hosting

If you run a self-hosted Open-Meteo instance:

```bash
export WEATHER_BASE_URL=https://your-open-meteo-instance.example.com/v1
```

Or set `base_url` in your config file.

## Troubleshooting

**Authentication errors (exit code 4)**
- Open-Meteo is free and doesn't require authentication
- Run `weather-goat-pp-cli doctor` to check connectivity

**Not found errors (exit code 3)**
- Check the location name or coordinates are correct
- Try `weather-goat-pp-cli geocoding "city name"` to verify coordinates

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Open-Meteo allows 10,000 requests/day on the free tier
- If persistent, wait a few minutes or use `--rate-limit 1`

**No location configured**
- Run `weather-goat-pp-cli config set-location "City Name"` to save a default
- Or pass a location inline: `weather-goat-pp-cli go walk "Denver"`

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wttr.in**](https://github.com/chubin/wttr.in) — Python
- [**open-meteo-mcp**](https://github.com/cmer81/open-meteo-mcp) — TypeScript
- [**wthrr**](https://github.com/ttytm/wthrr-the-weathercrab) — Rust

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `weather-goat-pp-cli profile save <name> --<flag> <value>`, then `weather-goat-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.weather-goat-pp-cli/feedback.jsonl`. When `WEATHER_GOAT_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `WEATHER_GOAT_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
