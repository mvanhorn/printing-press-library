# ExchangeRate-API CLI

**Every ExchangeRate-API endpoint, plus a local rate history, watchlists, drift alerts, and an MCP server no one else ships.**

This CLI exposes every ExchangeRate-API endpoint (latest, pair, codes, quota, enriched, history) as typed commands and layers on offline-first features the API can't provide: local snapshots in SQLite, watchlists with drift alerts, quota burn projection, N×N cross-rate matrices from a single API call, and an MCP server for AI agents. Free-tier users get time travel from their own captured history without paying for /history access.

Created by [@vinnyp](https://github.com/vinnyp) (Vinny Pasceri).

## Install

The recommended path installs both the `exchangerate-api-pp-cli` binary and the `pp-exchangerate-api` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install exchangerate-api
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install exchangerate-api --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install exchangerate-api --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install exchangerate-api --agent claude-code
npx -y @mvanhorn/printing-press-library install exchangerate-api --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/cmd/exchangerate-api-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/exchangerate-api-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install exchangerate-api --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-exchangerate-api --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-exchangerate-api --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install exchangerate-api --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/exchangerate-api-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EXCHANGERATE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "exchangerate-api": {
      "command": "exchangerate-api-pp-mcp",
      "env": {
        "EXCHANGERATE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `EXCHANGERATE_API_KEY` from your ExchangeRate-API dashboard (https://app.exchangerate-api.com/). All commands route through `https://v6.exchangerate-api.com/v6/{key}/...`. The `open` command uses the keyless `https://open.er-api.com/v6/latest/{base}` endpoint and requires the 'Rates By Exchange Rate API' attribution if you redistribute its output.

## Quick Start

```bash
# Confirms key works and the API is reachable
exchangerate-api-pp-cli doctor

# Pulls latest rates and seeds local snapshots; foundation for offline + history features
exchangerate-api-pp-cli sync-rates --base USD

# Single-pair lookup, the most common request
exchangerate-api-pp-cli rates pair USD EUR

# Multi-target conversion with one API call
exchangerate-api-pp-cli convert 250 USD EUR,GBP,JPY --json

# Cross-rate matrix — N² rates, 1 quota tick
exchangerate-api-pp-cli matrix USD,EUR,GBP,JPY --base USD --json

# How fast you're burning the monthly 1500-call quota
exchangerate-api-pp-cli quota burn --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`history-cache`** — Reconstruct historical FX rates from your prior `latest` syncs — free tier gets time travel without `/history` quota.

  _Use when the user asks for past rates and the API key is on the Free or Standard tier where `/history` returns plan-upgrade-required._

  ```bash
  exchangerate-api-pp-cli history-cache USD EUR --since 30d --json
  ```
- **`watch check`** — Define currency pairs with movement thresholds; one command reports which crossed since the last check.

  _Use as a periodic monitor (cron/agent loop) instead of polling a paid alerting service._

  ```bash
  exchangerate-api-pp-cli watch check --json
  ```
- **`drift`** — Diff the latest snapshot against any prior point and report the biggest movers as ranked output.

  _Use to surface unusual FX moves the user cares about without writing a custom analysis._

  ```bash
  exchangerate-api-pp-cli drift --base USD --since 24h --top 10 --json
  ```
- **`pair --as-of`** — Look up the rate from local snapshots at any historical timestamp — free tier time travel.

  _Use when reconstructing the rate that was live at a specific moment without needing the paid `/history` endpoint._

  ```bash
  exchangerate-api-pp-cli rates pair USD EUR --as-of 1h --json
  ```

### Quota intelligence
- **`quota burn`** — Fit a trend over `quota_snapshots` and project when your quota will exhaust before refresh day.

  _Use before deploying anything that will hammer the API — the projection tells you if you'll survive the month._

  ```bash
  exchangerate-api-pp-cli quota burn --json
  ```
- **`matrix`** — Show full cross-rate matrix for any set of currencies from a single `latest` call — N² rates, 1 quota tick.

  _Use when comparing many pairs at once; saves quota and runs offline after one sync._

  ```bash
  exchangerate-api-pp-cli matrix USD,EUR,GBP,JPY --base USD --json
  ```
- **`convert-batch`** — Pipe a list of amounts to a single command; converts all of them with one rate fetch.

  _Use when processing many amounts (orders, transactions) without burning the quota linearly._

  ```bash
  exchangerate-api-pp-cli convert-batch --from USD --to EUR --input /dev/null --json
  ```

### Onboarding
- **`plan-check`** — Probe each tier-gated endpoint with a single low-cost request; reports which tier your key supports.

  _Use right after `doctor` to know which commands will work without `plan-upgrade-required`._

  ```bash
  exchangerate-api-pp-cli plan-check --json
  ```

### Agent-native plumbing
- **`log show`** — Every `convert` call is logged; query the log by base, target, time window for recurring conversion analysis.

  _Use to remind an agent what FX work was done recently without re-querying the API._

  ```bash
  exchangerate-api-pp-cli log show --since 7d --json
  ```
- **`mcp serve`** — Every user-facing command is exposed as an MCP tool with read-only annotations on safe queries.

  _Use when wiring Claude/Cursor/Windsurf to FX context — no competing MCP for this API today._

  ```bash
  exchangerate-api-pp-cli mcp serve
  ```

## Usage

Run `exchangerate-api-pp-cli --help` for the full command reference and flag list.

## Commands

The API key is supplied via the `EXCHANGERATE_API_KEY` environment variable, never as a positional argument — every command below reads it from the environment.

### codes

Supported currency codes

- **`exchangerate-api-pp-cli codes`** - List all 161 supported currency codes

### quota

Your API quota usage

- **`exchangerate-api-pp-cli quota`** - Check remaining requests in the current quota period
- **`exchangerate-api-pp-cli quota burn`** - Project quota exhaustion from quota_snapshots history

### rates

Live exchange rates and conversions

- **`exchangerate-api-pp-cli rates latest <base_code>`** - Get all exchange rates from a base currency
- **`exchangerate-api-pp-cli rates pair <base_code> <target_code>`** - Get conversion rate between two currencies
- **`exchangerate-api-pp-cli rates pair-amount <base_code> <target_code> <amount>`** - Convert a specific amount between two currencies
- **`exchangerate-api-pp-cli rates enriched <base_code> <target_code>`** - Pair conversion with locale, flag, and currency display metadata (Business/Volume tier)
- **`exchangerate-api-pp-cli rates history <base_code> <year> <month> <day>`** - Historical exchange rates for a specific date (Pro/Business/Volume tier)

### Conversion & analysis (novel)

- **`exchangerate-api-pp-cli convert <amount> <from> <to[,to,...]>`** - Convert an amount; multi-target uses a single API call
- **`exchangerate-api-pp-cli convert-batch --from USD --to EUR --input -`** - Batch convert amounts piped on stdin
- **`exchangerate-api-pp-cli matrix <currencies>`** - N×N cross-rate matrix from a single `/latest` call
- **`exchangerate-api-pp-cli pair --as-of <timestamp>`** - Look up the rate from local snapshots at any past time
- **`exchangerate-api-pp-cli history-cache <base> <target>`** - Reconstruct historical rates from prior `latest` syncs

### Local data & monitoring (novel)

- **`exchangerate-api-pp-cli sync-rates --base USD`** - Pull latest rates and seed local snapshots
- **`exchangerate-api-pp-cli drift --base USD --since 24h`** - Diff latest snapshot vs prior and rank movers
- **`exchangerate-api-pp-cli watch check`** - Report watched pairs that crossed their thresholds
- **`exchangerate-api-pp-cli log show`** - Query recent `convert` calls from the local log
- **`exchangerate-api-pp-cli plan-check`** - Probe tier-gated endpoints to see what your key supports

### Utilities

- **`exchangerate-api-pp-cli doctor`** - Verify credentials and API reachability
- **`exchangerate-api-pp-cli open`** - Keyless `open.er-api.com` lookup (rate-limited, attribution required)
- **`exchangerate-api-pp-cli mcp serve`** - Run as an MCP server for AI agents

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
exchangerate-api-pp-cli codes

# JSON for scripting and agents
exchangerate-api-pp-cli codes --json

# Filter to specific fields
exchangerate-api-pp-cli codes --json --select supported_codes

# Dry run — show the request without sending
exchangerate-api-pp-cli rates pair USD EUR --dry-run

# Agent mode — JSON + compact + no prompts in one flag
exchangerate-api-pp-cli rates pair USD EUR --agent
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

## Recipes

End-to-end workflows agents and scripts commonly need. Each is ready to copy-paste.

### One-shot conversion in a script

```bash
exchangerate-api-pp-cli convert 1000 USD EUR --json --select results.0.result
```

Returns just the converted number for downstream piping.

### Daily sync + drift report (cron)

```bash
exchangerate-api-pp-cli sync-rates --base USD && \
  exchangerate-api-pp-cli drift --base USD --since 24h --top 10 --json
```

Captures today's rates and surfaces the largest movers vs the prior snapshot.

### Pre-deploy quota check

```bash
exchangerate-api-pp-cli quota burn --json --select projected_exhaustion,requests_remaining
```

Projects when the monthly quota will run out, given recent usage. Run before any deploy that will hammer the API.

### Cross-rate matrix for a portfolio

```bash
exchangerate-api-pp-cli matrix USD,EUR,GBP,JPY,CHF,CAD --base USD --csv
```

Full N×N matrix from one API call — N² rates, 1 quota tick. Pipes cleanly into spreadsheets.

### Watch a pair and alert when it moves

```bash
exchangerate-api-pp-cli watch add USD EUR --threshold 1.0
exchangerate-api-pp-cli watch check --json --select alerts
```

Stores a threshold per pair; subsequent `watch check` calls flag any pair that crossed.

### Time-travel a rate from local history

```bash
# After 'sync-rates --base USD' has populated rates_snapshots:
exchangerate-api-pp-cli rates pair USD EUR --as-of 30d --json
```

Reads the closest captured rate at-or-before the timestamp. Falls back to the live API for recent (≤24h) cutoffs when no snapshot exists.

## Health Check

```bash
exchangerate-api-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/exchangerate-api-pp-cli/config.yaml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EXCHANGERATE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `exchangerate-api-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EXCHANGERATE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`error-type: invalid-key`** — Run `exchangerate-api-pp-cli doctor` to confirm `EXCHANGERATE_API_KEY` is exported in the current shell.
- **`error-type: quota-reached`** — Hit your monthly limit. Use cached data via `pair --as-of`, `history-cache`, or wait until your refresh day (see `quota` output).
- **`error-type: plan-upgrade-required` on `enriched`/`history`** — Those endpoints require a paid tier. Use `plan-check` to see which endpoints your key supports.
- **`history` returns nothing** — Fall back to local snapshots: `history-cache <BASE> <TARGET>` if you've been syncing.
- **`open` rate-limited (HTTP 429)** — Open-access endpoint resets after 20 minutes. Switch to a keyed `latest <BASE>` call.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wesbos/currency-conversion-mcp**](https://github.com/wesbos/currency-conversion-mcp) — TypeScript (35 stars)
- [**TimothyYe/exchangerate**](https://github.com/TimothyYe/exchangerate) — Go (33 stars)
- [**markwragg/PowerShell-CurrencyConverter**](https://github.com/markwragg/PowerShell-CurrencyConverter) — PowerShell (6 stars)
- [**VersBinarii/poile**](https://github.com/VersBinarii/poile) — Rust (3 stars)
- [**cahthuranag/realtime-exchange-rate-mcp**](https://github.com/cahthuranag/realtime-exchange-rate-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
