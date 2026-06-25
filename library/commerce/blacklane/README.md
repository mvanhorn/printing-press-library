# Blacklane CLI

**Upfront chauffeur quotes from the terminal — transfer and hourly, with a local price history no other tool has.**

Quote Blacklane's fixed all-inclusive chauffeur fares (airport transfers and by-the-hour) by address, compare vehicle classes, and keep a searchable local log of every quote. Addresses resolve via OpenStreetMap — no API key, no login, no booking.

Created by [@omarshahine](https://github.com/omarshahine) (Omar Shahine).

## Install

The recommended path installs both the `blacklane-pp-cli` binary and the `pp-blacklane` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install blacklane
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install blacklane --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install blacklane --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install blacklane --agent claude-code
npx -y @mvanhorn/printing-press-library install blacklane --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/blacklane/cmd/blacklane-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/blacklane-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install blacklane --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-blacklane --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-blacklane --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install blacklane --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/blacklane-current).
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
    "blacklane": {
      "command": "blacklane-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Point-to-point transfer quote across vehicle classes.
blacklane-pp-cli quote "San Francisco Airport" "Union Square San Francisco" --at 2026-06-25T15:00

# By-the-hour chauffeur quote (3 hours).
blacklane-pp-cli quote "Union Square San Francisco" --hourly 3 --at 2026-06-25T15:00

# Inspect a vehicle class's models, capacity, and features.
blacklane-pp-cli catalog business

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch`** — Track a saved route's price over time and flag drops.

  _Reach for this to catch chauffeur price drops before a trip._

  ```bash
  blacklane-pp-cli watch "San Francisco Airport" "Union Square San Francisco" --at 2026-06-25T15:00 --agent
  ```
- **`compare`** — Quote one route across many departure times to find the cheapest.

  _Use when the pickup time is flexible and price matters._

  ```bash
  blacklane-pp-cli compare "JFK Airport" "Times Square New York" --dates 2026-06-20T15:00,2026-06-21T15:00 --agent
  ```
- **`log`** — Every quote saved to SQLite, full-text searchable and SQL-queryable.

  _Use to recall and compare past quotes offline._

  ```bash
  blacklane-pp-cli search "Times Square" --agent
  ```

### Trip planning
- **`trip`** — Quote a sequence of legs and total the fares.

  _Use to budget a full day of ground transport._

  ```bash
  blacklane-pp-cli trip --leg "JFK Airport>Times Square New York" --at 2026-06-20T09:00 --agent
  ```
- **`fit`** — Recommend the cheapest vehicle class that fits the party.

  _Use to avoid overpaying for more car than you need._

  ```bash
  blacklane-pp-cli fit "JFK Airport" "Times Square New York" --pax 3 --bags 4 --at 2026-06-20T15:00 --agent
  ```

### Authenticated account
- **`bookings`** — List your upcoming and past Blacklane rides (requires auth login).

  _Pull your ride history/status without opening the site._

  ```bash
  blacklane-pp-cli bookings --when upcoming --agent
  ```
- **`me`** — Show your Blacklane profile (requires auth login).

  _Confirm which account the CLI is acting as._

  ```bash
  blacklane-pp-cli me --agent
  ```
- **`wallet`** — Show wallet credits and vouchers (requires auth login).

  _Check available credits before booking._

  ```bash
  blacklane-pp-cli wallet --agent
  ```

### Booking
- **`book`** — Quote and assemble a booking, then open browser checkout for payment under --confirm. Never charges.

  _Assemble and price a ride in the terminal, confirm payment yourself in the browser._

  ```bash
  blacklane-pp-cli book 'JFK Airport' 'Times Square New York' --at 2026-06-25T15:00 --class business
  ```

## Recipes

### Quote and keep only class + price

```bash
blacklane-pp-cli quote "JFK Airport" "Times Square New York" --at 2026-06-20T15:00 --agent --select packages.title,packages.grossAmount,packages.currency
```

Narrow a verbose quote to just the class and total.

### Find the cheapest departure

```bash
blacklane-pp-cli compare "JFK Airport" "Times Square New York" --dates 2026-06-20T15:00,2026-06-21T09:00,2026-06-22T09:00 --agent
```

Fan out quotes across times and rank by price.

## Usage

Run `blacklane-pp-cli --help` for the full command reference and flag list.

## Commands

### catalog

Vehicle-class service catalog (models, capacity, features)

- **`blacklane-pp-cli catalog <slug>`** - Get a vehicle class by slug (business, first, van)

### prices

Raw pricing quotes (prefer the top-level 'quote' command)

- **`blacklane-pp-cli prices`** - Request prices for a journey (raw body; see 'quote' for a friendly interface)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
blacklane-pp-cli catalog mock-value

# JSON for scripting and agents
blacklane-pp-cli catalog mock-value --json

# Filter to specific fields
blacklane-pp-cli catalog mock-value --json --select id,name,status

# Dry run — show the request without sending
blacklane-pp-cli catalog mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
blacklane-pp-cli catalog mock-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
blacklane-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Authentication

Quotes, catalog, and geocoding need no auth. The account commands and `book` use your own Blacklane login — easiest via Chrome:

```bash
blacklane-pp-cli auth login --chrome    # imports your 24h access token from Chrome (non-invasive)
# durable alternative: pbpaste | blacklane-pp-cli auth login   (refresh token from DevTools)
```

Credentials are stored owner-only at `~/.config/blacklane-pp-cli/auth.json`. `book` never charges — payment (Braintree + 3-D Secure) is completed by you in the browser.

## Configuration

Config file: `~/.config/blacklane-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Quote returns no vehicle classes** — The route may be outside Blacklane's service area, or the date is in the past — try a major city/airport and a future time.
- **Address resolves to the wrong place** — Pass more specific text or use --pickup-lat/--pickup-lng (and --dropoff-lat/lng) to set coordinates directly.
