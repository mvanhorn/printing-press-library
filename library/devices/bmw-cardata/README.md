# BMW CarData CLI

**Your BMW's telemetry, on the command line — live snapshots, charging history, and a local SQLite store for trends and fleet insight no other tool offers.**

BMW CarData is the official, free-for-personal-use API for your own vehicle data. This CLI handles the OAuth device-code onboarding, fetches live telematic snapshots, charging history and tyre diagnosis, then keeps it all in SQLite so you can chart SoC trends (soc-trends), reconcile charging cost against your tariff (charging-cost), and see every vehicle at once (fleet status).

## Install

The recommended path installs both the `bmw-cardata-pp-cli` binary and the `pp-bmw-cardata` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bmw-cardata
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bmw-cardata --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bmw-cardata --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bmw-cardata --agent claude-code
npx -y @mvanhorn/printing-press-library install bmw-cardata --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/cmd/bmw-cardata-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bmw-cardata-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bmw-cardata --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bmw-cardata --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bmw-cardata --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bmw-cardata --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bmw-cardata-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BMW_CARDATA_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/cmd/bmw-cardata-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bmw-cardata": {
      "command": "bmw-cardata-pp-mcp",
      "env": {
        "BMW_CARDATA_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

BMW CarData uses an OAuth2 Device Code Flow with PKCE. First, generate a client_id in the BMW CarData portal (My BMW > BMW CarData > Create CarData Client; tick 'Request access to CarData API', and 'CarData Stream' if you want streaming) and save it — the portal hides it on reload. Then run `bmw-cardata-pp-cli auth login`, open the printed verification URL, log in to BMW, and approve. The CLI stores the access and refresh tokens locally and refreshes them automatically. You can also export BMW_CARDATA_CLIENT_ID and run auth login non-interactively. Requires an EU vehicle with an active SIM, a ConnectedDrive contract, and the vehicle mapped to your account as the primary user.

## Quick Start

```bash
# Health check; works before any auth is configured.
bmw-cardata-pp-cli doctor --dry-run

# One-time device-code onboarding (generate a client_id in the BMW portal first).
bmw-cardata-pp-cli auth login

# List the VINs mapped to your account.
bmw-cardata-pp-cli customers get-mappings --json

# Fetch live telematic data for a container, narrowing the nested VSS payload (also writes through to local store).
bmw-cardata-pp-cli customers get-telematic-data WBAJB3105JUV12345 --container-id 11111111-1111-1111-1111-111111111111 --agent --select telematicData.vehicle.drivetrain.batteryManagement

# Transcendence: SoC/range trend from your local SQLite store.
bmw-cardata-pp-cli soc-trends WBAJB3105JUV12345 --window 30d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local telemetry that compounds
- **`soc-trends`** — See your battery state-of-charge and derived range as a time series you can chart.

  _Pick this when an agent needs SoC/range history or trend direction instead of a single live snapshot._

  ```bash
  bmw-cardata-pp-cli soc-trends WBAJB3105JUV12345 --window 30d --agent
  ```
- **`battery-health`** — Track battery capacity degradation by comparing observed max energy against the nameplate capacity.

  _Pick this when an agent must report long-term battery degradation rather than current state of charge._

  ```bash
  bmw-cardata-pp-cli battery-health WBAJB3105JUV12345 --agent
  ```
- **`vehicles diff`** — Show exactly what changed between two telematic snapshots (windows, charging, locks, location).

  _Pick this when an agent needs discrete state changes (did charging start? did a window open?) not a numeric level._

  ```bash
  bmw-cardata-pp-cli vehicles diff WBAJB3105JUV12345 --since 24h
  ```
- **`trips`** — Reconstruct trips (start, end, elapsed, distance) from your vehicle's location breadcrumbs.

  _Pick this when an agent needs driven trips and distances rather than raw GPS points._

  ```bash
  bmw-cardata-pp-cli trips WBAJB3105JUV12345 --since 7d
  ```

### Cost & fleet insight
- **`charging-cost`** — Reconcile your charging sessions against your electricity tariff and see DC charge efficiency.

  _Pick this when an agent must turn raw charging sessions into money spent and charging efficiency._

  ```bash
  bmw-cardata-pp-cli charging-cost WBAJB3105JUV12345 --tariff 0.32 --agent
  ```
- **`fleet status`** — One table of current SoC, range, charging state and last location across every vehicle on your account.

  _Pick this when an agent needs a single multi-vehicle overview instead of calling each VIN separately._

  ```bash
  bmw-cardata-pp-cli fleet status --agent
  ```

### Self-service plumbing
- **`quota`** — See how many of your ~50 daily API calls you have spent and when the budget resets (midnight UTC).

  _Pick this before any sync or polling loop so an agent does not burn the daily quota and start failing._

  ```bash
  bmw-cardata-pp-cli quota
  ```
- **`descriptors search`** — Search the VSS telematic descriptor catalogue (e.g. batteryManagement, charging, navigation) to build containers.

  _Pick this when an agent needs to discover the exact VSS descriptor paths to put in a container._

  ```bash
  bmw-cardata-pp-cli descriptors search "batteryManagement"
  ```

## Recipes


### Narrow a deeply nested telematic payload for an agent

```bash
bmw-cardata-pp-cli customers get-telematic-data WBAJB3105JUV12345 --container-id 11111111-1111-1111-1111-111111111111 --agent --select telematicData.vehicle.drivetrain.batteryManagement,telematicData.vehicle.drivetrain.electricEngine.charging
```

telematicData returns every descriptor as a nested {value,unit,timestamp} map; --select trims it to just the battery and charging keys so an agent does not parse tens of KB.

### Reconcile last month's charging against your tariff

```bash
bmw-cardata-pp-cli charging-cost WBAJB3105JUV12345 --tariff 0.32 --agent
```

Joins synced charging sessions with a per-kWh price locally to report money spent and DC efficiency.

### One-glance multi-car status

```bash
bmw-cardata-pp-cli fleet status --agent
```

Cross-joins every mapped VIN with its latest telematic snapshot for SoC, range, charging state and location.

### Watch today's API budget before syncing

```bash
bmw-cardata-pp-cli quota
```

Reads the local call counter against the ~50/day cap and reports the midnight-UTC reset time.

### Build a container from searched descriptors

```bash
bmw-cardata-pp-cli customers create-container --technical-descriptors $(bmw-cardata-pp-cli descriptors search batteryManagement --json | jq -r '.[].path') --dry-run
```

Search the VSS catalogue, then create a container of matching descriptors with a dry-run preview.

## Usage

Run `bmw-cardata-pp-cli --help` for the full command reference and flag list.

## Commands

### customers

Manage customers

- **`bmw-cardata-pp-cli customers create-container`** - Please note that some keys are bound to dedicated endpoints. These keys can be added to a container, but the corresponding endpoint for which the container is an input-parameter (/telematicData/), won't return these keys. For further details, check the Integration Guide (chapter 3.3.2).
- **`bmw-cardata-pp-cli customers delete-container`** - Delete container by ID
- **`bmw-cardata-pp-cli customers get-basic-data`** - Request basic vehicle data for the specified vehicle.
- **`bmw-cardata-pp-cli customers get-charging-history`** - Request the vehicle's Charging History sessions.
- **`bmw-cardata-pp-cli customers get-container-details`** - Please note that some keys are bound to dedicated endpoints. These keys can be added to a container, but the corresponding endpoint for which the container is an input-parameter (/telematicData/), won't return these keys. For further details, check the Integration Guide (chapter 3.3.2).
- **`bmw-cardata-pp-cli customers get-image`** - Request a vehicle image for the specified vehicle.
- **`bmw-cardata-pp-cli customers get-location-based-charging-settings`** - Request Vehicle's location based charging settings
- **`bmw-cardata-pp-cli customers get-mappings`** - Request mapped vehicles.
- **`bmw-cardata-pp-cli customers get-smart-maintenance-tyre-diagnosis`** - Request a vehicle Smart Maintenance Tyre Diagnosis for the specified vehicle.
- **`bmw-cardata-pp-cli customers get-telematic-data`** - Please note that some keys are bound to dedicated endpoints. These keys can be added to a container, but the corresponding endpoint for which the container is an input-parameter (/telematicData/), won't return these keys. For further details, check the Integration Guide (chapter 3.3.2).
- **`bmw-cardata-pp-cli customers list-containers`** - List the containers


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bmw-cardata-pp-cli customers create-container

# JSON for scripting and agents
bmw-cardata-pp-cli customers create-container --json

# Filter to specific fields
bmw-cardata-pp-cli customers create-container --json --select id,name,status

# Dry run — show the request without sending
bmw-cardata-pp-cli customers create-container --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bmw-cardata-pp-cli customers create-container --agent
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
bmw-cardata-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cardata-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BMW_CARDATA_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `bmw-cardata-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bmw-cardata-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BMW_CARDATA_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands start returning 429 / quota_exceeded late in the day** — Run `bmw-cardata-pp-cli quota`; BMW caps ~50 calls/day per client resetting at midnight UTC — rely on the local store and `sync` instead of repeated live calls.
- **auth login stays on 'authorization_pending'** — Open the printed verification_uri_complete in a browser, log in to BMW, and approve the device; polling continues until you do or the code expires.
- **auth fails with invalid_client after regenerating a client** — The portal hides the client_id on reload — re-create the CarData client, save the new id, and set BMW_CARDATA_CLIENT_ID before `auth login`.
- **stream connects but no data arrives** — Re-create the CarData client with 'CarData Stream' enabled and subscribe to the events you want; streaming permissions can only be set at client creation.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dj0abr/bmw-mqtt-bridge**](https://github.com/dj0abr/bmw-mqtt-bridge) — C++ (46 stars)
- [**tjamet/bmw-cardata**](https://github.com/tjamet/bmw-cardata) — Go (1 stars)
- [**kvanbiesen/bmw-cardata-ha**](https://github.com/kvanbiesen/bmw-cardata-ha) — Python
- [**whi-tw/bmw-cardata-streaming-poc**](https://github.com/whi-tw/bmw-cardata-streaming-poc) — Python
- [**bausi2k/bmw-python-streaming-mqtt-bridge**](https://github.com/bausi2k/bmw-python-streaming-mqtt-bridge) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
