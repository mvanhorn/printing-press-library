---
name: pp-weather-goat
description: "Use this skill whenever the user asks about weather, forecasts, temperature, rain, storms, severe weather alerts, air quality, pollen, UV, or wants an activity recommendation (can I walk / bike / hike / commute / drive given the weather). Weather CLI powered by Open-Meteo (global, no auth, unlimited) + NWS (US severe weather). No API key. Triggers on phrasings like 'what's the weather', 'is it going to rain today', 'any storms coming', 'should I bike to work', 'how's the air quality', 'compare NYC and LA weather this weekend', 'is this unusually hot for April'."
version: "1.3.3-0.20260411222622-065697576de1+dirty"
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - weather-goat-pp-cli
    install:
      - kind: go
        bins: [weather-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/weather-goat/cmd/weather-goat-pp-cli
---

# Weather Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `weather-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install weather-goat --cli-only
   ```
2. Verify: `weather-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/weather-goat/cmd/weather-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this when a user wants a weather lookup, storm or alert check, air-quality read, activity recommendation (bike, hike, walk, commute, drive) based on current conditions, or a "is today normal for this time of year" comparison. Particularly valuable as a morning-briefing source with `weather-goat-pp-cli` (no args → current + today's forecast + active alerts for your configured home location).

Don't reach for this when the user needs hyperlocal or commercial-grade meteorology (use a paid provider like Tomorrow.io or ECMWF), or for international severe-weather alerts (NWS is US-only).

## Unique Capabilities

### Activity verdicts — the differentiator

- **`go walk|bike|hike|commute|drive [location]`** — GO / CAUTION / STOP verdict for a specific activity, with reasoning.

  _Turns "look at the weather" into "what should I do." Each activity has its own thresholds._

  - **walk** — preparation advice (umbrella? jacket? sunscreen? layers?)
  - **bike** — wind, rain, AQI, ice thresholds tuned for cycling
  - **hike** — lightning, hypothermia risk, altitude UV
  - **commute** — compares AM departure vs PM return weather (time-aware)
  - **drive** — visibility, ice, wind gusts, NWS warnings

### Severe weather intelligence

- **`alerts [location]`** — Active NWS warnings: tornado, storm, flood, heat. Structured output with severity and area polygons.

- **`watch [location]`** — Poll NWS every 60 seconds during active severe weather. Prints new/updated alerts as they arrive. For running during a storm.

### Context and comparison

- **`normal [location]`** — Is today normal for this time of year? Compares today's high/low/precip against historical average for the same date.

- **`compare <location1> <location2> [...]`** — Side-by-side forecast for 2+ locations. Useful for travel planning or "which city is the weather better this weekend."

- **`morning-brief`** (or just `weather-goat-pp-cli` with no args, if `config set-location` was run) — Current conditions + today's forecast + active NWS alerts in one call. The daily-kickoff command.

### Air, UV, and pollen

- **`breathe [location]`** — AQI + pollen + UV with an exercise recommendation.

- **`air-quality [location]`** — Detailed AQI with PM2.5, ozone, etc.

- **`weather_codes`** — Reference for Open-Meteo weather code mappings (internal but useful when working with `--json` output).

## Command Reference

Core:

- `weather-goat-pp-cli` — Morning brief (requires `config set-location` first, or pass `[location]`)
- `weather-goat-pp-cli forecast [location]` — Multi-day forecast
- `weather-goat-pp-cli alerts [location]` — Active NWS severe weather
- `weather-goat-pp-cli history [location]` — Historical weather

Activity:

- `weather-goat-pp-cli go <activity> [location]` — `walk`, `bike`, `hike`, `commute`, `drive`

Air / environment:

- `weather-goat-pp-cli breathe [location]` — AQI + pollen + UV brief
- `weather-goat-pp-cli air-quality [location]` — Detailed AQI

Context:

- `weather-goat-pp-cli normal [location]` — Today vs historical average
- `weather-goat-pp-cli compare <loc1> <loc2>` — Side-by-side
- `weather-goat-pp-cli watch [location]` — Live NWS monitoring loop

Config + utility:

- `weather-goat-pp-cli config set-location <place>` — Set home location
- `weather-goat-pp-cli geocoding <name>` — Resolve place names to coordinates
- `weather-goat-pp-cli doctor` — Verify connectivity

## Recipes

### "Should I bike to work today?"

```bash
weather-goat-pp-cli go bike --agent
```

Returns a structured GO/CAUTION/STOP verdict with reasoning — wind speed, precipitation, AQI, temperature, ice risk — weighted for cycling specifically.

### Morning briefing after setting home location

```bash
weather-goat-pp-cli config set-location "Seattle, WA"
weather-goat-pp-cli --agent  # morning brief — no args
```

Once location is saved, the no-arg invocation returns current conditions + today's forecast + active NWS alerts. Run from a shell startup script or cron at 7am.

### Travel decision — which city is better this weekend?

```bash
weather-goat-pp-cli compare "Portland, OR" "San Francisco, CA" --days 3 --agent
```

Side-by-side 3-day forecast for both. One glance picks the trip destination.

### Watch for severe weather during a warning

```bash
weather-goat-pp-cli watch --agent &  # background poll every 60s
```

Runs indefinitely; prints JSON events to stdout when alerts change. Pipe into a logging tool or Slack webhook for alerting.

### Is today unusually hot?

```bash
weather-goat-pp-cli normal --agent
```

Returns today's high/low/precip alongside the 30-year historical average for the same calendar date. Easy "is this a heat wave" check.

## Auth Setup

**None required.** Open-Meteo is free and unlimited; NWS is free and US-public-data. The `auth` command exists for consistency but is a no-op.

Optional config:
- `WEATHER_GOAT_CONFIG` — override config file path
- Home location persisted to `~/.config/weather-goat-pp-cli/config.toml` via `config set-location`

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes`. Use `--days N` for forecast range on relevant commands, `--no-cache` to bypass the 15-minute GET cache.

### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
weather-goat-pp-cli <command> --agent --select id,name
weather-goat-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Location not found (geocoding failed) |
| 5 | API error (Open-Meteo or NWS issue) |
| 7 | Rate limited (very rare; Open-Meteo is unlimited) |

## Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/other/weather-goat/cmd/weather-goat-pp-cli@latest
weather-goat-pp-cli config set-location "Your City, ST"
weather-goat-pp-cli doctor
```

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/other/weather-goat/cmd/weather-goat-pp-mcp@latest
claude mcp add weather-goat-pp-mcp -- weather-goat-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `weather-goat-pp-cli --help`
2. **`install`** → CLI; **`install mcp`** → MCP
3. **Activity queries ("should I bike/walk/hike...")** → `go <activity> [location] --agent`
4. **Severe weather ("any tornado warnings", "storms coming")** → `alerts [location] --agent`
5. **Comparison ("better weather in X or Y")** → `compare <loc1> <loc2> --agent`
6. **Anything else** → `forecast [location]` (or morning brief if no location passed and home is configured)

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
weather-goat-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
weather-goat-pp-cli --profile <name> <command>

# List / inspect / remove
weather-goat-pp-cli profile list
weather-goat-pp-cli profile show <name>
weather-goat-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
weather-goat-pp-cli <command> --deliver file:/path/to/out.json
weather-goat-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
weather-goat-pp-cli feedback "what surprised you or tripped you up"
weather-goat-pp-cli feedback list         # show local entries
weather-goat-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.weather-goat-pp-cli/feedback.jsonl` as JSON lines. When `WEATHER_GOAT_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `WEATHER_GOAT_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

