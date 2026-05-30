# Global Fishing Watch CLI

**Every GFW vessel-behavior and risk surface on the command line, plus a local cache that turns one-shot lookups into a compounding due-diligence index.**

The official GFW SDKs are Python and R libraries; the one community CLI only downloads datasets. gfw-pp-cli is the agent-native, offline-caching CLI for vessel due diligence: search vessels, pull their events and risk insights, and run compound queries no SDK offers — 'vessel dossier' merges identity + events + insights, 'encounters network' maps at-sea meetings, 'vessel gaps' flags dark activity.

Printed by [@6myfzqx6bv-ctrl](https://github.com/6myfzqx6bv-ctrl).

## Install

The recommended path installs both the `gfw-pp-cli` binary and the `pp-gfw` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gfw
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gfw --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gfw --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gfw --agent claude-code
npx -y @mvanhorn/printing-press-library install gfw --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gfw-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gfw --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gfw --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-gfw skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-gfw. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gfw-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GFW_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gfw": {
      "command": "gfw-pp-mcp",
      "env": {
        "GFW_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

GFW uses a Bearer token. Create a free token at https://globalfishingwatch.org/our-apis/tokens and set it as GFW_TOKEN. Every command reads GFW_TOKEN from the environment.

## Quick Start

```bash
# Health check — confirms GFW_TOKEN is set and the API is reachable.
gfw-pp-cli doctor --dry-run

# Find vessels by name/MMSI/IMO/callsign and resolve GFW vessel IDs.
gfw-pp-cli vessel search "OCEAN" --json

# One-shot DD snapshot: identity + recent events + risk insights.
gfw-pp-cli vessel dossier <vesselId> --json

# Surface AIS-gap / dark-activity events for a vessel.
gfw-pp-cli vessel gaps <vesselId> --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compound DD intelligence
- **`vessel dossier`** — One-shot due-diligence snapshot of a vessel: identity, recent events, and risk insights merged.

  _Reach for this first when vetting a vessel — it answers identity + behavior + risk in one call instead of three._

  ```bash
  gfw-pp-cli vessel dossier 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```
- **`vessel risk`** — Composite risk signal from GFW Insights indicators plus event patterns (encounters, AIS gaps, port visits).

  _Use to triage which vessels in a set warrant deeper review._

  ```bash
  gfw-pp-cli vessel risk 8c7304226-6c71-edbe-0b63-c246734b3c01 --agent
  ```
- **`encounters network`** — Builds the at-sea meeting graph for a vessel — which other vessels it encountered, when, and where.

  _Use to surface relationships (e.g. transshipment partners) a single-vessel lookup hides._

  ```bash
  gfw-pp-cli encounters network 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```
- **`vessel ports`** — Aggregates a vessel's port-visit events into a frequency/recency pattern.

  _Use to spot anomalous port behavior (sanctioned ports, sudden pattern change)._

  ```bash
  gfw-pp-cli vessel ports 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```
- **`vessel gaps`** — Surfaces AIS-gap and loitering events as a dark-activity signal for a vessel.

  _Use to flag possible AIS disabling — a classic sanctions-evasion indicator._

  ```bash
  gfw-pp-cli vessel gaps 8c7304226-6c71-edbe-0b63-c246734b3c01 --json
  ```

### Watchlist that compounds
- **`watch pin`** — Pin vessels under active due diligence to a local watchlist (with optional label).

  _Use to track a set of vessels across sessions; 'watch --list' shows them._

  ```bash
  gfw-pp-cli watch pin 8c7304226-6c71-edbe-0b63-c246734b3c01 --label "Lagos deal"
  ```
- **`watch refresh`** — Re-pull events and insights for watchlisted vessels under a polite throttle.

  _Use to bring a watchlist current before a review._

  ```bash
  gfw-pp-cli watch refresh --pinned
  ```
- **`watch since`** — Shows new events for watchlisted vessels within a time window.

  _Use for "what happened to my vessels in the last N days"._

  ```bash
  gfw-pp-cli watch since 7d --json
  ```

## Recipes


### Due-diligence snapshot

```bash
gfw-pp-cli vessel dossier <vesselId> --json
```

Identity, recent events, and risk insights merged in one call.

### Narrow a verbose events payload for an agent

```bash
gfw-pp-cli events list --vessels-0 <vesselId> --agent --select entries.type,entries.start
```

Events responses are large; --select with dotted paths keeps only the fields an agent needs.

### Map transshipment partners

```bash
gfw-pp-cli encounters network <vesselId> --json
```

Builds the at-sea encounter graph from cached events.

### Track a watchlist

```bash
gfw-pp-cli watch pin <vesselId> --label "case-42" && gfw-pp-cli watch since 7d
```

Pin vessels and review what changed in the last week.

## Usage

Run `gfw-pp-cli --help` for the full command reference and flag list.

## Commands

### 4wings

Manage 4wings

- **`gfw-pp-cli 4wings create`** - Report with vessel_type filter
- **`gfw-pp-cli 4wings list`** - Last report
- **`gfw-pp-cli 4wings list-report`** - Report, Carrier Vessels Only
- **`gfw-pp-cli 4wings list-stats`** - Get fishing effort stats for a time period with filter by distance from port(km)

### bulk-reports

Manage bulk reports

- **`gfw-pp-cli bulk-reports create`** - Create bulk report fixed infrastructure in ARG EEZ + structure ID
- **`gfw-pp-cli bulk-reports get`** - Get bulk report by id
- **`gfw-pp-cli bulk-reports list`** - Get all reports by user

### datasets

Manage datasets

- **`gfw-pp-cli datasets`** - SAR Fixed infra MVT - no filter Copy

### events

Manage events

- **`gfw-pp-cli events create`** - Get GAP events POST - Filter by flag custom polygon GAP
- **`gfw-pp-cli events create-stats`** - Port visits events Stats - includes TOTAL_COUNT and vessel type BUNKER
- **`gfw-pp-cli events get`** - Get one event by id port visit
- **`gfw-pp-cli events list`** - Get Events - several vessel ids

### insights

Manage insights

- **`gfw-pp-cli insights`** - Insights API - all carrier and fishing vessels

### vessels

Manage vessels

- **`gfw-pp-cli vessels get`** - Obtains all the characteristics that describe a single vessel, such as its name and idenTIFiers.
- **`gfw-pp-cli vessels list`** - Lists vessels given a list of vessels id.
- **`gfw-pp-cli vessels list-search`** - Advanced Search vessels - Gear Type = 'CARRIER'


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gfw-pp-cli 4wings list

# JSON for scripting and agents
gfw-pp-cli 4wings list --json

# Filter to specific fields
gfw-pp-cli 4wings list --json --select id,name,status

# Dry run — show the request without sending
gfw-pp-cli 4wings list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gfw-pp-cli 4wings list --agent
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

## Health Check

```bash
gfw-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gfw-public-endpoints-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GFW_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `gfw-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gfw-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GFW_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — Set GFW_TOKEN (free token at globalfishingwatch.org/our-apis/tokens); run 'gfw-pp-cli doctor'.
- **429 Too Many Requests** — Lower --rate-limit or use 'watch refresh' which throttles; bulk pulls should use the bulk-reports flow.
- **Empty events for a known vessel** — Widen the date range and confirm the GFW vessel ID (not MMSI) via 'vessel search'.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gfw-api-python-client**](https://github.com/GlobalFishingWatch/gfw-api-python-client) — Python
- [**gfwr**](https://github.com/GlobalFishingWatch/gfwr) — R
- [**gfw (samapriya)**](https://github.com/samapriya/gfw) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
