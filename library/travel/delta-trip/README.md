# Delta Trip CLI

Look up and manage Delta Air Lines trips by confirmation number — view all flights, seats, baggage, upgrade options, and flight details without logging in.

Learn more at [Delta Trip](https://www.delta.com).

Created by [@paulbockewitz](https://github.com/paulbockewitz) (Paul Bockewitz).

## Install

The recommended path installs both the `delta-trip-pp-cli` binary and the `pp-delta-trip` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install delta-trip
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install delta-trip --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install delta-trip --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install delta-trip --agent claude-code
npx -y @mvanhorn/printing-press-library install delta-trip --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/delta-trip/cmd/delta-trip-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/delta-trip-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install delta-trip --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-delta-trip --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-delta-trip --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install delta-trip --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/delta-trip-current).
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
    "delta-trip": {
      "command": "delta-trip-pp-mcp"
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
delta-trip-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
delta-trip-pp-cli flights --confirmation example-value --first-name example-resource --last-name example-resource
```

## Usage

Run `delta-trip-pp-cli --help` for the full command reference and flag list.

## Commands

### baggage

Baggage information and tracking for a trip

- **`delta-trip-pp-cli baggage`** - View baggage allowance and tracking info for a confirmation

### flights

Flight details for a trip — departure, arrival, seat, fare class, operator

- **`delta-trip-pp-cli flights`** - List all flights in a trip itinerary with details

### seats

Seat assignment and upgrade eligibility per passenger per flight

- **`delta-trip-pp-cli seats`** - View seat assignments and upgrade options for all passengers

### trips

Delta trip lookup and management by confirmation number

- **`delta-trip-pp-cli trips`** - Look up a trip by confirmation number, first name, and last name

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
delta-trip-pp-cli flights --confirmation example-value --first-name example-resource --last-name example-resource

# JSON for scripting and agents
delta-trip-pp-cli flights --confirmation example-value --first-name example-resource --last-name example-resource --json

# Filter to specific fields
delta-trip-pp-cli flights --confirmation example-value --first-name example-resource --last-name example-resource --json --select id,name,status

# Dry run — show the request without sending
delta-trip-pp-cli flights --confirmation example-value --first-name example-resource --last-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
delta-trip-pp-cli flights --confirmation example-value --first-name example-resource --last-name example-resource --agent
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
delta-trip-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## Browser Requirement

This CLI opens a **visible Chrome window** to fetch trip data from delta.com. The delta.com WAF blocks headless/automated HTTP clients, so a real Chrome browser session is required. Chrome must be installed on the system.

The browser window closes automatically after the trip data is retrieved. Subsequent calls within the 4-hour cache TTL are instant and require no browser.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
