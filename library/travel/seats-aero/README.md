# Seats Aero CLI

Seats.aero Partner API for award travel availability, cached search, route lists, and trip revalidation details.

## Install

The recommended path installs both the `seats-aero-pp-cli` binary and the `pp-seats-aero` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install seats-aero
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install seats-aero --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seats-aero-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-seats-aero --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-seats-aero --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-seats-aero skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-seats-aero. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export SEATS_AERO_API_KEY="***"
```

`SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION` is also supported for generator compatibility. You can persist the key in `~/.config/seats-aero-pp-cli/config.toml`.

### 3. Verify Setup

```bash
seats-aero-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
seats-aero-pp-cli routes
```

## Usage

Run `seats-aero-pp-cli --help` for the full command reference and flag list.

## Commands

### availability

Manage availability

- **`seats-aero-pp-cli availability bulk`** - Retrieve bulk availability for all tracked routes in a mileage program.

### routes

Manage routes

- **`seats-aero-pp-cli routes get`** - Get all origin-destination routes tracked for a mileage program.

### seats-aero-partner-search

Manage seats aero partner search

- **`seats-aero-pp-cli seats-aero-partner-search cached-search`** - Search Seats.aero cached award availability between an origin and destination.

### trips

Manage trips

- **`seats-aero-pp-cli trips get`** - Get detailed trip information by revalidation/trip ID from search or availability results.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
seats-aero-pp-cli routes

# JSON for scripting and agents
seats-aero-pp-cli routes --json

# Filter to specific fields
seats-aero-pp-cli routes --json --select id,name,status

# Dry run — show the request without sending
seats-aero-pp-cli routes --dry-run

# Agent mode — JSON + compact + no prompts in one flag
seats-aero-pp-cli routes --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-seats-aero -g
```

Then invoke `/pp-seats-aero <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-mcp@latest
```

Then register it:

```bash
claude mcp add seats-aero seats-aero-pp-mcp -e SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seats-aero-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "seats-aero": {
      "command": "seats-aero-pp-mcp",
      "env": {
        "SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
seats-aero-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/seats-aero-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `seats-aero-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SEATS_AERO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
