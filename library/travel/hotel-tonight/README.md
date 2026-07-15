# HotelTonight CLI

**Last-minute hotel deals with a local price-history database no HotelTonight client has — snapshot deals over time, watch a city for drops, and rank by real % off.**

HotelTonight's deals are deliberately ephemeral and geo-local: they appear, drop, and vanish, and you only ever see now, here. This CLI syncs the anonymous deal feed into a local SQLite store and snapshots prices over time, so you (or an agent) can watch a city for drops, see a hotel's real price history, and get an objective cheap/typical/expensive verdict — none of which the app supports.

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `hotel-tonight-pp-cli` binary and the `pp-hotel-tonight` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hotel-tonight
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hotel-tonight --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hotel-tonight --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hotel-tonight --agent claude-code
npx -y @mvanhorn/printing-press-library install hotel-tonight --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/cmd/hotel-tonight-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hotel-tonight-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install hotel-tonight --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hotel-tonight --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hotel-tonight --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install hotel-tonight --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hotel-tonight-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/cmd/hotel-tonight-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hotel-tonight": {
      "command": "hotel-tonight-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or API key. HotelTonight's consumer endpoints are reachable anonymously; the CLI sends the app-identifying headers it needs automatically. Booking still happens in the app — this tool is read-only.

## Quick Start

```bash
# See HotelTonight's major markets and their ids (1 = San Francisco, 72 = Austin).
hotel-tonight-pp-cli markets list

# Pull tonight's deals near a location ranked by % off; this also records a price snapshot so history and watch have data to work with.
hotel-tonight-pp-cli deals --lat 37.7749 --lng -122.4194 --sort discount

# Flag rooms under $150 or that dropped since the last snapshot.
hotel-tonight-pp-cli watch --lat 37.7749 --lng -122.4194 --when tonight --below 150

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local price intelligence

- **`watch`** — Snapshot a location's deals now and get told which rooms dropped below your threshold or fell since you last looked.

  _Reach for this when an agent needs to monitor a city for last-minute hotel price drops over time instead of re-querying blind._

  ```bash
  hotel-tonight-pp-cli watch --lat 37.7749 --lng -122.4194 --when tonight --below 150 --agent
  ```
- **`history`** — Show the recorded price and % off history for one hotel from the local store.

  _Use when you need the actual price trajectory of a hotel rather than a single ephemeral quote._

  ```bash
  hotel-tonight-pp-cli history "Argonaut Hotel" --days 30 --agent
  ```
- **`verdict`** — Classify a hotel's current quoted price against its own observed low/median/high as cheap, typical, or expensive.

  _Reach for this to answer 'is this actually cheap?' with a grounded baseline instead of a guess._

  ```bash
  hotel-tonight-pp-cli verdict "Argonaut Hotel" --agent
  ```

### Live deal views

- **`compare-neighborhoods`** — Group tonight's deals in a metro by neighborhood and rank the neighborhoods by median price or best % off.

  _Use when deciding which area of a city has the best last-minute value tonight._

  ```bash
  hotel-tonight-pp-cli compare-neighborhoods --metro 1 --when tonight --agent
  ```
- **`datescan`** — Compare a location's deals across tonight, tomorrow, and the weekend in one ranked side-by-side view.

  _Reach for this to find the cheapest night to stay in an area without running the search repeatedly._

  ```bash
  hotel-tonight-pp-cli datescan --lat 30.3071 --lng -97.7354 --agent
  ```
- **`daily-drop`** — Reveal today's Daily Drop hotel and its real discounted price for a market (the app hides it behind a slide-to-unlock gate), and with --history read the recorded run of past Daily Drops.

  _Use to see and track HotelTonight's exclusive once-a-day flash deal, which the app hides until you unlock it and erases each day._

  ```bash
  hotel-tonight-pp-cli daily-drop --metro 1 --agent
  ```

## Usage

Run `hotel-tonight-pp-cli --help` for the full command reference and flag list.

## Commands

### inventory

Last-minute hotel deal inventory by location

- **`hotel-tonight-pp-cli inventory`** - Search last-minute hotel deals near a latitude/longitude for a date range

### markets

HotelTonight markets (cities where deals are offered)

- **`hotel-tonight-pp-cli markets get`** - Get a single market by its numeric id
- **`hotel-tonight-pp-cli markets list`** - List HotelTonight's major markets with location, slug, and category prices
- **`hotel-tonight-pp-cli markets nearby`** - List popular/nearby markets for a given market id

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hotel-tonight-pp-cli inventory --latitude example-value --longitude example-value

# JSON for scripting and agents
hotel-tonight-pp-cli inventory --latitude example-value --longitude example-value --json

# Filter to specific fields
hotel-tonight-pp-cli inventory --latitude example-value --longitude example-value --json --select id,name,status

# Dry run — show the request without sending
hotel-tonight-pp-cli inventory --latitude example-value --longitude example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hotel-tonight-pp-cli inventory --latitude example-value --longitude example-value --agent
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
hotel-tonight-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **inventory search returns an error about a bad request** — Pass both --latitude and --longitude; the deal feed is geo-anchored and needs a coordinate pair.
- **history or verdict says no data for a hotel** — Run sync for that area first — those commands read the local snapshot store, which starts empty.
- **a market id is not in 'markets list'** — The list is a curated set of major markets; any numeric id still resolves via 'markets get <id>', and search works from any lat/lng.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
