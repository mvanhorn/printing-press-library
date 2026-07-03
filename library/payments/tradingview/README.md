# TradingView CLI

**Resolve any ticker and see its price in USD and EUR in one command — stocks, crypto, and forex, no API key.**

TradingView CLI turns a symbol into an answer: search resolves a query to a fully-qualified EXCHANGE:TICKER, quote returns the last price in its native currency plus USD and EUR, and convert does fiat conversion using TradingView's own forex rates. Terminal-first, agent-native --json, no data-API subscription required for the public endpoints. A local watchlist tracks symbols and snapshots their USD/EUR prices to SQLite for offline querying and drift.

## Install

The recommended path installs both the `tradingview-pp-cli` binary and the `pp-tradingview` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tradingview
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tradingview --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tradingview --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tradingview --agent claude-code
npx -y @mvanhorn/printing-press-library install tradingview --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/tradingview/cmd/tradingview-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tradingview-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tradingview --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tradingview --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tradingview --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tradingview --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tradingview-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/tradingview/cmd/tradingview-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tradingview": {
      "command": "tradingview-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the TradingView endpoints are reachable
tradingview-pp-cli doctor --dry-run

# resolve a query to a fully-qualified ticker like NASDAQ:AAPL (shows candidates)
tradingview-pp-cli search AAPL

# last price in native currency, USD, and EUR
tradingview-pp-cli quote NASDAQ:AAPL

# convert an amount using TradingView forex rates
tradingview-pp-cli convert 100 USD EUR

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Multi-currency pricing
- **`quote`** — See any ticker's last price in its native currency, USD, and EUR in a single command.

  _Reach for this when an agent needs a symbol's value normalized to USD or EUR without wiring up a separate FX API._

  ```bash
  tradingview-pp-cli quote NASDAQ:AAPL --agent --select symbol,price_usd,price_eur
  ```
- **`convert`** — Convert an amount between currencies using TradingView's own live forex rates.

  _Use this to turn any USD figure into EUR (or between other majors) from the same source that priced the instrument._

  ```bash
  tradingview-pp-cli convert 100 USD EUR --agent
  ```

### Local state that compounds
- **`watchlist`** — Track a list of symbols locally and snapshot their USD/EUR quotes to SQLite for offline querying.

  _Use this to keep a personal set of symbols and their USD/EUR prices without re-resolving them each call._

  ```bash
  tradingview-pp-cli watchlist add AAPL BINANCE:BTCUSDT
  ```
- **`watchlist changes`** — Show how each watched symbol moved in USD between its two most recent snapshots.

  _Reach for this to see what moved since your last sync without diffing raw API responses yourself._

  ```bash
  tradingview-pp-cli watchlist changes --agent
  ```

## Recipes

### Price in euros

```bash
tradingview-pp-cli quote NASDAQ:MSFT --agent --select symbol,price_eur
```

Return only the EUR-converted price for scripting.

### Find then quote

```bash
tradingview-pp-cli search bitcoin --type crypto
```

Resolve a name to a fully-qualified crypto ticker, then quote it.

### Currency conversion

```bash
tradingview-pp-cli convert 250 USD EUR --agent
```

Convert 250 USD to EUR using TradingView's EURUSD forex rate.

### Track a watchlist

```bash
tradingview-pp-cli watchlist add AAPL BINANCE:BTCUSDT
```

Add symbols, then 'watchlist sync' and 'watchlist quotes' for offline USD/EUR prices.

## Usage

Run `tradingview-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TRADINGVIEW_CONFIG_DIR`, `TRADINGVIEW_DATA_DIR`, `TRADINGVIEW_STATE_DIR`, or `TRADINGVIEW_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TRADINGVIEW_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TRADINGVIEW_HOME=/srv/tradingview
tradingview-pp-cli doctor
```

Under `TRADINGVIEW_HOME=/srv/tradingview`, the four dirs resolve to `/srv/tradingview/config`, `/srv/tradingview/data`, `/srv/tradingview/state`, and `/srv/tradingview/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "tradingview": {
      "command": "tradingview-pp-mcp",
      "env": {
        "TRADINGVIEW_HOME": "/srv/tradingview"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TRADINGVIEW_DATA_DIR` overrides an explicit `--home` for that kind. Use `TRADINGVIEW_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TRADINGVIEW_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `tradingview-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### market

Fetch prices and quotes

- **`tradingview-pp-cli market`** - Get last price and currency for a fully-qualified EXCHANGE:TICKER

### symbols

Search and resolve TradingView symbols

- **`tradingview-pp-cli symbols`** - Search symbols by text (returns exchange, type, currency)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tradingview-pp-cli symbols --text AAPL

# JSON for scripting and agents
tradingview-pp-cli symbols --text AAPL --json

# Filter to specific fields
tradingview-pp-cli symbols --text AAPL --json --select id,name,status

# Dry run — show the request without sending
tradingview-pp-cli symbols --text AAPL --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tradingview-pp-cli symbols --text AAPL --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
tradingview-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `tradingview-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/tradingview-pp-cli/config.toml`; `--home`, `TRADINGVIEW_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **quote returns nothing for a bare ticker** — run 'tradingview-pp-cli search <ticker>' first and use the fully-qualified EXCHANGE:TICKER
- **crypto price shows currency USDT not USD** — use a USD pair (e.g. COINBASE:ETHUSD) or read the price_usd field, which treats USDT as USD-pegged

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**TradingView-Screener**](https://github.com/shner-elmo/TradingView-Screener) — Python
- [**tvscreener**](https://github.com/deepentropy/tvscreener) — Python
- [**python-tradingview-ta**](https://github.com/AnalyzerREST/python-tradingview-ta) — Python
- [**tradingview-screener-ts**](https://github.com/jmargieh/tradingview-screener) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
