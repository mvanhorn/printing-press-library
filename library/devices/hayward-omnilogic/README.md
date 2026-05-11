# Hayward OmniLogic CLI

**Take control of every Hayward OmniLogic feature, plus a local store, schedule diffs, chemistry trends, and a morning multi-site sweep no other tool offers.**

Drives the same partner API the Hayward mobile app and Home Assistant integration use, but adds the historical chemistry log, the equipment diagnostic the cloud refuses to compute, the schedule-change detector for service techs, and the multi-site alarm sweep pool-service businesses ask for. Agent-native JSON throughout, with a local SQLite store that turns single-shot cloud reads into compound answers.

## Install

The recommended path installs both the `hayward-omnilogic-pp-cli` binary and the `pp-hayward-omnilogic` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install hayward-omnilogic
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install hayward-omnilogic --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hayward-omnilogic-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hayward-omnilogic --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hayward-omnilogic --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-hayward-omnilogic skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-hayward-omnilogic. The skill defines how its required CLI can be installed.
```

## Authentication

OmniLogic uses a two-stage auth: a JSON login against services-gamma.haywardcloud.net returns a token + refresh token, then every operation POSTs an XML envelope to the legacy HAAPI endpoint with the token in the header. The CLI caches the token under your XDG state directory and refreshes it transparently. Set HAYWARD_USER (your account email) and HAYWARD_PW; the first call to any command logs in automatically.

## Quick Start

```bash
# Verify auth + reachability + token cache before anything else.
hayward-omnilogic-pp-cli doctor


# Pull sites, equipment inventory, current telemetry, and alarms into the local store.
hayward-omnilogic-pp-cli sync --full


# One-shot pool-readiness composite — chemistry, temp, alarms, pump state with a traffic-light verdict.
hayward-omnilogic-pp-cli status --json


# Export the last week's pH/ORP/salt/temp readings for service records.
hayward-omnilogic-pp-cli chemistry log --since 7d --csv


# Enable the heater and compute when to start it so the pool reaches 84°F by 6pm.
hayward-omnilogic-pp-cli ready-by 18:00 --temp 84

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Pool readiness at a glance
- **`status`** — One-shot "is the pool ready for guests?" view: chemistry in range, temp at setpoint, no active alarms, pump running, with a traffic-light verdict.

  _Reach for this when an agent or user wants a one-shot pool-state summary instead of correlating three commands._

  ```bash
  hayward-omnilogic-pp-cli status --json
  ```
- **`ready-by`** — Enables the heater and computes when to start it so the pool hits your target temperature by a specified arrival time, using the learned heat rate from telemetry history.

  _Use this instead of "set heater + setpoint and guess" when the user has a specific swim time._

  ```bash
  hayward-omnilogic-pp-cli ready-by 18:00 --temp 84
  ```

### Local state that compounds
- **`chemistry log`** — Weekly/monthly pH, ORP, salt, and temperature history from the local store, exportable as CSV or JSON for HOA / service / insurance records.

  _Use this when an agent needs trend data or a compliance log over a date range._

  ```bash
  hayward-omnilogic-pp-cli chemistry log --since 30d --csv
  ```
- **`chemistry drift`** — Detects pH/ORP/salt drift versus a rolling baseline before Hayward's static thresholds fire; --forecast projects when each metric will leave the safe range.

  _Reach for this when the user wants early warning, not just "alarm is active."_

  ```bash
  hayward-omnilogic-pp-cli chemistry drift --forecast --json
  ```
- **`runtime`** — Pump hours, heater hours, salt-cell hours derived from telemetry deltas — for maintenance planning, warranty, and end-of-season service.

  _Use this when a service-business agent needs maintenance projections or a warranty case-file._

  ```bash
  hayward-omnilogic-pp-cli runtime --since 90d --json
  ```
- **`command-log`** — Every Set* command issued via this CLI is logged with who/when/what/result. --replay <id> re-issues a prior command for quick undo or redo.

  _Use this when an agent or operator needs to know what was changed recently or roll back a misfire._

  ```bash
  hayward-omnilogic-pp-cli command-log --since 7d
  ```

### Diagnostics the cloud can't do
- **`why-not-running`** — Diagnose why a pump, heater, or light isn't running: correlates active alarms, current relay state, scheduled run windows, heater demand, and superchlor lockouts into one explanation.

  _Reach for this when an agent is asked to triage "X isn't running" instead of telling the user to open the app._

  ```bash
  hayward-omnilogic-pp-cli why-not-running 'Main Pump'
  ```
- **`schedule diff`** — Diffs today's MSP-config schedule tree against prior versioned snapshots; catches silent edits made by service techs or by other app users.

  _Use this when a user reports unexpected pool behavior and you need to know what changed._

  ```bash
  hayward-omnilogic-pp-cli schedule diff --since yesterday
  ```

### Multi-site operations
- **`sweep`** — Across every site in the account, surface active alarms, out-of-range chemistry, and offline controllers in a single report — built for pool-service businesses doing route planning.

  _Use this when an agent is asked to prioritize the day's truck rolls across many pools._

  ```bash
  hayward-omnilogic-pp-cli sweep --alarms --chemistry --json
  ```

## Usage

Run `hayward-omnilogic-pp-cli --help` for the full command reference and flag list.

## Commands

### alarms

Active alarms across the OmniLogic system

- **`hayward-omnilogic-pp-cli alarms list`** - List active alarms for a site or every site.

### chemistry

Chemistry-only view of telemetry plus historical-store-backed readouts

- **`hayward-omnilogic-pp-cli chemistry get`** - Current chemistry snapshot: pH, ORP, salt, water temp, with safe-range verdict.

### chlorinator

Salt chlorinator configuration

- **`hayward-omnilogic-pp-cli chlorinator set_params`** - Set chlorinator config (op mode, timed percent, ORP timeout, etc). Defaults to current MSP values for any flag you don't pass.

### config

Equipment inventory (MSP config tree) per site

- **`hayward-omnilogic-pp-cli config get`** - Fetch the equipment inventory for one site — pumps, heaters, chlorinator, lights, valves, relays, sensors.

### equipment

Generic on/off + timed-run control for valves, relays, lights, and accessory pumps

- **`hayward-omnilogic-pp-cli equipment off`** - Turn an equipment item off.
- **`hayward-omnilogic-pp-cli equipment on`** - Turn an equipment item on. Use --for to run for a bounded duration; otherwise stays on until you turn it off.

### heater

Heater control (enable/disable + setpoint)

- **`hayward-omnilogic-pp-cli heater disable`** - Turn a heater off.
- **`hayward-omnilogic-pp-cli heater enable`** - Turn a heater on. Heater stays on until set-temp is reached or you disable it.
- **`hayward-omnilogic-pp-cli heater set_temp`** - Set a heater's target setpoint in degrees Fahrenheit.

### light

ColorLogic light shows

- **`hayward-omnilogic-pp-cli light list_shows`** - List every available ColorLogic show with its numeric ID and human-readable name.
- **`hayward-omnilogic-pp-cli light show`** - Activate a ColorLogic show. V2 lights also accept --speed and --brightness.

### pump

Variable-speed pump control

- **`hayward-omnilogic-pp-cli pump set_speed`** - Set a pump's running speed in RPM or percent (range comes from the pump's MSP config).

### sites

Hayward OmniLogic sites (one per backyard controller registered to your account)

- **`hayward-omnilogic-pp-cli sites list`** - List every site registered under your Hayward account.

### spillover

Spillover control (pool-to-spa overflow)

- **`hayward-omnilogic-pp-cli spillover set`** - Set spillover speed and optional run duration.

### superchlor

Manual superchlorination (one-shot)

- **`hayward-omnilogic-pp-cli superchlor off`** - Stop superchlorination.
- **`hayward-omnilogic-pp-cli superchlor on`** - Start superchlorination on the body-of-water's salt chlorinator. Runs until the configured SCTimeout expires or you turn it off.

### telemetry

Live state of every equipment item at one site

- **`hayward-omnilogic-pp-cli telemetry get`** - Snapshot live state: chemistry (pH/ORP/salt), water and air temperature, pump speeds, heater enable, light state, alarm flags. Append-only-stored in telemetry_samples.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hayward-omnilogic-pp-cli alarms

# JSON for scripting and agents
hayward-omnilogic-pp-cli alarms --json

# Filter to specific fields
hayward-omnilogic-pp-cli alarms --json --select id,name,status

# Dry run — show the request without sending
hayward-omnilogic-pp-cli alarms --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hayward-omnilogic-pp-cli alarms --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-hayward-omnilogic -g
```

Then invoke `/pp-hayward-omnilogic <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add hayward-omnilogic hayward-omnilogic-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hayward-omnilogic-current).
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
    "hayward-omnilogic": {
      "command": "hayward-omnilogic-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
hayward-omnilogic-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/hayward-omnilogic/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 / "Failed Login" on first call** — Confirm HAYWARD_USER is the email you log in with at haywardomnilogic.com (post-Oct-2025 the API rejects username-only); rerun `doctor` to re-cache the token.
- **Token works for a day then fails** — Token cache TTL is 24h; the CLI auto-refreshes via /auth-service/v2/refresh. If refresh fails (e.g., laptop slept past expiry), delete `~/.local/state/hayward-omnilogic/auth.json` and rerun any command.
- **Schedule diff says "no baseline" on first run** — Run `sync --full` at least once so the CLI has a prior MSP-config snapshot to compare against.
- **chemistry log returns empty** — telemetry samples are appended by `sync` (or any read-side call); after `sync --full` you have one point. Run `sync` periodically (cron / launchd / 30-60s minimum interval per community guidance).

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**djtimca/omnilogic-api**](https://github.com/djtimca/omnilogic-api) — Python (27 stars)
- [**home-assistant/core omnilogic**](https://github.com/home-assistant/core/tree/dev/homeassistant/components/omnilogic) — Python
- [**djtimca/haomnilogic**](https://github.com/djtimca/haomnilogic) — Python
- [**openhab/openhab-addons haywardomnilogic**](https://github.com/openhab/openhab-addons/tree/main/bundles/org.openhab.binding.haywardomnilogic) — Java

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
