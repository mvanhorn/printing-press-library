# Home Assistant CLI

**A unified interface for your smart home with offline search and streaming states.**

Call services, read sensors, and search your entire Home Assistant entity list instantly. Built with local SQLite caching so you never have to guess an entity ID again.

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

## Authentication

Requires a Long-Lived Access Token generated from your Home Assistant user profile.

## Quick Start

```bash
# Verify your token and connection.
homeassistant doctor


# Search for your light entities.
homeassistant find light


# Turn on a light.
homeassistant services call light.turn_on '{"entity_id": "light.living_room"}'

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`homeassistant find`** — Find exact entities instantly using full-text search across friendly names and attributes.

  _Agents can locate entity IDs without knowing the exact domain prefix._

  ```bash
  homeassistant find "living room light"
  ```

### Agent-native plumbing
- **`homeassistant monitor`** — Stream state changes live to the terminal.

  _Allows waiting for an event to happen rather than polling._

  ```bash
  homeassistant monitor light.living_room
  ```
- **`homeassistant services payload`** — Print the exact JSON payload expected by any Home Assistant service.

  _Self-documenting commands prevent malformed requests._

  ```bash
  homeassistant services payload light.turn_on
  ```
- **`homeassistant template`** — Render a Home Assistant Jinja2 template server-side.

  _Agents can compute derived values using HA's native template language._

  ```bash
  homeassistant template "{{ states.light | count }} lights"
  ```
- **`homeassistant events`** — List event types or fire custom events on the HA event bus.

  _Enables automation triggers and custom event-driven workflows._

  ```bash
  homeassistant events list
  ```

### Temporal intelligence
- **`homeassistant history`** — Query state change history for entities over a time period.

  _Agents can analyze trends and detect anomalies without polling._

  ```bash
  homeassistant history --entity sensor.living_room_temperature
  ```
- **`homeassistant logbook`** — View the activity logbook with human-readable event descriptions.

  _Provides context about what happened and why, not just state values._

  ```bash
  homeassistant logbook --entity alarm_control_panel.area_001
  ```

### Operations
- **`homeassistant check-config`** — Validate the Home Assistant configuration.yaml remotely.

  _Catch config errors before restarting HA._

  ```bash
  homeassistant check-config
  ```

## Usage

Run `homeassistant-pp-cli --help` for the full command reference and flag list.

## Commands

### default

Operations on default

- **`homeassistant-pp-cli default create_endpoint`** - POST /{id}

### hassio

Operations on info

- **`homeassistant-pp-cli hassio create_info`** - POST /api/hassio/addons/{id}/info
- **`homeassistant-pp-cli hassio create_install`** - POST /api/hassio/addons/{id}/install
- **`homeassistant-pp-cli hassio create_restart`** - POST /api/hassio/addons/{id}/restart
- **`homeassistant-pp-cli hassio create_start`** - POST /api/hassio/addons/{id}/start
- **`homeassistant-pp-cli hassio create_stop`** - POST /api/hassio/addons/{id}/stop
- **`homeassistant-pp-cli hassio create_uninstall`** - POST /api/hassio/addons/{id}/uninstall

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

### states

Operations on states

- **`homeassistant-pp-cli states get_states`** - GET /api/states/{id}

### websocket

Operations on websocket

- **`homeassistant-pp-cli websocket create_websocket`** - POST /api/websocket


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
homeassistant-pp-cli default mock-value

# JSON for scripting and agents
homeassistant-pp-cli default mock-value --json

# Filter to specific fields
homeassistant-pp-cli default mock-value --json --select id,name,status

# Dry run — show the request without sending
homeassistant-pp-cli default mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
homeassistant-pp-cli default mock-value --agent
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

### API-specific

- **Connection refused** — Ensure `HOMEASSISTANT_URL` includes the port (e.g. `http://homeassistant.local:8123/api`).

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Home Assistant REST API**](https://developers.home-assistant.io/docs/api/rest/) — Official API documentation
- [**homeassistant-cli**](https://github.com/home-assistant/home-assistant-cli) — Python (340 stars)
- [**ha-cli**](https://github.com/home-assistant/cli) — Go (200 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
