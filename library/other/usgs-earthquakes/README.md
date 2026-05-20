# USGS Earthquakes CLI

**Every USGS earthquake feed and event query in one terminal, with offline SQLite cache, agent-native output, and a live watch mode.**

Wraps the full USGS FDSN Event Service and all 20 GeoJSON summary feeds in a single binary. Adds a 30-day rolling local SQLite cache so search, sql, aftershocks, swarm-detect, top, and changes run instantly without the network. Built for seismologists, journalists, emergency managers, and agents that need to reason about earthquake activity programmatically.

Learn more at [USGS Earthquakes](https://earthquake.usgs.gov).

Printed by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `usgs-earthquakes-pp-cli` binary and the `pp-usgs-earthquakes` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install usgs-earthquakes
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install usgs-earthquakes --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install usgs-earthquakes --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install usgs-earthquakes --agent claude-code
npx -y @mvanhorn/printing-press install usgs-earthquakes --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/cmd/usgs-earthquakes-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/usgs-earthquakes-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-usgs-earthquakes --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-usgs-earthquakes --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-usgs-earthquakes skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-usgs-earthquakes. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/usgs-earthquakes-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/cmd/usgs-earthquakes-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "usgs-earthquakes": {
      "command": "usgs-earthquakes-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required. USGS earthquake services are fully public; doctor verifies reachability of both the FDSN Event base URL and the summary feeds.

## Quick Start

```bash
# List M4.5+ earthquakes in the last 24h. Default for newsroom triage.
usgs-earthquakes-pp-cli recent --min-magnitude 4.5 --json


# Pull the pre-built 'past week, significant' summary feed once.
usgs-earthquakes-pp-cli feeds get significant_week --json


# Populate the local SQLite cache (30 days, M2.5+) so search/sql/aftershocks/top/changes work offline.
usgs-earthquakes-pp-cli sync


# Rank recent events by composite editorial score (sig × alert × felt × tsunami).
usgs-earthquakes-pp-cli top --window 24h --limit 10 --json


# Get a one-event briefing with PAGER, DYFI, tsunami, ShakeMap MMI, and product inventory.
usgs-earthquakes-pp-cli brief us7000abcd --format markdown


# Investigate a mainshock's aftershock sequence from the local store.
usgs-earthquakes-pp-cli aftershocks us7000abcd --radius-km 100 --days 30 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Real-time monitoring
- **`watch`** — Long-running poll of a USGS summary feed with deduplication against the local store. Invokes an optional shell hook per new event for custom alerting.

  _Reach for this when you want a long-running monitor over USGS earthquakes without writing a polling loop. Pluggable shell hook means it composes with notify-send, Slack curl, paging systems, anything._

  ```bash
  usgs-earthquakes-pp-cli watch --min-magnitude 5 --notify "echo {id}: {place}" --interval 60s
  ```

### Cross-event analysis
- **`aftershocks`** — Show events within R km and T days after a mainshock, ordered by time. Local SQLite haversine query with FDSN fallback when uncached.

  _Reach for this any time you have a mainshock event ID and need to see what came after. The single-call alternative is dozens of /query invocations stitched in jq._

  ```bash
  usgs-earthquakes-pp-cli aftershocks us7000abcd --radius-km 100 --days 30 --min-mag 3.0 --json
  ```
- **`swarm-detect`** — Detect time-space clusters in the local earthquake store. Grid-bucket clustering finds foreshock/aftershock sequences, volcanic swarms, and induced seismicity hotspots.

  _Reach for this when monitoring a volcano, fault zone, or fracking region for unusual clustering. The output names each cluster with center, count, peak magnitude, and time range._

  ```bash
  usgs-earthquakes-pp-cli swarm-detect --bbox -122.5,46.5,-121.5,47.5 --window 7d --min-events 10 --cluster-radius-km 20 --json
  ```
- **`compare`** — Side-by-side comparison of two regions or two time periods. Returns parallel columns with counts, max magnitude, and total seismic energy.

  _Reach for this when answering 'is region A more active than region B?' or 'is this region quieter than it used to be?'. Outputs delta percentages and energy ratios._

  ```bash
  usgs-earthquakes-pp-cli compare --region-a -122.5,37.5,-122.0,37.9 --region-b -118.5,33.8,-118.0,34.2 --window 30d --json
  ```

### Agent-native output
- **`brief`** — Agent-ready briefing for a single earthquake: magnitude, place, PAGER alert, DYFI felt count, ShakeMap MMI, tsunami status, and product inventory.

  _Reach for this in newsroom or Slack contexts where you need a one-event summary an editor or downstream agent can drop straight into copy. Includes the USGS event-page URL._

  ```bash
  usgs-earthquakes-pp-cli brief us7000abcd --format markdown
  ```
- **`top`** — Rank recent events by composite editorial score: significance × alert weight × felt count × tsunami flag. Default window 24h, limit 10.

  _Reach for this when you need 'the events that matter right now' rather than 'all events by magnitude'. The composite score promotes felt + PAGER + tsunami over raw magnitude._

  ```bash
  usgs-earthquakes-pp-cli top --window 24h --limit 10 --score composite --json
  ```

### Local state that compounds
- **`changes`** — Diff since the last sync: what events appeared, what events had magnitudes/depths/alerts revised, what events were retracted. Tracks USGS solution revisions over time. Returns 'no revisions recorded yet' until at least two sync runs have completed.

  _Reach for this when a previously-automatic event might have just been reviewed by an analyst (revising magnitude or alert), or to answer 'what's new since I last looked?'_

  ```bash
  usgs-earthquakes-pp-cli changes --since 24h --type revised --min-mag-delta 0.3 --json
  ```
- **`decode-id`** — Parse a USGS event ID into its source network code, sequence, and operator name. Joins the cached contributors dictionary for the network display name.

  _Reach for this when you see an opaque USGS event ID (e.g. nc73947885, ak0202xyz) and need to know which network reported it before reading the data._

  ```bash
  usgs-earthquakes-pp-cli decode-id us7000abcd
  ```

## Usage

Run `usgs-earthquakes-pp-cli --help` for the full command reference and flag list.

## Commands

### catalogs

USGS earthquake source catalogs (ANSS contributors and processing centers)

- **`usgs-earthquakes-pp-cli catalogs`** - List all source catalogs known to the FDSN Event service (XML response)

### contributors

USGS earthquake data contributors

- **`usgs-earthquakes-pp-cli contributors`** - List all data contributors known to the FDSN Event service (XML response)

### events

Search and retrieve earthquakes from the USGS FDSN Event service

- **`usgs-earthquakes-pp-cli events count`** - Count events matching the given filter, without returning event data (fast precheck before a full search)
- **`usgs-earthquakes-pp-cli events get`** - Fetch a single earthquake event by USGS event ID (e.g. us7000abcd)
- **`usgs-earthquakes-pp-cli events search`** - Search the USGS earthquake catalog with full FDSN parameter coverage

### feeds

Pre-built GeoJSON earthquake summary feeds (updated every minute by USGS)

- **`usgs-earthquakes-pp-cli feeds <feed>`** - Fetch a named GeoJSON summary feed. Use one of: significant_{hour|day|week|month}, 4.5_{hour|day|week|month}, 2.5_{hour|day|week|month}, 1.0_{hour|day|week|month}, all_{hour|day|week|month}.

### metadata

FDSN Event service metadata: enumerated values for every parameter

- **`usgs-earthquakes-pp-cli metadata`** - Show enum dictionaries for catalogs, contributors, event types, product types, and magnitude types


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
usgs-earthquakes-pp-cli catalogs

# JSON for scripting and agents
usgs-earthquakes-pp-cli catalogs --json

# Filter to specific fields
usgs-earthquakes-pp-cli catalogs --json --select id,name,status

# Dry run — show the request without sending
usgs-earthquakes-pp-cli catalogs --dry-run

# Agent mode — JSON + compact + no prompts in one flag
usgs-earthquakes-pp-cli catalogs --agent
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

## Health Check

```bash
usgs-earthquakes-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/usgs-earthquakes-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Search or sql returns empty rows on a fresh install** — Run `usgs-earthquakes-pp-cli sync` first — the local SQLite store is empty until sync populates it.
- **FDSN /query returns 400 'limit must be ≤ 20000'** — Lower --limit below 20000 or paginate with --offset; the FDSN cap is a hard ceiling.
- **Repeated polling slows down or returns stale data** — Use `feeds get <feed>` (e.g. `feeds get all_hour`) for high-cadence monitoring — USGS recommends summary feeds over /query for repeated polls.
- **watch keeps exec'ing the notify hook during printing-press verify** — Already handled: watch checks PRINTING_PRESS_VERIFY=1 and skips the hook in mock-mode subprocesses.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**blake365/usgs-quakes-mcp**](https://github.com/blake365/usgs-quakes-mcp) — TypeScript
- [**DOI-USGS/dataretrieval-python**](https://github.com/DOI-USGS/dataretrieval-python) — Python
- [**obspy/obspy**](https://github.com/obspy/obspy) — Python
- [**usgs-earthquake-api (doojin)**](https://github.com/doojin/usgs-earthquake-api) — JavaScript
- [**exxamalte/python-aio-geojson-usgs-earthquakes**](https://github.com/exxamalte/python-aio-geojson-usgs-earthquakes) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
