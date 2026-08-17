# Airhint CLI

AirHint flight price prediction API — buy/wait recommendations for airline tickets

Learn more at [Airhint](https://www.airhint.com).

Created by [@jvm](https://github.com/jvm) (jvm).

## Install

The recommended path installs both the `airhint-pp-cli` binary and the `pp-airhint` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install airhint
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install airhint --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install airhint --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install airhint --agent claude-code
npx -y @mvanhorn/printing-press-library install airhint --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airhint-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install airhint --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airhint --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airhint --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install airhint --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airhint-current).
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
    "airhint": {
      "command": "airhint-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
airhint-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
airhint-pp-cli airport-autocomplete --query example-value
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compound query
- **`workflow predict-sweep`** — Sweep buy/wait predictions across a date range for a route, fetching live prices for each date

  _Enables batch price intelligence for flexible travelers — find the cheapest AND safest date to buy_

  ```bash
  airhint-pp-cli workflow predict-sweep STN DUB --days 14 --airline FR --json
  ```
- **`workflow compare-routes`** — Compare buy/wait predictions across multiple origin-destination pairs on the same date

  _One command to compare 3+ routes vs 3+ browser tabs — agent-native travel planning_

  ```bash
  airhint-pp-cli workflow compare-routes 2026-08-16 STN:DUB LGW:BCN MAD:LIS --json
  ```

### Local state that compounds
- **`workflow cheapest-window`** — Find the cheapest departure date in a given month using the cheapest-deal-month and cheapest-airline-deal-month endpoints

  _Answers 'what date in August is cheapest for STN→DUB?' without opening the browser_

  ```bash
  airhint-pp-cli workflow cheapest-window STN DUB 8 --airline FR
  ```

## Usage

Run `airhint-pp-cli --help` for the full command reference and flag list.

## Commands

### airport-autocomplete

Airport and city search autocomplete

- **`airhint-pp-cli airport-autocomplete`** - Search airports and cities by name or IATA code

### airport-names

Bulk airport name lookup

- **`airhint-pp-cli airport-names`** - Get airport names for a list of IATA codes

### cheapest-airline-deal-month

Find the cheapest deal for a specific airline in a month

- **`airhint-pp-cli cheapest-airline-deal-month <airline> <origin> <destination> <date> <currency>`** - Get cheapest fare for a specific airline on a route near a date

### cheapest-deal-month

Find the cheapest flight deal in a given month

- **`airhint-pp-cli cheapest-deal-month <origin> <destination> <month>`** - Get the cheapest one-way fare for a route in a given month

### flights

Flight search — find available flights and current prices

- **`airhint-pp-cli flights create-search`** - Initiate a flight search, returns a search_id for polling
- **`airhint-pp-cli flights get-search`** - Poll for search results using the search_id from create_search

### predict

Flight price prediction — buy or wait recommendations

- **`airhint-pp-cli predict <airline> <origin> <destination> <date>`** - Get buy/wait recommendation for a flight at a given price


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airhint-pp-cli airport-autocomplete --query example-value

# JSON for scripting and agents
airhint-pp-cli airport-autocomplete --query example-value --json

# Filter to specific fields
airhint-pp-cli airport-autocomplete --query example-value --json --select id,name,status

# Dry run — show the request without sending
airhint-pp-cli airport-autocomplete --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airhint-pp-cli airport-autocomplete --query example-value --agent
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
airhint-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/airhint-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://region1.analytics.google.com/g/collect
- Capture coverage: 8 API entries from 16 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: rest_json (75% confidence)
- Protection signals: cloudflare (90% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_airport_names — Derived from observed POST /airport-names traffic.; get_DUB — Derived from observed GET /cheapest-deal-month/one-way/STN/DUB/{dub_id} traffic.; get_search — Derived from observed GET /search/{search_id} traffic.; list_2026_08_16 — Derived from observed GET /predict/FR/STN/DUB/2026-08-16 traffic.; list_airport_autocomplete — Derived from observed GET /airport-autocomplete traffic.; list_inline_ads — Derived from observed GET /inline-ads traffic.; list_kayak_location_lookup — Derived from observed GET /kayak-location-lookup traffic.

Warnings from discovery:
- weak_schema_evidence: Binary or protobuf response cannot provide reliable JSON schema evidence.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
