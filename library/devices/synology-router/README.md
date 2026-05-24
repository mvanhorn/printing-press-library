# Synology Router CLI

REST-style API for managing Synology routers via SRM (Synology Router Manager).
All operations use form-encoded POST requests to /webapi/entry.cgi with
api, method, and version parameters. Authentication uses SYNO.API.Auth to
obtain a session ID (sid) stored as a Cookie header id=<sid>.
The router model is RT2600ac. Router IP defaults to 192.168.1.254 port 8001 (HTTPS).

Learn more at [Synology Router](https://www.synology.com).

Printed by [@e-jung](https://github.com/e-jung) (Eric Jung).

## Install

The recommended path installs both the `synology-router-pp-cli` binary and the `pp-synology-router` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install synology-router
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install synology-router --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install synology-router --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install synology-router --agent claude-code
npx -y @mvanhorn/printing-press install synology-router --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/synology-router-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-synology-router --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-synology-router --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-synology-router skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-synology-router. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/synology-router-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SYNOLOGY_ROUTER_COOKIE_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "synology-router": {
      "command": "synology-router-pp-mcp",
      "env": {
        "SYNOLOGY_ROUTER_COOKIE_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

> **Prerequisite**: You must be on the same LAN as your Synology RT2600ac (default IP: 192.168.1.254)
> or connected via a machine that can reach it (e.g., the NAS via Tailscale).

### 1. Install

See [Install](#install) above.

### 2. Authenticate

```bash
# Login and save the session ID automatically
synology-router-pp-cli login --account admin --passwd YOUR_PASSWORD

# Or export the session ID from a previous login
export SYNOLOGY_ROUTER_COOKIE_AUTH="<session-id>"
```

The session ID is saved to `~/.config/synology-router-pp-cli/config.toml` under `router_cookie_auth`.
SRM sessions expire after inactivity — re-run `login` when you see authentication errors.

### 3. Verify Setup

```bash
synology-router-pp-cli doctor
# Expected output: Config OK, Auth OK, API OK
```

### 4. Common Commands

```bash
# List all connected devices with online status
synology-router-pp-cli devices

# Live traffic — who's using bandwidth right now
synology-router-pp-cli traffic --interval live

# Top 5 devices by download today
synology-router-pp-cli traffic --interval day --top 5

# WAN connection status and external IP
synology-router-pp-cli wan

# Wake a device by MAC address
synology-router-pp-cli wol wake --mac AA:BB:CC:DD:EE:FF

# Wi-Fi networks
synology-router-pp-cli wifi settings-get

# Firewall rules
synology-router-pp-cli firewall rules-list
```

### 5. AI Agent Usage

```bash
# JSON output for AI agents
synology-router-pp-cli devices --agent

# Non-interactive with structured errors
synology-router-pp-cli traffic --interval day --top 10 --json

# MCP server for Claude Desktop / tool use
synology-router-pp-mcp
```

## Usage

Run `synology-router-pp-cli --help` for the full command reference and flag list.

## Commands

### access-control

Access control groups and safe access (SYNO.SafeAccess)

- **`synology-router-pp-cli access-control`** - Returns access control / safe access groups with their devices and time spent online.
Wire call: POST /webapi/entry.cgi with api=SYNO.SafeAccess.AccessControl.ConfigGroup, method=get, version=1.

### devices

Connected device management — IP, MAC, hostname, online status (SYNO.Core.Network.NSM.Device)

- **`synology-router-pp-cli devices`** - Lists all devices known to the router with their IP, MAC, hostname, and online status.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.NSM.Device, method=get, version=5.

### dns

DDNS records and external IP (SYNO.Core.DDNS)

- **`synology-router-pp-cli dns ddns-records-list`** - Returns all configured DDNS records and their update status.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.DDNS.Record, method=list, version=1.
- **`synology-router-pp-cli dns external-ip-get`** - Returns the current external (WAN) IP address.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.DDNS.ExtIP, method=list, version=1.

### firewall

Firewall policy route rules (SYNO.Core.Network.Router.PolicyRoute)

- **`synology-router-pp-cli firewall rules-list`** - Returns all IPv4 policy routing / firewall rules.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.Router.PolicyRoute, method=get, version=1.
- **`synology-router-pp-cli firewall rules-set`** - Replaces all policy routing rules. Requires the complete rules array.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.Router.PolicyRoute, method=set, version=1.

### mesh

Mesh network nodes and system info (SYNO.Mesh)

- **`synology-router-pp-cli mesh nodes-list`** - Returns all mesh nodes with status, current rate, and connected device counts.
Wire call: POST /webapi/entry.cgi with api=SYNO.Mesh.Node.List, method=get, version=4.
- **`synology-router-pp-cli mesh system-info-get`** - Returns SRM system information including firmware version, model, and uptime.
Wire call: POST /webapi/entry.cgi with api=SYNO.Mesh.System.Info, method=get, version=1.

### qos

Quality of Service rules (SYNO.Core.NGFW.QoS.Rules)

- **`synology-router-pp-cli qos`** - Returns Quality of Service rules per device including bandwidth guarantees and maximums.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.NGFW.QoS.Rules, method=get, version=1.

### session

Manage session

- **`synology-router-pp-cli session auth-login`** - Authenticates with the router using admin credentials and returns a session ID (sid).
Wire call: POST /webapi/auth.cgi with api=SYNO.API.Auth, method=Login, version=2.
The sid is stored in ~/.config/synology-router/session and sent as Cookie: id=<sid>.
- **`synology-router-pp-cli session auth-logout`** - Destroys the current session. Wire call: POST /webapi/auth.cgi with api=SYNO.API.Auth, method=Logout.

### smartwan

Smart WAN / multi-WAN configuration (SYNO.Core.Network.SmartWAN)

- **`synology-router-pp-cli smartwan smart-wan-config-get`** - Returns the Smart WAN (multi-WAN) configuration including failover/load-balancing mode.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.SmartWAN.General, method=get, version=1.
- **`synology-router-pp-cli smartwan smart-wan-config-set`** - Updates the Smart WAN (multi-WAN) configuration.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.SmartWAN.General, method=set, version=1.
- **`synology-router-pp-cli smartwan smart-wan-gateway-list`** - Returns all Smart WAN gateway configurations with their active/standby status.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.SmartWAN.Gateway, method=list, version=1.

### system

System information and utilization (SYNO.Core.System)

- **`synology-router-pp-cli system`** - Returns the list of all available SRM APIs with their supported versions.
Wire call: GET /webapi/query.cgi with api=SYNO.API.Info, method=query, version=1, query=ALL.
No authentication required. Useful for diagnostics.

### traffic

Network traffic and bandwidth statistics (SYNO.Core.NGFW.Traffic)

- **`synology-router-pp-cli traffic`** - Returns network traffic statistics per device for a given time interval.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.NGFW.Traffic, method=get, version=1.
Returns download/upload bytes per device with protocol breakdown.

### utilization

Manage utilization

- **`synology-router-pp-cli utilization`** - Returns CPU, memory, and network interface utilization statistics.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.System.Utilization, method=get, version=1.

### wan

WAN connection status (SYNO.Core.Network.Router.ConnectionStatus)

- **`synology-router-pp-cli wan`** - Returns the current WAN connection status including IPv4/IPv6 status, IP address,
gateway, DNS servers, and interface name.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.Router.ConnectionStatus, method=get, version=1.

### wifi

Wi-Fi network and client management (SYNO.Wifi.Network.Setting)

- **`synology-router-pp-cli wifi settings-get`** - Returns all Wi-Fi network profiles including SSIDs, security settings, and radio configuration.
Wire call: POST /webapi/entry.cgi with api=SYNO.Wifi.Network.Setting, method=get, version=1.
- **`synology-router-pp-cli wifi settings-set`** - Updates Wi-Fi network profiles. Requires the complete profiles array.
Wire call: POST /webapi/entry.cgi with api=SYNO.Wifi.Network.Setting, method=set, version=1.

### wol

Wake-on-LAN management (SYNO.Core.Network.WOL)

- **`synology-router-pp-cli wol device-add`** - Registers a device MAC address for Wake-on-LAN.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.WOL, method=add_device, version=1.
- **`synology-router-pp-cli wol devices-list`** - Returns all devices configured for Wake-on-LAN.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.WOL, method=get_devices, version=1.
- **`synology-router-pp-cli wol wake`** - Sends a Wake-on-LAN magic packet to the specified device.
Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.WOL, method=wake, version=1.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
synology-router-pp-cli devices

# JSON for scripting and agents
synology-router-pp-cli devices --json

# Filter to specific fields
synology-router-pp-cli devices --json --select id,name,status

# Dry run — show the request without sending
synology-router-pp-cli devices --dry-run

# Agent mode — JSON + compact + no prompts in one flag
synology-router-pp-cli devices --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Examples

```bash
# Authenticate and save session
$ synology-router-pp-cli login --account admin --passwd mysecretpassword

# Check health
$ synology-router-pp-cli doctor

# List all connected devices
$ synology-router-pp-cli devices

# Show only wireless devices
$ synology-router-pp-cli devices --conntype wireless

# Live traffic — who's eating bandwidth right now
$ synology-router-pp-cli traffic --interval live

# Top 5 devices by download today
$ synology-router-pp-cli traffic --interval day --top 5

# JSON output for piping / agents
$ synology-router-pp-cli traffic --interval day --json

# WAN status and external IP
$ synology-router-pp-cli wan

# Wake a device
$ synology-router-pp-cli wol wake --mac AA:BB:CC:DD:EE:FF

# Register a device for Wake-on-LAN
$ synology-router-pp-cli wol device-add --mac AA:BB:CC:DD:EE:FF --host mydesktop

# List WOL devices
$ synology-router-pp-cli wol devices-list

# Wi-Fi settings
$ synology-router-pp-cli wifi settings-get

# Firewall rules
$ synology-router-pp-cli firewall rules-list

# DDNS records
$ synology-router-pp-cli dns ddns-records-list

# External IP address
$ synology-router-pp-cli dns external-ip-get

# Mesh nodes
$ synology-router-pp-cli mesh nodes-list

# Router info (firmware, uptime, model)
$ synology-router-pp-cli mesh system-info-get

# QoS rules per device
$ synology-router-pp-cli qos

# Smart WAN config
$ synology-router-pp-cli smartwan smart-wan-config-get

# Access control groups
$ synology-router-pp-cli access-control

# MCP server for AI tool use
$ synology-router-pp-mcp
```

## Health Check

```bash
$ synology-router-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/synology-router-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SYNOLOGY_ROUTER_COOKIE_AUTH` | harvested | Yes | Populated automatically by auth login. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `synology-router-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SYNOLOGY_ROUTER_COOKIE_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## Cookbook

### Find which device is hogging bandwidth

```bash
# Top 10 devices by download today with human-readable sizes
$ synology-router-pp-cli busiest --interval day --top 10

# Live view of who's using bandwidth right now
$ synology-router-pp-cli traffic --interval live
```

### Quick network health check

```bash
# Single-command snapshot: WAN, devices, top talkers, mesh
$ synology-router-pp-cli health

# Full doctor check
$ synology-router-pp-cli doctor --json
```

### Find a specific device by name or IP

```bash
# Search local store (fast, requires prior sync)
$ synology-router-pp-cli search "laptop"

# Or search live device list
$ synology-router-pp-cli devices --json | jq '.[] | select(.hostname | test("laptop"; "i"))'
```

### Wake a sleeping machine remotely

```bash
# Register once, then wake anytime
$ synology-router-pp-cli wol device-add --mac AA:BB:CC:DD:EE:FF --host mydesktop
$ synology-router-pp-cli wol wake --mac AA:BB:CC:DD:EE:FF
```

### Audit network security in one command

```bash
# Firewall rules, Wi-Fi security, and access control
$ synology-router-pp-cli firewall rules-list --json
$ synology-router-pp-cli wifi settings-get --json
$ synology-router-pp-cli access-control --json
```

### Monitor router resources

```bash
# CPU, memory, and network utilization
$ synology-router-pp-cli utilization --resource '["cpu","memory","network"]'

# Check what's stale and needs re-sync
$ synology-router-pp-cli stale --older-than 24h
```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**synology-srm**](https://github.com/aerialls/synology-srm) — Python (44 stars)
- [**synology-srm-php-api**](https://github.com/nioc/synology-srm-php-api) — PHP (6 stars)
- [**synology-srm-nodejs-api**](https://github.com/nioc/synology-srm-nodejs-api) — JavaScript/Node.js (4 stars)
- [**SRM-reboot**](https://github.com/MHeuvel/SRM-reboot) — Python (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
