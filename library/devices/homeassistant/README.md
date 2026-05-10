# Homeassistant CLI

Discovered API spec for homeassistant (crowd-sniff)

## Install

The recommended path installs both the `homeassistant-pp-cli` binary and the `pp-homeassistant` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install homeassistant
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install homeassistant --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/homeassistant-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-homeassistant --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-homeassistant --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-homeassistant skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-homeassistant. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
homeassistant-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export HOMEASSISTANT_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
homeassistant-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
homeassistant-pp-cli calendars get_calendar_events mock-value --start example-value --end example-value
```

## Usage

Run `homeassistant-pp-cli --help` for the full command reference and flag list.

## Commands

### calendars

Calendar entity operations

- **`homeassistant-pp-cli calendars get_calendar_events`** - Returns calendar events between start and end times
- **`homeassistant-pp-cli calendars list_calendars`** - Returns the list of calendar entities

### camera

Camera proxy

- **`homeassistant-pp-cli camera get_camera_image`** - Returns the image data from the specified camera entity

### components

Loaded integration components

- **`homeassistant-pp-cli components list_components`** - Returns a list of currently loaded components

### config

Home Assistant instance configuration

- **`homeassistant-pp-cli config check_config`** - Trigger a check of configuration.yaml and return validation result
- **`homeassistant-pp-cli config get_config`** - Returns the current configuration including version, location, timezone, and loaded components

### default

Operations on default

- **`homeassistant-pp-cli default create_endpoint`** - POST /{id}

### error_log

Error log retrieval

- **`homeassistant-pp-cli error_log get_error_log`** - Retrieve all errors logged during the current session as plaintext

### events

Event bus operations

- **`homeassistant-pp-cli events fire_event`** - Fires an event with event_type and optional event_data
- **`homeassistant-pp-cli events list_events`** - Returns an array of event objects with event name and listener count

### hassio

Operations on Hassio addon management

- **`homeassistant-pp-cli hassio create_info`** - POST /api/hassio/addons/{id}/info
- **`homeassistant-pp-cli hassio create_install`** - POST /api/hassio/addons/{id}/install
- **`homeassistant-pp-cli hassio create_restart`** - POST /api/hassio/addons/{id}/restart
- **`homeassistant-pp-cli hassio create_start`** - POST /api/hassio/addons/{id}/start
- **`homeassistant-pp-cli hassio create_stop`** - POST /api/hassio/addons/{id}/stop
- **`homeassistant-pp-cli hassio create_uninstall`** - POST /api/hassio/addons/{id}/uninstall

### history

State change history

- **`homeassistant-pp-cli history get_history`** - Returns state changes in the past, filtered by entity and time range

### intent

Intent handling

- **`homeassistant-pp-cli intent handle_intent`** - Handle an intent with name and data

### logbook

Activity logbook

- **`homeassistant-pp-cli logbook get_logbook`** - Returns an array of logbook entries with optional entity and time filters

### models

Operations on models

- **`homeassistant-pp-cli models create_models`** - POST /models

### ping

Operations on ping

- **`homeassistant-pp-cli ping delete_ping`** - DELETE /ping

### repository

Operations on install

- **`homeassistant-pp-cli repository create_install`** - POST /repository/install
- **`homeassistant-pp-cli repository create_uninstall`** - POST /repository/uninstall
- **`homeassistant-pp-cli repository create_update`** - POST /repository/update

### services

Service domain operations

- **`homeassistant-pp-cli services call_service`** - Calls a service within a specific domain with optional service_data
- **`homeassistant-pp-cli services list_services`** - Returns an array of service objects grouped by domain

### states

Operations on states

- **`homeassistant-pp-cli states delete_state`** - Deletes an entity with the specified entity_id
- **`homeassistant-pp-cli states get_states`** - Returns a state object for specified entity_id
- **`homeassistant-pp-cli states list_states`** - Returns an array of state objects with entity_id, state, last_changed and attributes
- **`homeassistant-pp-cli states update_state`** - Updates or creates a state representation

### template

Template rendering

- **`homeassistant-pp-cli template render_template`** - Render a Home Assistant Jinja2 template and return the result as plaintext

### websocket

Operations on websocket

- **`homeassistant-pp-cli websocket create_websocket`** - POST /api/websocket


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
homeassistant-pp-cli calendars get_calendar_events mock-value --start example-value --end example-value

# JSON for scripting and agents
homeassistant-pp-cli calendars get_calendar_events mock-value --start example-value --end example-value --json

# Filter to specific fields
homeassistant-pp-cli calendars get_calendar_events mock-value --start example-value --end example-value --json --select id,name,status

# Dry run — show the request without sending
homeassistant-pp-cli calendars get_calendar_events mock-value --start example-value --end example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
homeassistant-pp-cli calendars get_calendar_events mock-value --start example-value --end example-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-homeassistant -g
```

Then invoke `/pp-homeassistant <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add homeassistant homeassistant-pp-mcp -e HOMEASSISTANT_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/homeassistant-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HOMEASSISTANT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "homeassistant": {
      "command": "homeassistant-pp-mcp",
      "env": {
        "HOMEASSISTANT_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
homeassistant-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/homeassistant-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HOMEASSISTANT_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `homeassistant-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HOMEASSISTANT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
