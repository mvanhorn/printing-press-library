# Roadside America CLI

**Every offbeat roadside attraction on RoadsideAmerica.com — with a local SQLite cache, offline search, and superlative categories no other tool offers.**

RoadsideAmerica.com is the web's best catalog of quirky US & Canada tourist attractions, but it has no API and a paywalled app. This CLI turns it into an agent-native, pipe-friendly tool: find what's near a place or coordinates, browse a whole state, pull the full writeup, and slice by superlative categories like biggest/smallest/tallest/weird-food. Everything is cached locally (fresh-on-read), every record links back to its source, and the scraper stays a polite, attributing, user-initiated citizen of the site.

## Install

The recommended path installs both the `roadside-america-pp-cli` binary and the `pp-roadside-america` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install roadside-america
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install roadside-america --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install roadside-america --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install roadside-america --agent claude-code
npx -y @mvanhorn/printing-press-library install roadside-america --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/roadside-america/cmd/roadside-america-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/roadside-america-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install roadside-america --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-roadside-america --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-roadside-america --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install roadside-america --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/roadside-america-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/roadside-america/cmd/roadside-america-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "roadside-america": {
      "command": "roadside-america-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# health + reachability check; no auth needed
roadside-america-pp-cli doctor

# quirky attractions near a city (geocoded), within 20 miles
roadside-america-pp-cli near "Austin, TX" --radius 20

# browse offbeat attractions in a state
roadside-america-pp-cli state TX --limit 10

# full writeup + location + source link for one attraction
roadside-america-pp-cli show 2055

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local cache intelligence
- **`category`** — Find the biggest, smallest, tallest, or weird-food attractions (and more) by classifying cached attractions locally.

  _Reach for this when the user wants superlatives or a themed slice (giants, muffler-men, weird-food) rather than a place- or state-scoped list._

  ```bash
  roadside-america-pp-cli category biggest --json
  ```
- **`stats`** — Summarize the local cache: counts by state and by category, plus totals.

  _Use to understand coverage of the local cache before planning, or to answer 'which state has the most offbeat stuff cached'._

  ```bash
  roadside-america-pp-cli stats --agent
  ```
- **`random`** — Pick a random offbeat attraction, optionally constrained by state or category.

  _Use for serendipity or road-trip inspiration when the user has no specific target._

  ```bash
  roadside-america-pp-cli random --state TX
  ```

### Route & comparison
- **`trip`** — Collect quirky stops near a list of cities or coordinates in one call, deduped and labeled by stop.

  _Reach for this when planning a route and the user wants offbeat stops across several waypoints at once._

  ```bash
  roadside-america-pp-cli trip "Austin, TX" "Waco, TX" --radius 15 --json
  ```
- **`compare`** — Compare two states by offbeat-attraction count and surface a few top picks from each.

  _Use when the user is deciding between regions or wants a quick 'which state is weirder' answer._

  ```bash
  roadside-america-pp-cli compare TX CA
  ```

## Recipes


### Quirky stops near coordinates, agent-friendly fields

```bash
roadside-america-pp-cli near 30.27,-97.74 --radius 25 --agent --select name,city,distance,source_url
```

Pass raw lat,lng to skip geocoding and select only the fields an agent needs.

### Weird food across a cached state

```bash
roadside-america-pp-cli category weird-food --json
```

Classifies cached attractions by food keywords; populate the cache with state/near first.

### Plan offbeat stops across a route

```bash
roadside-america-pp-cli trip "Austin, TX" "Waco, TX" "Dallas, TX" --radius 15 --json
```

Aggregates nearby attractions for each waypoint, deduped and labeled by stop.

### Full writeup with source attribution

```bash
roadside-america-pp-cli show 2055 --json
```

Returns structured name/address/writeup plus the RoadsideAmerica.com source URL.

## Usage

Run `roadside-america-pp-cli --help` for the full command reference and flag list.

## Commands

### raw

Raw RoadsideAmerica.com passthrough (HTML link/page extraction). Prefer the top-level near / state / show / category commands for structured output.

- **`roadside-america-pp-cli raw by-state`** - Raw attraction links for a US/Canada state (HTML fragment).
- **`roadside-america-pp-cli raw detail`** - Raw attraction detail page (HTML).
- **`roadside-america-pp-cli raw nearby`** - Raw nearby attraction links for coordinates (HTML fragment).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
roadside-america-pp-cli raw by-state --state example-value

# JSON for scripting and agents
roadside-america-pp-cli raw by-state --state example-value --json

# Filter to specific fields
roadside-america-pp-cli raw by-state --state example-value --json --select id,name,status

# Dry run — show the request without sending
roadside-america-pp-cli raw by-state --state example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
roadside-america-pp-cli raw by-state --state example-value --agent
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
roadside-america-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/roadside-america-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Slow responses or HTTP 429** — The CLI self-limits to ~1 request/3s and caches in SQLite; let it back off, or rely on cached data (results show fetched_at).
- **near "<place>" returns nothing** — Widen --radius, or pass coordinates directly: near 30.27,-97.74 --radius 25.
- **Geocoding a place name fails** — Pass latitude,longitude directly to near/trip; geocoding uses keyless OSM Nominatim which may rate-limit.
- **category returns few results** — Categories classify the LOCAL cache; run state/sync/near first to populate, then re-run category.
