# Surfline CLI

**Every Surfline forecast, scriptable from your terminal — plus multi-spot ranking, a local forecast journal, and cron-able alerts no Surfline tool has.**

Pulls wave, swell, wind, tide, weather, conditions and rating straight from Surfline's API over a browser-fingerprint transport that clears Cloudflare. The differentiators live in the local SQLite store: 'rank' scans a set of spots and sorts them best-first (Surfline's own comparison is web-only), 'now' collapses one spot's next hours into a paddle/no-paddle readout, 'windows' finds the daylight blocks worth surfing, and 'alert run' evaluates your own threshold rules for cron.

## Install

The recommended path installs both the `surfline-pp-cli` binary and the `pp-surfline` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install surfline
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install surfline --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install surfline --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install surfline --agent claude-code
npx -y @mvanhorn/printing-press-library install surfline --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/surfline/cmd/surfline-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/surfline-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install surfline --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-surfline --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-surfline --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install surfline --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/surfline-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SURFLINE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/surfline/cmd/surfline-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "surfline": {
      "command": "surfline-pp-mcp",
      "env": {
        "SURFLINE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Basic forecasts, search, taxonomy and multi-spot data need no auth at all (up to a 6-day horizon). For 7–17 day forecasts and premium cams, set a Surfline access token: run 'surfline-pp-cli auth login' with your Surfline email and password (it uses the community password-grant flow against /trusted/token), or 'surfline-pp-cli auth set-token <token>' if you already have one. The token is stored locally and sent as the accesstoken query param; SURFLINE_ACCESS_TOKEN is also honored.

## Quick Start

```bash
# Confirm the binary, transport and (optional) token wiring before anything hits the network.
surfline-pp-cli doctor --dry-run

# Look up a spot by name online to get its spotId.
surfline-pp-cli spots find "Lower Trestles"

# Dawn-patrol readout for one spot's next hours.
surfline-pp-cli now 5842041f4e65fad6a7708807

# Rank a set of spots best-first to pick where to go.
surfline-pp-cli rank 5842041f4e65fad6a7708807 5842041f4e65fad6a7708cfd

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Decision-shaped readouts
- **`now`** — One spot's next few hours as a paddle/no-paddle line readout: swell, wind, tide and rating joined per hour.

  _Reach for this when an agent needs a go/no-go answer for one break without parsing five separate forecast payloads._

  ```bash
  surfline-pp-cli now 5842041f4e65fad6a7708807 --agent
  ```
- **`rank`** — Score and sort several spots best-first on a transparent sum of wave, wind and swell optimalScore.

  _Reach for this to pick today's best break from a set instead of opening N forecast pages._

  ```bash
  surfline-pp-cli rank 5842041f4e65fad6a7708807 5842041f4e65fad6a7708cfd --agent
  ```
- **`windows`** — Emit only the contiguous time blocks where wave, wind and swell optimalScore are all good, daylight-only.

  _Reach for this to find the good time slots at one spot without eyeballing an hourly graph._

  ```bash
  surfline-pp-cli windows 5842041f4e65fad6a7708807 --days 3 --agent
  ```

### Raw data for scripts
- **`raw`** — Pipe-friendly table/JSON of min/max/optimalScore/humanRelation plus swell components and wind directionType/gust, no rating editorializing.

  _Reach for this when an agent needs raw numbers to feed its own scoring instead of a pre-judged rating._

  ```bash
  surfline-pp-cli raw 5842041f4e65fad6a7708807 --agent --select data.wave.surf.max,data.wave.swells.period
  ```
- **`buoy-check`** — Show nearby-buoy observed swell against the spot's wave forecast for the same window, side by side.

  _Reach for this to tell whether the forecast is tracking the actual buoy readings before trusting it._

  ```bash
  surfline-pp-cli buoy-check 5842041f4e65fad6a7708807 --agent
  ```

### Local state that compounds
- **`alert run`** — Store swell/wind/tide threshold rules locally; alert run fetches a fresh forecast, evaluates them, prints matches and sets an exit code for cron.

  _Reach for this in unattended/cron contexts to get a nonzero exit when conditions a user defined are met._

  ```bash
  surfline-pp-cli alert run --agent
  ```
- **`journal show`** — Snapshot the current forecast into local SQLite and review a spot's snapshots over time.

  _Reach for this to review how a spot's forecast has read over the past days from your own captures._

  ```bash
  surfline-pp-cli journal show 5842041f4e65fad6a7708807 --agent
  ```
- **`search`** — Resolve spot names to spotIds via FTS over the locally-synced taxonomy, with no network.

  _Reach for this to turn a spot name into an ID before calling forecast commands, even offline._

  ```bash
  surfline-pp-cli search "Ocean Beach" --agent
  ```

## Recipes


### Pick today's break from your favorites

```bash
surfline-pp-cli rank 5842041f4e65fad6a7708807 5842041f4e65fad6a7708cfd 5842041f4e65fad6a7708e3d --agent
```

Batch-fetches all spots and sorts them best-first on combined optimalScore.

### Narrow a deep wave payload to the fields you need

```bash
surfline-pp-cli raw 5842041f4e65fad6a7708807 --agent --select data.wave.surf.max,data.wave.swells.period,data.wave.swells.direction
```

The wave response is deeply nested; dotted --select pulls just surf height and the swell period/direction so agents don't parse tens of KB.

### Find when it's actually good this week

```bash
surfline-pp-cli windows 5842041f4e65fad6a7708807 --days 5
```

Emits only the daylight blocks where wave, wind and swell optimalScore all clear the bar.

### Cron a surf alert

```bash
surfline-pp-cli alert run --agent
```

Evaluates your stored threshold rules against a fresh forecast and exits nonzero when one matches, so cron/CI can act on it.

### Sanity-check the forecast against buoys

```bash
surfline-pp-cli buoy-check 5842041f4e65fad6a7708807 --agent
```

Puts observed nearby-buoy swell next to the spot's forecast swell for the same window.

## Usage

Run `surfline-pp-cli --help` for the full command reference and flag list.

## Commands

### buoys

Nearby NDBC-style buoy observations

- **`surfline-pp-cli buoys`** - Buoys near a lat/lon with observed swell readings

### regions

Region/subregion-level forecasts

- **`surfline-pp-cli regions`** - Subregion conditions forecast (multi-day AM/PM rating across a subregion)

### spots

Find spots and pull per-spot forecasts and reports (wave, wind, tide, weather, conditions, rating)

- **`surfline-pp-cli spots batch`** - Rich per-spot info for many spots in one call (conditions, cameras, current swell/wind/tide)
- **`surfline-pp-cli spots conditions`** - AM/PM conditions: rating, observation, surf range, forecaster notes, humanRelation
- **`surfline-pp-cli spots details`** - Spot metadata: name, location, ability levels, travel info
- **`surfline-pp-cli spots find`** - Live spot search by name (online); returns hits with spotIds. For offline name lookup after a sync, use the top-level `search` command instead.
- **`surfline-pp-cli spots forecast`** - Combined forecast: forecasts + tides + sunrise/sunset in one call
- **`surfline-pp-cli spots rating`** - Rating forecast: VERY_POOR..EPIC key plus numeric value per time point
- **`surfline-pp-cli spots report`** - Spot report: forecaster narrative, current conditions, cameras
- **`surfline-pp-cli spots sunlight`** - Sunlight forecast: dawn, sunrise, sunset, dusk local times
- **`surfline-pp-cli spots tides`** - Tide forecast: HIGH/LOW/NORMAL extremes with heights and local times
- **`surfline-pp-cli spots wave`** - Wave and swell forecast: surf min/max, optimalScore, swell components (height/period/direction)
- **`surfline-pp-cli spots weather`** - Weather forecast: temperature, condition, pressure, plus sunlight times
- **`surfline-pp-cli spots wind`** - Wind forecast: speed, direction, directionType (Onshore/Offshore/Cross-shore), gust, optimalScore

### taxonomy

Browse Surfline's geographic hierarchy (geoname > region > subregion > spot)

- **`surfline-pp-cli taxonomy <id>`** - Fetch a taxonomy node and its children (ancestors via `in`, children via `contains`)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
surfline-pp-cli taxonomy mock-value

# JSON for scripting and agents
surfline-pp-cli taxonomy mock-value --json

# Filter to specific fields
surfline-pp-cli taxonomy mock-value --json --select id,name,status

# Dry run — show the request without sending
surfline-pp-cli taxonomy mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
surfline-pp-cli taxonomy mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
surfline-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/surfline/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SURFLINE_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `surfline-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `surfline-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SURFLINE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return HTTP 403 / Cloudflare HTML** — Update to a build that ships the Surf (Chrome TLS) transport; run 'surfline-pp-cli doctor' to confirm the transport is browser-chrome.
- **Forecast cut off at 6 days** — Set a token: 'surfline-pp-cli auth login' or 'surfline-pp-cli auth set-token <token>'. 7–17 day horizons are premium-gated.
- **search returns nothing offline** — Offline 'search' indexes spots you've captured with 'journal log'. Run 'surfline-pp-cli journal log <spotId>' first, or use the online 'surfline-pp-cli spots find <name>'.
- **Don't know a spot's ID** — Use 'surfline-pp-cli search "<name>"' to get the 24-char spotId, then pass it to forecast commands.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**swrobel/meta-surf-forecast**](https://github.com/swrobel/meta-surf-forecast) — Ruby (336 stars)
- [**mhelmetag/surflinef**](https://github.com/mhelmetag/surflinef) — Go (83 stars)
- [**TGOlson/surfline**](https://github.com/TGOlson/surfline) — TypeScript (8 stars)
- [**englishar/surfline-mcp-server**](https://github.com/englishar/surfline-mcp-server) — TypeScript (7 stars)
- [**Mircobrb/pysurfline**](https://github.com/Mircobrb/pysurfline) — Python (4 stars)
- [**kylerberry/surfline-nodejs**](https://github.com/kylerberry/surfline-nodejs) — JavaScript (1 stars)
- [**mdecourcy/go-surfline-api**](https://github.com/mdecourcy/go-surfline-api) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
