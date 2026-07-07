# National Hurricane Center CLI

**Credible, real-time National Hurricane Center data for AI agents: active storms, parsed advisories, and tropical outlooks, text-first with link-outs for everything else.**

This CLI gives agents the most credible real-time hurricane information straight from the National Hurricane Center and the National Weather Service: active storms (CurrentStorms.json), parsed Public Advisories and Forecast Discussions, the Tropical Weather Outlook with formation odds, and live tropical watches and warnings. It is built text-first and links out to the official GIS for anyone who wants polygons. Deep thanks to the forecasters, hurricane hunters, and support staff at NHC who sacrifice so much for the safety of so many. This is an unofficial tool; in an emergency, follow the official watches, warnings, and evacuation orders from NHC, the NWS, and your local authorities.

## Install from source

This repository builds with the Go toolchain (1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/nhc/cmd/nhc-pp-cli@latest
```

Or clone and build the CLI plus the bundled MCP server:

```bash
git clone https://github.com/mvanhorn/printing-press-library
cd nhc-pp-cli
make build       # binaries in ./bin
go test ./...    # parsers verified against real 2024-season fixtures in internal/cli/testdata
```

Then run `nhc-pp-cli doctor` (no API key needed) and `nhc-pp-cli brief --markdown` for a one-call situational briefing.

## Install via the Printing Press library

Once this CLI is published to the public library, one command installs both the `nhc-pp-cli` binary and the `pp-nhc` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nhc
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nhc --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nhc --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nhc --agent claude-code
npx -y @mvanhorn/printing-press-library install nhc --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nhc-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nhc --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nhc --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nhc --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nhc --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nhc-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nhc": {
      "command": "nhc-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI is wired up; no API key needed.
nhc-pp-cli doctor --dry-run

# What tropical cyclones are active right now (empty in the quiet season).
nhc-pp-cli storms list

# What may be developing, even when no storms are named.
nhc-pp-cli outlook --basin atl

# One-call situational briefing across storms, outlook, and alerts.
nhc-pp-cli brief --markdown

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### One-call situational awareness
- **`brief`** — One command returns active storms, the tropical outlook, and live NWS tropical alerts as a single payload, with an optional human briefing.

  _Reach for this first when an agent asks 'what is the tropical situation right now' — one call instead of four._

  ```bash
  nhc-pp-cli brief --basin atl --markdown
  ```

### Parsed NHC products
- **`storm`** — Full detail for one storm: vitals plus every advisory-product URL and GIS link-out, with not-applicable fields returned as explicit null.

  _Use when you have a storm id and need its products and graphics without parsing the raw feed._

  ```bash
  nhc-pp-cli storm al092024
  ```
- **`advisory`** — Fetches and parses the Public Advisory, Forecast Discussion, or Forecast/Marine Advisory into structured fields plus the clean body text, with no HTML scraping by the agent.

  _Use when you need the meteorologist's reasoning or exact wind/pressure/movement fields, not a web page._

  ```bash
  nhc-pp-cli advisory al092024 --type tcd
  ```
- **`outlook`** — Parses the Tropical Weather Outlook for any basin into development areas and 48-hour / 7-day formation chances, with graphic link-outs.

  _Use in the quiet season or to forecast-ahead; it is useful when 'storms list' is empty._

  ```bash
  nhc-pp-cli outlook --basin atl
  ```

### Link-outs done right
- **`graphics`** — Returns the cone, track, surge, and wind link-outs for a storm; --download saves the files locally and --open views them.

  _Use to hand a dashboard or person the official NHC graphics without re-deriving file paths._

  ```bash
  nhc-pp-cli graphics al092024 --kind cone,surge
  ```
- **`gis`** — Maps a storm to its ArcGIS REST layer URLs (forecast cone, wind field, watch/warning) for mapping clients. Link-out only; never ingested.

  _Use when a caller wants the spatial layers; this CLI references them instead of parsing them._

  ```bash
  nhc-pp-cli gis al092024
  ```

### Gratitude and safety
- **`credits`** — Thanks the people of the National Hurricane Center and states plainly that this is an unofficial tool and NHC/NWS is authoritative.

  _Read or surface this so users know the source of truth and who to thank._

  ```bash
  nhc-pp-cli credits
  ```

## Recipes


### Active storms as compact JSON for an agent

```bash
nhc-pp-cli storms list --json --select id,name,classification,intensity,pressure
```

Returns just the high-gravity fields so an agent does not burn context on the full feed.

### Read the forecast discussion

```bash
nhc-pp-cli advisory al092024 --type tcd
```

Fetches and parses the meteorologist's narrative for a storm id.

### One-call situational briefing

```bash
nhc-pp-cli brief --basin atl --markdown
```

Bundles active storms, the outlook, and tropical alerts into a single human-readable briefing.

### Official cone and surge graphics for a storm

```bash
nhc-pp-cli graphics al092024 --kind cone,surge
```

Hands back the official NHC link-outs without re-deriving file paths.

## Usage

Run `nhc-pp-cli --help` for the full command reference and flag list.

## Commands

### alerts

Active NWS tropical watches and warnings (api.weather.gov)

- **`nhc-pp-cli alerts`** - Active tropical alerts. Pass a comma-separated --event list (Hurricane/Tropical Storm/Storm Surge x Warning/Watch). A descriptive User-Agent is mandatory (sent automatically).

### storms

Active tropical cyclones from NHC CurrentStorms.json (all basins)

- **`nhc-pp-cli storms`** - List every active tropical cyclone across all basins. Returns [] when none are active (quiet season); run 'outlook' to see what may be developing.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nhc-pp-cli storms

# JSON for scripting and agents
nhc-pp-cli storms --json

# Filter to specific fields
nhc-pp-cli storms --json --select id,name,status

# Dry run — show the request without sending
nhc-pp-cli storms --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nhc-pp-cli storms --agent
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
nhc-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/nhc-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **api.weather.gov returns 403** — A descriptive User-Agent is mandatory; nhc-pp-cli sends one automatically, so re-run rather than calling the API by hand.
- **storms list is empty** — That is the verified quiet-season contract (zero active storms). Run 'nhc-pp-cli outlook' to see formation chances.
- **advisory says product not found** — Products only exist for active or recent storms; check 'nhc-pp-cli storms list' for valid ids first.
