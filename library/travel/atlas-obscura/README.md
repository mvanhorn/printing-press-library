# Atlas Obscura CLI

**Every Atlas Obscura search, plus a local database, road-trip corridor routing, and saved trips no other Atlas Obscura tool has.**

Search the world's hidden wonders by keyword or coordinates, pull full place detail with Know-Before-You-Go notes, and mirror it all into a local SQLite store for offline, agent-native use. Then go further than any scraper: find wonders along a driving route, save and export trips, and track what you've visited. Community-sourced from atlasobscura.com — not an official API.

Learn more at [Atlas Obscura](https://www.atlasobscura.com).

Created by [@dbryson](https://github.com/dbryson) (David Bryson).

## Install

The recommended path installs both the `atlas-obscura-pp-cli` binary and the `pp-atlas-obscura` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install atlas-obscura
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install atlas-obscura --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install atlas-obscura --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install atlas-obscura --agent claude-code
npx -y @mvanhorn/printing-press-library install atlas-obscura --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/cmd/atlas-obscura-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/atlas-obscura-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install atlas-obscura --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-atlas-obscura --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-atlas-obscura --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install atlas-obscura --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/atlas-obscura-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/cmd/atlas-obscura-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "atlas-obscura": {
      "command": "atlas-obscura-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# health check; works with no setup since Atlas Obscura needs no API key
atlas-obscura-pp-cli doctor --dry-run

# keyword search across all wonders
atlas-obscura-pp-cli search "catacombs" --json

# wonders within 5 miles of a place, sorted by distance
atlas-obscura-pp-cli near "Paris" --radius 5 --json

# full detail incl. Know Before You Go
atlas-obscura-pp-cli show gustave-eiffels-secret-apartment

# wonders along the drive
atlas-obscura-pp-cli route "San Francisco" "Los Angeles" --limit 10 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Trip planning
- **`route`** — Find Atlas Obscura wonders along the driving corridor between two cities, not just in one place.

  _Reach for this when a user is driving between two places and wants worthwhile stops along the way._

  ```bash
  atlas-obscura-pp-cli route "San Francisco" "Los Angeles" --min-score 6 --limit 15 --json
  ```
- **`cluster`** — Group nearby wonders into spatially tight clusters that make a walkable half-day.

  _Use to turn a pile of nearby wonders into an efficient day on foot._

  ```bash
  atlas-obscura-pp-cli cluster "Edinburgh" --radius 3 --min 3 --json
  ```
- **`export`** — Serialize a saved trip to GPX, GeoJSON, or a markdown itinerary, fully offline.

  _Use to hand a road trip to a GPS, a map tool, or a human-readable log._

  ```bash
  atlas-obscura-pp-cli export california-oddities --format md
  ```

### Local state that compounds
- **`trip`** — Accumulate places into named itineraries that persist across sessions.

  _Use to build up a trip over multiple sessions instead of re-searching each time._

  ```bash
  atlas-obscura-pp-cli trip add winchester-mystery-house --trip california-oddities
  ```
- **`visited`** — Record which wonders you've seen, with optional date and note.

  _Use to remember what you've already seen so gaps and surprise can skip them._

  ```bash
  atlas-obscura-pp-cli visited mark salvation-mountain --note "worth the desert drive"
  ```
- **`gaps`** — Show good wonders near a point that you haven't visited yet, ranked by interestingness.

  _Use to plan what's left to see near a place you're revisiting._

  ```bash
  atlas-obscura-pp-cli gaps "Portland, Oregon" --radius 40 --min-score 6 --json
  ```
- **`surprise`** — Pick one high-interest wonder you haven't visited, seeded by date so it's stable per day.

  _Use in a daily agent heartbeat to surface a fresh wonder without repeats._

  ```bash
  atlas-obscura-pp-cli surprise --near "Tokyo" --exclude-visited --json
  ```

## Recipes


### Plan stops along a road trip

```bash
atlas-obscura-pp-cli route "Denver" "Moab" --min-score 6 --limit 12 --json
```

Surfaces the best-scoring wonders within the driving corridor between two cities.

### Lean nearby scan for an agent

```bash
atlas-obscura-pp-cli near "35.0116,135.7681" --radius 3 --json --select results.title,results.distance_from_query,results.url
```

Deeply nested response narrowed to just title, distance, and URL so an agent doesn't parse image and coordinate noise.

### Build and export a trip

```bash
atlas-obscura-pp-cli trip add winchester-mystery-house --trip ca && atlas-obscura-pp-cli export ca --format md
```

Accumulate places into a named trip, then render a markdown itinerary with descriptions and Know Before You Go.

### What haven't I seen near home

```bash
atlas-obscura-pp-cli gaps "Austin, Texas" --radius 30 --min-score 6 --json
```

Cross-references cached wonders against your visited log and returns only the worthwhile unseen ones.

## Usage

Run `atlas-obscura-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

Browse places by Atlas Obscura category

- **`atlas-obscura-pp-cli categories <slug>`** - List place links in a category (e.g. cemeteries, caves, ruins)

### destinations

Browse places by destination (city/region)

- **`atlas-obscura-pp-cli destinations <slug>`** - List place links for a destination (e.g. paris-france, new-york)

### places

Atlas Obscura places (wonders)

- **`atlas-obscura-pp-cli places <slug>`** - Fetch a place detail page by slug or numeric id


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
atlas-obscura-pp-cli places mock-value

# JSON for scripting and agents
atlas-obscura-pp-cli places mock-value --json

# Filter to specific fields
atlas-obscura-pp-cli places mock-value --json --select id,name,status

# Dry run — show the request without sending
atlas-obscura-pp-cli places mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
atlas-obscura-pp-cli places mock-value --agent
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
atlas-obscura-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/atlas-obscura-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty results for a place name** — Place geocoding uses Open-Meteo; try a more specific name ("Portland, Oregon") or pass explicit lat,lng to near.
- **near --category returns few results** — Category is filtered client-side from place tags; raise --max-scan-pages or --radius to widen the scan.
- **Stale cached place** — Pass --refresh on show/near to bypass the local cache TTL and re-fetch from atlasobscura.com.
- **Rate-limited or 403 from atlasobscura.com** — The CLI keeps a low request rate by default; slow down with --limit and rely on the local cache for repeat queries.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**atlas-obscura-api**](https://github.com/bartholomej/atlas-obscura-api) — TypeScript (9 stars)
- [**node-atlas-obscura**](https://github.com/TruitMeGood/node-atlas-obscura) — JavaScript
- [**travel-hacking-toolkit (atlas-obscura skill)**](https://github.com/borski/travel-hacking-toolkit) — JavaScript
- [**obscura-scraper**](https://github.com/seeksort/obscura-scraper) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
