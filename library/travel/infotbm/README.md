# TBM Bordeaux CLI

**Every Bordeaux tram, bus, and ferry schedule offline in SQLite, plus real-time arrivals, ghost service detection, and journey planning no other Bordeaux transit tool offers.**

Syncs the full TBM GTFS dataset locally so schedule lookups, stop searches, and frequency analysis work without an internet connection. Overlays live real-time data from the SIRI-Lite API for arrival predictions, vehicle tracking, and disruption alerts. Computes journeys, detects ghost services, and diffs timetable changes — capabilities no existing Bordeaux transit tool provides.

Created by [@pawlclawbot](https://github.com/pawlclawbot) (pawlclawbot).

## Install

The recommended path installs both the `infotbm-pp-cli` binary and the `pp-infotbm` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install infotbm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install infotbm --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install infotbm --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install infotbm --agent claude-code
npx -y @mvanhorn/printing-press-library install infotbm --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/infotbm/cmd/infotbm-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/infotbm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install infotbm --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-infotbm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-infotbm --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install infotbm --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/infotbm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `INFOTBM_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/infotbm/cmd/infotbm-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "infotbm": {
      "command": "infotbm-pp-mcp",
      "env": {
        "INFOTBM_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check — verifies API reachability and local store state
infotbm-pp-cli doctor --dry-run

# Download the full GTFS dataset into local SQLite
infotbm-pp-cli sync --full

# Find stops matching a name
infotbm-pp-cli stops --name Quinconces --json

# Real-time next arrivals at Quinconces
infotbm-pp-cli realtime stop --stop-id bordeaux:StopPoint:BP:3648:LOC --json

# Tram A headway patterns by hour
infotbm-pp-cli lines frequency --line A --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Schedule intelligence
- **`schedule diff`** — Surface trips that are scheduled in GTFS but absent from live real-time data — the buses that exist on paper but never showed up

  _When an agent needs to know whether a scheduled service is actually running today, this is the only command that answers definitively_

  ```bash
  infotbm-pp-cli schedule diff --stop BEQUI --line A --json
  ```
- **`lines stops`** — Print the ordered stop list for a line and direction, optionally with scheduled departure times

  _When an agent needs the exact stop order for a tram line to plan boarding points or give directions, this is the canonical answer_

  ```bash
  infotbm-pp-cli lines stops --line A --direction 0 --json
  ```
- **`schedule changes`** — Compare GTFS sync snapshots to detect added, removed, or modified routes on a line

  _When an agent manages a commuter's routine, this proactively detects schedule changes that would break their timing_

  ```bash
  infotbm-pp-cli schedule changes --line C --since 7d --json
  ```
- **`lines frequency`** — Compute average headways per hour from the SIRI estimated timetable

  _When an agent evaluates transit reliability for scheduling, this shows the real-world gaps between departures by time of day_

  ```bash
  infotbm-pp-cli lines frequency --line B --json
  ```

### Journey intelligence
- **`trips last-departure`** — Find the latest departure from a stop that still reaches your destination before a cutoff time

  _When an agent plans evening activities, this answers 'what is the absolute latest I can leave and still get home by transit'_

  ```bash
  infotbm-pp-cli trips last-departure --from Victoire --to Pessac --before 23:30 --json
  ```
- **`trips reroute`** — When your current connection is delayed, compute the best alternate onward path using live vehicle data

  _When an agent detects a delay notification, this computes the optimal recovery without re-planning from scratch_

  ```bash
  infotbm-pp-cli trips reroute --at "Gare Saint-Jean" --to Meriadeck --delay 8 --json
  ```
- **`trips plan`** — Plan multi-modal journeys across tram, bus, and ferry using local GTFS data with live disruption awareness

  _When an agent needs transit directions in Bordeaux without relying on third-party map APIs, this plans the route locally_

  ```bash
  infotbm-pp-cli trips plan --from Victoire --to "Gare Saint-Jean" --depart 08:15 --json
  ```

### Commute intelligence
- **`alerts impact`** — Filter all active disruptions to only those affecting your specific lines or stops

  _When an agent monitors a commuter's daily lines, this filters noise to only relevant disruptions_

  ```bash
  infotbm-pp-cli alerts impact --lines A,C,15 --json
  ```

## Recipes

### Morning commute check

```bash
infotbm-pp-cli realtime stop --stop-id bordeaux:StopPoint:BP:3648:LOC --json --select items.lineName,items.destinationName,items.expectedDepartureTime
```

Check next departures at your stop with only the fields that matter

### Weekend disruption audit

```bash
infotbm-pp-cli alerts impact --lines A,B,C --json
```

Filter alerts to only tram lines before a weekend outing

### Late night last train

```bash
infotbm-pp-cli trips last-departure --from Victoire --to Pessac --before 23:30 --json
```

Find the latest departure that still gets you home

### Detect ghost services

```bash
infotbm-pp-cli schedule diff --stop BEQUI --line A --agent
```

Surface scheduled trams that are not showing up in real-time data

### Weekly schedule change alert

```bash
infotbm-pp-cli schedule changes --line C --since 7d --json
```

Detect any trips added, removed, or shifted since last week's sync

## Usage

Run `infotbm-pp-cli --help` for the full command reference and flag list.

## Commands

### agencies

- **`infotbm-pp-cli agencies`** - Transit agencies in the Bordeaux network

### alerts

- **`infotbm-pp-cli alerts`** - Active service disruption alerts

### fares

- **`infotbm-pp-cli fares`** - Fare structure including pricing and transfer rules

### feed-info

- **`infotbm-pp-cli feed-info`** - GTFS feed metadata including validity dates and timestamps

### kml

- **`infotbm-pp-cli kml`** - KML geographic data export with route geometry and stop locations

### realtime

- **`infotbm-pp-cli realtime stop`** - Real-time departure information at a specific stop
- **`infotbm-pp-cli realtime vehicles`** - Real-time vehicle positions across the network

### routes

- **`infotbm-pp-cli routes`** - All transit routes/lines in the TBM network

### server-info

- **`infotbm-pp-cli server-info`** - API version and build information

### siri

- **`infotbm-pp-cli siri check-status`** - SIRI service health check
- **`infotbm-pp-cli siri estimated-timetable`** - Estimated real-time timetable for a line
- **`infotbm-pp-cli siri general-message`** - General service messages and disruption information
- **`infotbm-pp-cli siri lines-discovery`** - Discover all lines with destinations and transport modes
- **`infotbm-pp-cli siri stop-monitoring`** - Real-time arrival/departure monitoring at a stop
- **`infotbm-pp-cli siri stoppoints-discovery`** - Discover all stop points with coordinates and serving lines

### stops

- **`infotbm-pp-cli stops`** - All transit stops in the TBM network

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
infotbm-pp-cli agencies

# JSON for scripting and agents
infotbm-pp-cli agencies --json

# Filter to specific fields
infotbm-pp-cli agencies --json --select id,name,status

# Dry run — show the request without sending
infotbm-pp-cli agencies --dry-run

# Agent mode — JSON + compact + no prompts in one flag
infotbm-pp-cli agencies --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - most commands only read data; `import` and `sync` write to the local store but do not mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
infotbm-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/infotbm-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INFOTBM_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `infotbm-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `infotbm-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INFOTBM_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the relevant resource command (e.g., `stops --name <name>`, `routes`) to see available items

### API-specific
- **Empty arrivals response** — Service may have ended for the day. Check with `infotbm-pp-cli lines frequency --line <line>` to see operating hours.
- **Sync fails with timeout** — The GTFS ZIP is ~15MB. Retry with `infotbm-pp-cli sync --full --timeout 60s`.
- **Stop ID format confusion** — Use `stops --name <name>` to find the full stop reference (e.g., bordeaux:StopPoint:BP:3648:LOC). Short codes like BEQUI also work for some endpoints.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**JulienLavocat/InfoTBM-Client**](https://github.com/JulienLavocat/InfoTBM-Client) — JavaScript (5 stars)
- [**Almtesh/infotbm**](https://github.com/Almtesh/infotbm) — Python (4 stars)
- [**drawbu/nextbus**](https://github.com/drawbu/nextbus) — Go (3 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
