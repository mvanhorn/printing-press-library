# Zaptec CLI

**The only command-line tool for Zaptec Go chargers — pause and resume charging, read live state in plain English, and track energy and cost from a local database.**

Control and monitor your Zaptec charger from the terminal or a cron job, without opening the portal app or running Home Assistant. Decodes the API's numeric observation and command IDs for you, syncs your charging history into a local SQLite store, and answers questions the portal can't — like monthly kWh and cost rollups (cost), instant fleet status (live), and offline-charger detection (chargers stale).

## Install

The recommended path installs both the `zaptec-pp-cli` binary and the `pp-zaptec` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install zaptec
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install zaptec --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zaptec-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zaptec --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zaptec --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-zaptec skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-zaptec. The skill defines how its required CLI can be installed.
```

## Authentication

Zaptec uses OAuth2 password grant. Run `zaptec-pp-cli auth login` with your Zaptec portal username and password (or set ZAPTEC_USERNAME and ZAPTEC_PASSWORD); the CLI exchanges them for a bearer token at https://api.zaptec.com/oauth/token, caches it locally, and refreshes it when it expires.

## Quick Start

```bash
# Exchange your Zaptec username/password for a cached bearer token
zaptec-pp-cli auth login


# Find your charger and copy its id
zaptec-pp-cli chargers list


# See decoded live state: mode, power, current, energy
zaptec-pp-cli state CHARGER_ID


# Preview a pause command before sending it
zaptec-pp-cli pause CHARGER_ID --dry-run


# Pull charging history into the local store for cost analytics
zaptec-pp-cli sync


# Monthly kWh and cost rollup the portal never shows
zaptec-pp-cli cost --by month --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`cost`** — See total kWh and cost for your charging over any period, rolled up by month or by charger.

  _Reach for this when the user asks how much they charged or what it cost over a period — it answers from local history instead of N paginated API calls._

  ```bash
  zaptec-pp-cli cost --by month --from 2026-01-01 --agent
  ```
- **`chargers stale`** — List chargers that are offline, disconnected, or haven't reported within a threshold.

  _Reach for this to catch dead or stuck chargers before a resident or driver complains._

  ```bash
  zaptec-pp-cli chargers stale --minutes 30 --agent
  ```
- **`sessions anomalies`** — Flag charging sessions with near-zero energy, abnormally long duration, or zero cost.

  _Reach for this to audit charging-session quality and catch stuck or mis-metered sessions._

  ```bash
  zaptec-pp-cli sessions anomalies --since 2026-04-01 --agent
  ```

### Decoded fleet visibility
- **`live`** — One table showing every charger's mode (charging/connected/finished/offline) with decoded power, current, and phase.

  _Reach for this for an instant fleet status check instead of clicking through chargers one by one in the portal._

  ```bash
  zaptec-pp-cli live --agent
  ```
- **`current headroom`** — Show how many amps of the installation's breaker limit are uncommitted versus actively drawn.

  _Reach for this before raising or lowering an installation's available current so the change is informed by real draw._

  ```bash
  zaptec-pp-cli current headroom 11111111-2222-3333-4444-555555555555 --agent
  ```
- **`firmware drift`** — Group an installation's chargers by firmware version and flag the ones behind the fleet majority.

  _Reach for this before scheduling firmware upgrades to see which chargers actually need one._

  ```bash
  zaptec-pp-cli firmware drift 11111111-2222-3333-4444-555555555555 --agent
  ```

## Usage

Run `zaptec-pp-cli --help` for the full command reference and flag list.

## Commands

### chargehistory

Manage chargehistory

- **`zaptec-pp-cli chargehistory get`** - Retrieves all completed charge sessions accessible by the current user,  
matching the provided filters.  
Default page size: 50. Max: 100.
- **`zaptec-pp-cli chargehistory installationreport-get`** - **Deprecated**: Use the POST version instead.  
Retrieves a usage report based on the provided filters (requires installation owner permissions).  
GET requests are limited to a 2048-character URL length and may fail if exceeded.
- **`zaptec-pp-cli chargehistory installationreport-post`** - Retrieves a usage report based on the provided filters (requires installation owner permissions).

### charger-firmware

Manage charger firmware

- **`zaptec-pp-cli charger-firmware installation-installationid-get`** - Retrieves firmware details for all chargers in the specified installation  
(requires owner or service permissions).

### chargers

Manage chargers

- **`zaptec-pp-cli chargers get`** - Retrieves all chargers accessible by the current user, matching the provided filters.  
By default, returns the first 50 items. Use `pageIndex` for pagination or adjust `pageSize` (max: 100).
- **`zaptec-pp-cli chargers id-get`** - Retrieves the specified charger (requires owner or service permissions).

### constants

Manage constants

- **`zaptec-pp-cli constants get`** - Retrieves a set of predefined constants that rarely change but may be updated occasionally.  
These constants include schemas, enumerations, supported languages, country data,  
charger operation modes, network types, user roles, error codes, settings, and more.


This data provides system-wide reference values, reducing the need for frequent queries.

### installation

Manage installation

- **`zaptec-pp-cli installation get`** - Retrieves all installations accessible by the current user, matching the provided filters.


By default, returns the first 50 items. Use <b>pageIndex</b> for pagination or adjust <b>pageSize</b> (max: 100).
- **`zaptec-pp-cli installation id-get`** - Retrieves details for the specified installation.  
The level of detail available depends on the current user's permissions.

### session

Manage session

- **`zaptec-pp-cli session id-get`** - Retrieves details for the specified charging session.

### user-groups

Manage user groups



## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zaptec-pp-cli chargehistory get

# JSON for scripting and agents
zaptec-pp-cli chargehistory get --json

# Filter to specific fields
zaptec-pp-cli chargehistory get --json --select id,name,status

# Dry run — show the request without sending
zaptec-pp-cli chargehistory get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zaptec-pp-cli chargehistory get --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-zaptec -g
```

Then invoke `/pp-zaptec <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add zaptec zaptec-pp-mcp -e ZAPTEC_LEGACY_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zaptec-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZAPTEC_LEGACY_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zaptec": {
      "command": "zaptec-pp-mcp",
      "env": {
        "ZAPTEC_LEGACY_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
zaptec-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/zaptec-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZAPTEC_LEGACY_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zaptec-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZAPTEC_LEGACY_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Run `zaptec-pp-cli auth login` again — the cached token expired or the credentials changed.
- **429 Too Many Requests** — The API caps at 10 req/s per account; the client backs off automatically, but avoid tight loops over many chargers.
- **current set refuses or warns about the 15-minute guard** — Zaptec enforces a 15-minute minimum between installation available-current updates; wait or pass the override flag if you accept the risk.
- **state shows raw numeric ids instead of names** — Run `zaptec-pp-cli sync` (or `constants`) to refresh the baked decode tables from /api/constants.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**custom-components/zaptec**](https://github.com/custom-components/zaptec) — Python
- [**evcc-io/evcc**](https://github.com/evcc-io/evcc) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
