# SolarEdge CLI

**Every SolarEdge Monitoring API endpoint, plus local history that answers questions the API itself can't.**

Existing SolarEdge wrappers (Python, Node, Go, Rust) are thin 1:1 API clients that return exactly what one endpoint gives you. This CLI computes things the API never does on its own: 'site underperformance' flags days statistically below the site's own trailing average, 'site changes' diffs the current period against the prior one, 'site health' combines four calls into a single go/no-go view, and 'budget status' tracks how much of the 300-request/day quota this CLI has used, since the API exposes no header or endpoint for it.

Created by [@garrickpg](https://github.com/garrickpg) (garrickpg).

## Install

The recommended path installs both the `solaredge-pp-cli` binary and the `pp-solaredge` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install solaredge
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install solaredge --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install solaredge --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install solaredge --agent claude-code
npx -y @mvanhorn/printing-press-library install solaredge --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/cmd/solaredge-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/solaredge-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install solaredge --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-solaredge --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-solaredge --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install solaredge --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/solaredge-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SOLAREDGE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/cmd/solaredge-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "solaredge": {
      "command": "solaredge-pp-mcp",
      "env": {
        "SOLAREDGE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

SolarEdge issues two kinds of API keys: an Account-level key (Account Admin > Company Details > API Access) that can see every site on the account, or a Site-level key (Site Admin > Site Details > API Access) scoped to one site. Either works with `SOLAREDGE_API_KEY`; account-level keys additionally unlock `accounts list` and bulk multi-site commands.

## Quick Start

```bash
# verify the binary and config resolve before touching the network
solaredge-pp-cli doctor --dry-run


# confirm the API key works and see which sites you can access
solaredge-pp-cli sites list --json


# check current power, today/month/lifetime energy for a known site
solaredge-pp-cli site overview 1223050 --json


# cache your site list locally so 'sites' searches and lookups work offline
solaredge-pp-cli sync --resources sites

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing

- **`site health`** — See one combined go/no-go status for a site instead of cross-referencing several separate calls.

  _Pick this over 'site overview' or 'site current-power-flow' when you need one answer to 'is this site OK right now'._

  ```bash
  solaredge-pp-cli site health 1223050 --json
  ```
- **`site underperformance`** — Flag days where production fell below this site's own historical average for that time of year.

  _Pick this when the question is 'is this normal for this site' rather than 'what changed recently' (use 'site changes' for the latter)._

  ```bash
  solaredge-pp-cli site underperformance 1223050 --since 30d --agent
  ```
- **`site changes`** — Get a short digest of the energy delta vs the prior period of equal length, plus a current equipment-count snapshot.

  _Pick this for a recent-delta digest; pick 'site underperformance' instead for statistical baseline comparisons._

  ```bash
  solaredge-pp-cli site changes 1223050 --since 7d --json
  ```
- **`equipment faults`** — See only the inverters, batteries, or system elements in a non-nominal state.

  _Use this for a filtered fault list; use 'equipment inventory' for the full unfiltered equipment list._

  ```bash
  solaredge-pp-cli equipment faults 1223050 --json
  ```

### Reachability mitigation

- **`budget status`** — See how much of today's 300-request quota this CLI has used, per site.

  _Check this before running several site commands in a row so a 429 doesn't interrupt you partway through._

  ```bash
  solaredge-pp-cli budget status --json
  ```

## Recipes


### Morning health check

```bash
solaredge-pp-cli site health 1223050
```

One combined go/no-go status instead of cross-referencing overview, power flow, and equipment calls by hand.

### Is this normal for this time of year

```bash
solaredge-pp-cli site underperformance 1223050 --since 30d --agent
```

Flags days statistically below this site's own trailing average, the question the raw API can never answer in one call.

### Pull only the meter breakdown an agent needs

```bash
solaredge-pp-cli site energy-details 1223050 --start-time '2026-06-01 00:00:00' --end-time '2026-06-30 23:59:59' --agent --select meters.type,meters.values.value
```

energyDetails returns a verbose nested meters array; --select narrows it to just the meter type and values an agent needs.

### Check the daily rate-limit budget before a big sync

```bash
solaredge-pp-cli budget status --json
```

Shows requests used/remaining against the 300/day cap before kicking off a fleet-wide sync that could trigger a 429.

## Usage

Run `solaredge-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SOLAREDGE_CONFIG_DIR`, `SOLAREDGE_DATA_DIR`, `SOLAREDGE_STATE_DIR`, or `SOLAREDGE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SOLAREDGE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SOLAREDGE_HOME=/srv/solaredge
solaredge-pp-cli doctor
```

Under `SOLAREDGE_HOME=/srv/solaredge`, the four dirs resolve to `/srv/solaredge/config`, `/srv/solaredge/data`, `/srv/solaredge/state`, and `/srv/solaredge/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "solaredge": {
      "command": "solaredge-pp-mcp",
      "env": {
        "SOLAREDGE_HOME": "/srv/solaredge"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SOLAREDGE_DATA_DIR` overrides an explicit `--home` for that kind. Use `SOLAREDGE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SOLAREDGE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `solaredge-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Installer account and sub-account management

- **`solaredge-pp-cli accounts`** - List the account and its sub-accounts (requires an account-level API key)

### api-version

Monitoring API version information

- **`solaredge-pp-cli api-version current`** - Most up-to-date API version number
- **`solaredge-pp-cli api-version supported`** - List of supported API version numbers

### equipment

Query SolarEdge equipment (inverters, optimizers, sensors)

- **`solaredge-pp-cli equipment inverter-data`** - Technical telemetry for one inverter (voltage, current, power) over a period (1 week max window)
- **`solaredge-pp-cli equipment list`** - List inverters/SMIs on this site, with name, model, manufacturer, serial number
- **`solaredge-pp-cli equipment sensors`** - List of sensors installed on this site and the gateway they connect to

### site

Query a single SolarEdge site

- **`solaredge-pp-cli site current-power-flow`** - Live power flow between PV, storage, load, and grid
- **`solaredge-pp-cli site data-period`** - Production data start/end dates for this site
- **`solaredge-pp-cli site details`** - Site details: name, location, status, peak power
- **`solaredge-pp-cli site energy`** - Site energy measurements (1 year max for DAY, 1 month max for QUARTER_OF_AN_HOUR/HOUR)
- **`solaredge-pp-cli site energy-details`** - Detailed energy by meter: production, consumption, self-consumption, feed-in, purchased
- **`solaredge-pp-cli site installer-image`** - Installer logo image as uploaded to the monitoring portal
- **`solaredge-pp-cli site inventory`** - Full equipment inventory: inverters, batteries, meters, gateways, sensors
- **`solaredge-pp-cli site overview`** - Current power and lifetime/yearly/monthly/daily energy and revenue
- **`solaredge-pp-cli site power`** - 15-minute resolution power measurements (1 month max window)
- **`solaredge-pp-cli site power-details`** - Detailed power by meter: production, consumption, self-consumption, feed-in, purchased (1 month max window)
- **`solaredge-pp-cli site sensor-data`** - Telemetry from all sensors on this site (1 week max window)
- **`solaredge-pp-cli site site-image`** - Site image as uploaded to the monitoring portal
- **`solaredge-pp-cli site storage-data`** - Battery state of energy, power, and lifetime energy (1 week max window)
- **`solaredge-pp-cli site time-frame-energy`** - Total energy produced over a given period (1 year max)

### sites

List and bulk-query SolarEdge sites

- **`solaredge-pp-cli sites data-period`** - Bulk: production data start/end dates for multiple sites
- **`solaredge-pp-cli sites energy`** - Bulk: site energy measurements for multiple sites
- **`solaredge-pp-cli sites list`** - List all sites visible to this API key, with search/sort/pagination
- **`solaredge-pp-cli sites overview`** - Bulk: current power and lifetime/yearly/monthly/daily energy for multiple sites
- **`solaredge-pp-cli sites power`** - Bulk: 15-minute resolution power measurements for multiple sites (max 1 month window)
- **`solaredge-pp-cli sites time-frame-energy`** - Bulk: total energy produced over a period for multiple sites


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
solaredge-pp-cli accounts

# JSON for scripting and agents
solaredge-pp-cli accounts --json

# Filter to specific fields
solaredge-pp-cli accounts --json --select id,name,status

# Dry run — show the request without sending
solaredge-pp-cli accounts --dry-run

# Agent mode — JSON + compact + no prompts in one flag
solaredge-pp-cli accounts --agent
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

## Health Check

```bash
solaredge-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `solaredge-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/solaredge-pp-cli/config.toml`; `--home`, `SOLAREDGE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SOLAREDGE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `solaredge-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `solaredge-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SOLAREDGE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 429 'too many requests'** — you've hit the 300-requests/day or 3-concurrent-call limit for this account+site; run `solaredge-pp-cli budget status` to see this CLI's tracked usage before retrying
- **HTTP 403 on a power/energy/storage call** — check the requested time window against the endpoint's limit (1 week for storage/sensor/inverter data, 1 month for power/powerDetails, 1 year for daily energy) — narrow the range and retry
- **HTTP 401 'Invalid API key'** — confirm SOLAREDGE_API_KEY is set to a key generated from Account Admin or Site Admin > API Access, not a portal password

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**solaredge-interface**](https://github.com/M-Fatah/solaredge-interface) — Python
- [**EVWorth/solaredge**](https://github.com/bertouttier/solaredge) — Python
- [**pysolaredge**](https://github.com/Yenthe666/pysolaredge) — Python
- [**clambin/solaredge**](https://github.com/clambin/solaredge) — Go
- [**shaungrady/solaredge-client**](https://github.com/shaungrady/solaredge-client) — JavaScript
- [**se_ms_api**](https://github.com/grtwje/se_ms_api) — Rust

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
