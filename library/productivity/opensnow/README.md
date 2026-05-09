# OpenSnow CLI

**Every OpenSnow forecast, snow report, and Daily Snow post — plus powder scoring, storm tracking, and historical trends no other tool has.**

The OpenSnow CLI puts the same hyper-local mountain forecasts trusted by millions of skiers directly into your terminal. Sync resort data to a local SQLite store, then score powder days, rank resorts, and track storms offline. The Daily Snow digest brings expert meteorologist forecasts to your terminal without opening a browser.

## Install

The recommended path installs both the `opensnow-pp-cli` binary and the `pp-opensnow` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install opensnow
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install opensnow --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/opensnow/cmd/opensnow-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/opensnow-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-opensnow --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-opensnow --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-opensnow skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-opensnow. The skill defines how its required CLI can be installed.
```

## Authentication

OpenSnow API access is partnership-only. Set your API key with `opensnow-pp-cli auth set-token <key>`. The key is stored locally and passed as a query parameter on every request. Access level 5 covers point-based forecasts; level 10 unlocks named locations, snow reports, and Daily Snow.

Alternatively, set the environment variable:

```bash
export OPENSNOW_API_KEY=your-api-key
```

Contact hello@opensnow.com to request API partnership access.

## Quick Start

```bash
# Set your OpenSnow API key (partnership access required)
opensnow-pp-cli auth set-token YOUR_API_KEY

# Verify API key is valid and API is reachable
opensnow-pp-cli doctor

# Get the 5-day forecast for Alta
opensnow-pp-cli forecast get-detail alta

# Get Steamboat's latest snow report
opensnow-pp-cli snow-report steamboat

# Read the Colorado Daily Snow forecast
opensnow-pp-cli daily-reads content get-daily-snow colorado

# Save your favorite resorts
opensnow-pp-cli favorites add alta steamboat telluride

# See your dashboard
opensnow-pp-cli dashboard
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Powder intelligence
- **`powder-score`** — Rate upcoming days 1-10 for powder quality by combining forecast snow totals, wind, temperature, and historical averages

  _When an agent needs to recommend the single best day to ski this week, this command gives a scored ranking instead of raw forecast numbers_

  ```bash
  opensnow-pp-cli powder-score alta --days 5 --agent
  ```
- **`powder-rank`** — Rank all synced resorts by powder potential, combining expected snowfall, base depth, and terrain openness

  _When an agent needs to answer 'where should I ski this weekend in Colorado', this gives a ranked list with scores_

  ```bash
  opensnow-pp-cli powder-rank --slugs vail,aspen,telluride --agent
  ```
- **`storm-track`** — Show storm progression — when snow starts, peaks, and ends — by correlating rolling hourly forecasts over time

  _When planning travel around a storm, this shows exactly when to arrive and when the storm window closes_

  ```bash
  opensnow-pp-cli storm-track telluride --agent
  ```
- **`overnight`** — Check the semi-daily forecast for the overnight period at favorited resorts — the powder hunter's morning ritual

  _The first question every skier asks: 'How much snow fell overnight?' This answers it for all favorites in one call_

  ```bash
  opensnow-pp-cli overnight --agent
  ```

### Local state that compounds
- **`dashboard`** — One-command view of all favorited locations with current temp, 24h snow, 5-day total, and operating status

  _Replaces opening the OpenSnow app — one command shows everything an agent needs about the user's preferred mountains_

  ```bash
  opensnow-pp-cli dashboard --agent
  ```
- **`diff`** — Compare current snow report against the last-synced version to see what changed: new snow, lifts opened or closed, status changes

  _Agents monitoring resort conditions can detect meaningful changes without parsing raw reports_

  ```bash
  opensnow-pp-cli diff alta --agent
  ```
- **`history`** — Show snowfall trends, season totals vs averages, and base depth progression from cached report snapshots

  _When planning a trip weeks out, historical context shows whether the mountain is trending up or down_

  ```bash
  opensnow-pp-cli history steamboat --days 30 --agent
  ```

### Agent-native plumbing
- **`digest`** — Pull all Daily Snow posts for favorited regions, strip HTML to clean text, and show a summary digest

  _Expert forecasts from OpenSnow meteorologists, rendered for agents and terminals instead of requiring a browser_

  ```bash
  opensnow-pp-cli digest --region colorado --agent
  ```

## Commands

### Forecasts

| Command | Description |
| --- | --- |
| `forecast get-by-point` | Forecast for any lng,lat coordinate and elevation (access level 5) |
| `forecast get-detail` | 5-day forecast for a named location with current, hourly, and daily periods (access level 10) |
| `forecast get-snow-detail` | 5-day day + night snowfall forecast split into 6am-6pm and 6pm-6am segments (access level 10) |

### Snow Reports

| Command | Description |
| --- | --- |
| `snow-report` | Latest resort-reported snowfall, base depth, operating status, and conditions (access level 10) |

### Daily Snow

| Command | Description |
| --- | --- |
| `daily-reads content get-daily-snow` | Most recent Daily Snow post by an OpenSnow meteorologist for a region |

### Powder Intelligence

| Command | Description |
| --- | --- |
| `powder-score` | Rate upcoming days 1-10 for powder quality at a resort |
| `powder-rank` | Rank multiple resorts by best powder day potential |
| `storm-track` | Track storm progression: start, peak, and end times |
| `overnight` | Overnight snow forecast for all favorited resorts |

### Dashboard and Comparison

| Command | Description |
| --- | --- |
| `dashboard` | Summary view of all favorites: temp, snow, base depth, status |
| `compare` | Side-by-side metrics for multiple resorts |
| `diff` | What changed since last sync: new snow, lift status |
| `history` | Snowfall trends and base depth from cached snapshots |
| `digest` | Daily Snow posts for favorite regions as clean text |

### Utilities

| Command | Description |
| --- | --- |
| `doctor` | Check API key validity and connectivity |
| `auth set-token` | Save API key to config |
| `auth status` | Show current authentication status |
| `sync` | Sync resort data to local SQLite for offline use |
| `favorites add` | Add resort slugs to favorites list |
| `favorites list` | List all favorited resorts |
| `favorites remove` | Remove a resort from favorites |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
opensnow-pp-cli snow-report alta

# JSON for scripting and agents
opensnow-pp-cli snow-report alta --json

# Filter to specific fields
opensnow-pp-cli snow-report alta --json --select status_display,precip_snow_24h,base_depth

# CSV for spreadsheets
opensnow-pp-cli snow-report alta --csv

# Dry run — show the request without sending
opensnow-pp-cli snow-report alta --dry-run

# Agent mode — JSON + compact + no prompts in one flag
opensnow-pp-cli snow-report alta --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-opensnow -g
```

Then invoke `/pp-opensnow <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/opensnow/cmd/opensnow-pp-mcp@latest
```

Then register it:

```bash
claude mcp add opensnow opensnow-pp-mcp -e OPENSNOW_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/opensnow-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPENSNOW_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/opensnow/cmd/opensnow-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "opensnow": {
      "command": "opensnow-pp-mcp",
      "env": {
        "OPENSNOW_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Cookbook

```bash
# Morning powder check — what fell overnight at all favorites
opensnow-pp-cli overnight

# Best powder day this week at Alta
opensnow-pp-cli powder-score alta --days 5

# Rank Colorado resorts for this weekend
opensnow-pp-cli powder-rank --slugs vail,aspen,telluride,steamboat

# Is a storm coming to Telluride?
opensnow-pp-cli storm-track telluride

# Side-by-side comparison of Vail vs Aspen vs Breckenridge
opensnow-pp-cli compare vail aspen breckenridge

# What changed at Alta since last check?
opensnow-pp-cli diff alta

# Read the Colorado Daily Snow expert forecast
opensnow-pp-cli daily-reads content get-daily-snow colorado

# Get a forecast for an exact GPS coordinate
opensnow-pp-cli forecast get-by-point -111.5838,40.5884 --elev 8530

# Show day + night snowfall detail for Steamboat
opensnow-pp-cli forecast get-snow-detail steamboat

# Sync data locally for offline use
opensnow-pp-cli sync

# Snowfall trends over the last month
opensnow-pp-cli history steamboat --days 30

# Pipeline: get Alta forecast as JSON, extract snow totals
opensnow-pp-cli forecast get-detail alta --json --select precip_snow

# Agent workflow: dashboard for all favorites as compact JSON
opensnow-pp-cli dashboard --agent
```

## Health Check

```bash
opensnow-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the OpenSnow API. Reports API key status, access level, and reachability.

## Configuration

Config file: `~/.config/opensnow-pp-cli/config.toml`

Environment variables:

| Name | Required | Description |
| --- | --- | --- |
| `OPENSNOW_API_KEY` | Yes | OpenSnow API key (partnership access) |
| `OPENSNOW_BASE_URL` | No | Override API base URL (default: `https://api.opensnow.com`) |
| `OPENSNOW_CONFIG` | No | Override config file path |

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `opensnow-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPENSNOW_API_KEY`
- Set your key: `opensnow-pp-cli auth set-token <key>`

**Not found errors (exit code 3)**
- Check the resort slug is correct (case-sensitive)
- Try common slugs: alta, steamboat, vail, aspen, telluride, aspenhighlands

### API-specific

- **401 Unauthorized on every request** — Run `opensnow-pp-cli auth set-token <key>` — the API key must be set before any request
- **403 Forbidden on /forecast/detail** — Your API key has access level 5 — named-location endpoints require level 10. Contact hello@opensnow.com to upgrade.
- **Empty snow report (all nulls)** — Resort may be closed for the season. Check status_display field — 'Closed' or 'Summer Operations' means no active snow data.
- **No results for a location slug** — Slugs are case-sensitive and specific. Try common names: alta, steamboat, aspenhighlands. Email hello@opensnow.com for the full slug list.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**jwise/snow**](https://github.com/jwise/snow) — Python (15 stars)
- [**meccaLeccaHi/snow_scraper**](https://github.com/meccaLeccaHi/snow_scraper) — Python (8 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
