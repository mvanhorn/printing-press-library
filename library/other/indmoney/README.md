# INDmoney CLI

**Scrape your INDmoney US stock portfolio, holdings, and full trade history from the terminal.**

Pull your complete portfolio summary, per-stock P&L, and every buy/sell order from INDmoney's private API. Sync to local SQLite for offline analysis, SQL queries, and historical tracking — no spreadsheet exports needed.

## Install

The recommended path installs both the `indmoney-pp-cli` binary and the `pp-indmoney` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install indmoney
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install indmoney --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install indmoney --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install indmoney --agent claude-code
npx -y @mvanhorn/printing-press-library install indmoney --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/indmoney/cmd/indmoney-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/indmoney-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install indmoney --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-indmoney --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-indmoney --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install indmoney --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/indmoney-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `INDMONEY_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/indmoney/cmd/indmoney-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "indmoney": {
      "command": "indmoney-pp-mcp",
      "env": {
        "INDMONEY_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

INDmoney authenticates via a JWT stored in the 'token' browser cookie. Extract the token cookie value from your logged-in browser session (DevTools > Application > Cookies > token), then run `auth set-token <token>` or set the INDMONEY_TOKEN env var. The token expires every ~9 hours; extract and set a fresh one to refresh.

## Quick Start

```bash
# Verify the CLI is ready before authenticating
indmoney-pp-cli doctor --dry-run

# Set your INDmoney token (extract from browser cookies)
indmoney-pp-cli auth set-token <token>

# See your total portfolio value and P&L
indmoney-pp-cli portfolio summary

# Export your full trade history as JSON
indmoney-pp-cli orders list --json

# Save everything to local SQLite for offline analysis
indmoney-pp-cli sync

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Portfolio visibility
- **`portfolio summary`** — See your total invested value, current value, overall P&L, and day change across all US stock holdings.

  _Get instant portfolio overview without opening the INDmoney app._

  ```bash
  indmoney-pp-cli portfolio summary --json
  ```
- **`portfolio holdings`** — List every stock you hold with quantity, avg price, current value, unrealised gains, and day P&L.

  _Filter and sort holdings by any field without the INDmoney UI's fixed views._

  ```bash
  indmoney-pp-cli portfolio holdings --select name,quantity,overall_pnl --json
  ```

### Trade history
- **`orders list`** — Every buy and sell order you have placed, with transaction type, quantity, avg price, and status.

  _Export your full trade history for tax reporting or analysis in one command._

  ```bash
  indmoney-pp-cli orders list --limit 100 --json
  ```

### Local state that compounds
- **`sync`** — Download all portfolio data and order history into a local SQLite database for offline analysis.

  _Build a local historical record of your portfolio snapshots and query it with SQL._

  ```bash
  indmoney-pp-cli sync --db ~/my-portfolio.db
  ```

### Market context
- **`indices list`** — Live prices for Nasdaq, S&P 500, Dow Jones, and other US indices with market status.

  _Quick market context before making trading decisions._

  ```bash
  indmoney-pp-cli indices list
  ```

## Recipes

### Portfolio overview

```bash
indmoney-pp-cli portfolio summary
```

Get total invested value, current value, and overall P&L in one call.

### Export all holdings as JSON

```bash
indmoney-pp-cli portfolio holdings --json
```

Every stock with quantity, avg price, current value, and P&L for piping into jq or a spreadsheet.

### Full trade history

```bash
indmoney-pp-cli orders list --limit 200 --json --select name,transaction_type,qty,avg_price,date
```

Every buy and sell with key fields only, ready for tax reporting.

### Sync and search locally

```bash
indmoney-pp-cli sync && indmoney-pp-cli search "GOOGL" --data-source local
```

Sync to SQLite, then search your portfolio data offline.

## Usage

Run `indmoney-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `INDMONEY_CONFIG_DIR`, `INDMONEY_DATA_DIR`, `INDMONEY_STATE_DIR`, or `INDMONEY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `INDMONEY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export INDMONEY_HOME=/srv/indmoney
indmoney-pp-cli doctor
```

Under `INDMONEY_HOME=/srv/indmoney`, the four dirs resolve to `/srv/indmoney/config`, `/srv/indmoney/data`, `/srv/indmoney/state`, and `/srv/indmoney/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "indmoney": {
      "command": "indmoney-pp-mcp",
      "env": {
        "INDMONEY_HOME": "/srv/indmoney"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `INDMONEY_DATA_DIR` overrides an explicit `--home` for that kind. Use `INDMONEY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `INDMONEY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `indmoney-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### indices

US market indices with live prices

- **`indmoney-pp-cli indices`** - Get live prices for major US market indices (Nasdaq, S&P 500, Dow Jones, etc.) and market status.

### orders

US stock order history (all buys and sells)

- **`indmoney-pp-cli orders`** - Get complete order history with transaction type, quantity, avg price, and status.

### portfolio

US stock portfolio summary with holdings and P&L

- **`indmoney-pp-cli portfolio`** - Get portfolio summary including current value, invested value, overall P&L, and per-stock holdings breakdown.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`indmoney-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`indmoney-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`indmoney-pp-cli learnings list`** - Inspect taught rows
- **`indmoney-pp-cli learnings forget <query>`** - Undo a teach
- **`indmoney-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`indmoney-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`indmoney-pp-cli teach-pattern`** - Install a query/resource template up front
- **`indmoney-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `INDMONEY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `indmoney-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
indmoney-pp-cli indices --segment example-value --mkt-status-required true --require-live-feed true

# JSON for scripting and agents
indmoney-pp-cli indices --segment example-value --mkt-status-required true --require-live-feed true --json

# Filter to specific fields
indmoney-pp-cli indices --segment example-value --mkt-status-required true --require-live-feed true --json --select id,name,status

# Dry run — show the request without sending
indmoney-pp-cli indices --segment example-value --mkt-status-required true --require-live-feed true --dry-run

# Agent mode — JSON + compact + no prompts in one flag
indmoney-pp-cli indices --segment example-value --mkt-status-required true --require-live-feed true --agent
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
indmoney-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `indmoney-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `INDMONEY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INDMONEY_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `indmoney-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `indmoney-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INDMONEY_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 400: Device Type doesn't exist** — The platform header is missing or token is malformed. Re-extract the token cookie and run auth set-token again.
- **HTTP 512: Auth returned with status false** — Your token expired (9h lifetime). Extract a fresh token cookie from the browser and run auth set-token again.
- **HTTP 401 or empty results** — Token not set. Run auth set-token <token> or set INDMONEY_TOKEN env var with the token cookie value.
