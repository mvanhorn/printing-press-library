# SRAM AXS CLI

SRAM AXS CLI for bikes, components, registrations, ride activities, and synced component-summary analytics.

Created by [@stellato](https://github.com/stellato) (Greg Stellato).

## Install

The recommended path installs both the `axs-pp-cli` binary and the `pp-axs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install axs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install axs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install axs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install axs --agent claude-code
npx -y @mvanhorn/printing-press-library install axs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/axs/cmd/axs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/axs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install axs --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-axs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-axs --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install axs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/axs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SRAM_AXS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/axs/cmd/axs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "axs": {
      "command": "axs-pp-mcp",
      "env": {
        "SRAM_AXS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

AXS uses your SRAM account (Auth0). Run `axs-pp-cli auth login` to sign in with your email and password (handled directly against SRAM's token endpoint — no browser), or paste an access token with `axs-pp-cli auth set-token` / set SRAM_AXS_TOKEN. The public device-type catalog works with no login.

## Quick Start

```bash
# check reachability and config before anything else
axs-pp-cli doctor --dry-run

# public catalog — works with no login, proves the API is reachable
axs-pp-cli devicetypes list

# sign in with your SRAM account to unlock your bikes and components
axs-pp-cli auth login

# one-command overview of every bike and component
axs-pp-cli garage

# rank synced component usage from local component summaries
axs-pp-cli wear --json

# read latest synced battery_status per AXS device type
axs-pp-cli battery --json

# total front and rear shifts across synced ride summaries
axs-pp-cli shifts --totals --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Synced component-summary views
- **`firmware-check`** — Show the latest synced firmware version per AXS device type.

  _Pick this when an agent needs the current synced firmware signal without clicking through every component._

  ```bash
  axs-pp-cli firmware-check --json
  ```
- **`battery`** — Latest synced battery status per AXS device type, including voltage when available.

  _Pick this for a quick pre-ride battery signal across synced AXS device types._

  ```bash
  axs-pp-cli battery --json
  ```

### Local state that compounds
- **`wear`** — Rank components by synced shift counts, distance, and actuations from component summaries.

  _Use to find the most-used drivetrain component for maintenance planning._

  ```bash
  axs-pp-cli wear --json --select device_type,device_label,shift_count,fd_shift_count,rd_shift_count
  ```
- **`shifts`** — Show per-ride front/rear shift counts, chainrings, and cogs from synced component summaries.

  _Use for ride-by-ride drivetrain usage analysis without dumping thousands of raw gear samples._

  ```bash
  axs-pp-cli shifts --totals --json
  ```
- **`since`** — Show new ride activities and notifications since your last sync.

  _Pick this to catch up on what changed without re-reading everything._

  ```bash
  axs-pp-cli since 7d
  ```

### AXS web data joined for agents
- **`garage`** — A tree of every bike with its components and serials in one shot.

  _Use as the one-command overview of a rider's whole AXS setup._

  ```bash
  axs-pp-cli garage
  ```

## Recipes


### Narrow a verbose component list

```bash
axs-pp-cli components list --agent --select id,serial,model
```

Pulls only the high-gravity fields from a component response instead of the full payload.

### Register a part safely

```bash
axs-pp-cli registrations create --serial ABC123 --dry-run
```

Shows the registration request without sending it.

## Usage

Run `axs-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Your SRAM account profile and settings

- **`axs-pp-cli account accessgroups`** - Get your public access groups
- **`axs-pp-cli account export`** - Request a data export of your account
- **`axs-pp-cli account flags`** - Get your account feature flags
- **`axs-pp-cli account profile`** - Get your account profile

### activities

Your ride activities (telemetry host)

- **`axs-pp-cli activities list`** - List your ride activities
- **`axs-pp-cli activities types`** - List activity types

### bikes

Your registered bikes

- **`axs-pp-cli bikes get`** - Get a bike by ID
- **`axs-pp-cli bikes list`** - List your registered bikes

### components

Your AXS components (derailleurs, shifters, batteries, dropper posts)

- **`axs-pp-cli components get`** - Get a component by ID
- **`axs-pp-cli components list`** - List your AXS components

### devicetypes

Catalog of AXS device types (public, no login required)

- **`axs-pp-cli devicetypes`** - List all AXS device types

### linkedids

Linked third-party accounts (Strava, etc.)

- **`axs-pp-cli linkedids list`** - List your linked third-party accounts
- **`axs-pp-cli linkedids unlink`** - Unlink a third-party account

### models

Catalog of component models and their firmware versions

- **`axs-pp-cli models`** - List component models and latest firmware versions

### notifications

Your notifications inbox

- **`axs-pp-cli notifications`** - List your notifications

### products

Product detail lookups

- **`axs-pp-cli products <id>`** - Get product detail by product ID

### registrations

Register AXS components by serial number

- **`axs-pp-cli registrations`** - Register a component to your account by serial number

### stats

Aggregate account stats (telemetry host)

- **`axs-pp-cli stats`** - Get your aggregate riding stats

### summaries

Activity and component usage summaries (telemetry host)

- **`axs-pp-cli summaries activities`** - List per-activity summaries
- **`axs-pp-cli summaries components`** - List component usage summaries (wear, shift counts, distance)

### units

Measurement / advanced units catalog

- **`axs-pp-cli units`** - List advanced measurement units


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
axs-pp-cli activities list

# JSON for scripting and agents
axs-pp-cli activities list --json

# Filter to specific fields
axs-pp-cli activities list --json --select id,name,status

# Dry run — show the request without sending
axs-pp-cli activities list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
axs-pp-cli activities list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
axs-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/axs-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SRAM_AXS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `axs-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `axs-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SRAM_AXS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Authentication credentials were not provided** — Run `axs-pp-cli auth login` or set SRAM_AXS_TOKEN to a valid access token.
- **Token expired** — Run `axs-pp-cli auth login` again; access tokens are short-lived.
- **Empty bikes/components after login** — Run `axs-pp-cli sync` to populate the local mirror, then retry.

## Known Gaps

- **No firmware flashing / BLE control.** This CLI can read account, activity, and synced component-summary data exposed to AXS web clients. Updating firmware, pairing components, or changing shift behavior still happens over Bluetooth in the SRAM AXS phone app and is out of scope.
