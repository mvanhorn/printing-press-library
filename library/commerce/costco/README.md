# Costco CLI

**Your complete Costco receipt history as data — past the website's 2-year wall, in a local database the site never gives you.**

Costco's site shows receipts in 6-month chunks up to 2 years, but the backend serves more. This CLI pulls in-warehouse, gas, and online purchase history over any date range, probes how far back your account actually goes with history-depth, computes spend and item-price-history analytics on the fly, and builds a local SQLite archive (sync) for offline SQL and search.

Learn more at [Costco](https://ecom-api.costco.com).

Created by [@richie305](https://github.com/richie305) (David Richie).

## Install

The recommended path installs both the `costco-pp-cli` binary and the `pp-costco` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install costco
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install costco --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install costco --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install costco --agent claude-code
npx -y @mvanhorn/printing-press-library install costco --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/costco/cmd/costco-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/costco-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install costco --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-costco --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-costco --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install costco --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/costco-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `COSTCO_ID_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/costco/cmd/costco-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "costco": {
      "command": "costco-pp-mcp",
      "env": {
        "COSTCO_ID_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Costco's receipts API uses a short-lived bearer token (idToken) that your browser stores in localStorage after login. Capture it once from DevTools (localStorage.idToken and localStorage.clientID) and set it with auth set-token; the token expires in minutes, so doctor decodes its expiry and tells you when to refresh. No cookies or password are stored.

## Quick Start

```bash
# Check token presence and JWT expiry before anything else
costco-pp-cli doctor

# Find how far back the API actually serves your receipts
costco-pp-cli history-depth

# Pull in-warehouse receipts for a date range
costco-pp-cli receipts --since 2024-01-01

# Build the local SQLite archive for offline search and analytics
costco-pp-cli sync

# Roll up spend the website never computes
costco-pp-cli spend --by month

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Reach past the UI
- **`history-depth`** — Discover how far back your Costco receipts actually go, past the website's 2-year UI cap.

  _Reach for this to answer 'can I get receipts older than two years' — it probes the live API boundary instead of trusting the UI._

  ```bash
  costco-pp-cli history-depth --json
  ```

### Local state that compounds
- **`spend`** — Roll up your spend by month, warehouse, or department over a date range.

  _Use when an agent needs spend totals or trends the website never computes._

  ```bash
  costco-pp-cli spend --by month --json
  ```
- **`savings`** — Total the instant savings and coupons you captured over a date range.

  _Use to quantify how much Costco deals actually saved you._

  ```bash
  costco-pp-cli savings --since 2024-01-01 --json
  ```
- **`returns-window`** — Flag recently purchased items still inside a return window you set.

  _Reach for this to find what you can still return before a deadline._

  ```bash
  costco-pp-cli returns-window --days 90 --json
  ```

### Spend insight
- **`item-history`** — Track one item's unit price across every receipt over time.

  _Pick this to see whether a recurring buy has crept up in price._

  ```bash
  costco-pp-cli item-history "rotisserie chicken" --json
  ```

## Recipes


### Find your true history floor

```bash
costco-pp-cli history-depth --json
```

Steps startDate backward and reports the earliest receipt the API will serve for your account.

### Export receipts to a spreadsheet

```bash
costco-pp-cli receipts --since 2024-01-01 --csv
```

Flat line-item CSV ready for budgeting tools.

### Narrow a verbose receipt payload for an agent

```bash
costco-pp-cli receipts --since 2025-01-01 --agent --select transactionDate,warehouseName,total
```

Receipts list output is a flat array; --select trims each row to just the fields an agent needs.

### See spend by warehouse

```bash
costco-pp-cli spend --by warehouse --json
```

Aggregates local receipts into per-warehouse spend totals.

### What can I still return?

```bash
costco-pp-cli returns-window --days 90 --json
```

Lists recently bought items still inside a 90-day window from their receipt date.

## Usage

Run `costco-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `COSTCO_CONFIG_DIR`, `COSTCO_DATA_DIR`, `COSTCO_STATE_DIR`, or `COSTCO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `COSTCO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export COSTCO_HOME=/srv/costco
costco-pp-cli doctor
```

Under `COSTCO_HOME=/srv/costco`, the four dirs resolve to `/srv/costco/config`, `/srv/costco/data`, `/srv/costco/state`, and `/srv/costco/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "costco": {
      "command": "costco-pp-mcp",
      "env": {
        "COSTCO_HOME": "/srv/costco"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `COSTCO_DATA_DIR` overrides an explicit `--home` for that kind. Use `COSTCO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `COSTCO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `costco-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### Receipt data

- **`costco-pp-cli receipts`** — List in-warehouse, gas, and carwash receipts for a date range. Supports `--since`, `--until`, `--years`, `--type` (warehouse/gas/carwash), `--csv`.
- **`costco-pp-cli receipt get <barcode>`** — Show full line-item detail for one receipt by transaction barcode.
- **`costco-pp-cli orders`** — List online costco.com orders (requires `--warehouse`).
- **`costco-pp-cli counts`** — Summarize receipt counts and spend by channel.

### Analytics (novel)

- **`costco-pp-cli history-depth`** — Discover how far back your receipts actually go.
- **`costco-pp-cli spend`** — Roll up spend by month, warehouse, or department (`--by`).
- **`costco-pp-cli item-history <query>`** — Track one item's unit price across receipts over time.
- **`costco-pp-cli savings`** — Total instant savings and coupons captured.
- **`costco-pp-cli returns-window`** — Flag items still inside a return window (`--days`).

### Local archive

- **`costco-pp-cli sync`** — Fetch receipts into a local SQLite archive (idempotent).
- **`costco-pp-cli search <term>`** — Search synced line items by description or UPC.
- **`costco-pp-cli sql <query>`** — Run a read-only SQL SELECT against the archive. Tables: `receipts`, `items`.
- **`costco-pp-cli export`** — Export receipts to JSONL or CSV (`--format`, `--output`).

### Utilities

- **`costco-pp-cli doctor`** — Check CLI health, token expiry, and path resolution.
- **`costco-pp-cli auth set-token`** — Store a Costco idToken for API calls.
- **`costco-pp-cli raw`** — Raw GraphQL passthrough (advanced).
- **`costco-pp-cli which <query>`** — Find the command for a capability in natural language.
- **`costco-pp-cli profile`** — Save and reuse named sets of flags.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
costco-pp-cli receipts --years 1

# JSON for scripting and agents
costco-pp-cli receipts --years 1 --json

# CSV for spreadsheets
costco-pp-cli receipts --years 1 --csv

# Filter to specific fields
costco-pp-cli receipts --years 1 --json --select transactionDate,warehouseName,total

# Dry run — show the request without sending
costco-pp-cli receipts --years 1 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
costco-pp-cli receipts --years 1 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
costco-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `costco-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/costco-pp-cli/config.toml`; `--home`, `COSTCO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `COSTCO_ID_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `costco-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `costco-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $COSTCO_ID_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Invalid credentials** — Token expired (idToken lives ~15 min). Re-copy localStorage.idToken from DevTools and run 'auth set-token'.
- **Empty receipts for a known period** — Confirm the date range with --since/--until; the API returns the full account history, so widen the range with history-depth.
- **doctor says token expired** — Refresh the idToken from a logged-in costco.com tab and re-run auth set-token.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**TCRDD**](https://github.com/TechStud/TCRDD) — JavaScript
- [**Costco_Scraping**](https://github.com/dheerajW125/Costco_Scraping) — Python
- [**costco-importer**](https://github.com/garyhtou/costco-importer) — JavaScript
- [**CostcoWrapped**](https://github.com/GonzaloZiadi/CostcoWrapped) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
