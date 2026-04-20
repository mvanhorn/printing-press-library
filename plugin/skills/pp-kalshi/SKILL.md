---
name: pp-kalshi
description: "Use this skill whenever the user asks about prediction markets, Kalshi, event contracts, market odds, binary markets, or trading on outcomes like elections, economic indicators, weather, or sports. Also use for portfolio P&L, win rates, exposure analysis, or settlement calendars on prediction market positions. Kalshi CLI covering all 91 API endpoints with RSA-PSS auth, 20,000+ market local sync, and 9 transcendence commands for portfolio analytics and market intelligence. Triggers on phrasings like 'what are the odds on the election', 'show my Kalshi positions', 'what's my win rate on prediction markets', 'which markets are settling this week', 'find the biggest movers on Kalshi today'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["kalshi-pp-cli"],"env":["KALSHI_API_KEY","KALSHI_PRIVATE_KEY_PATH"]},"primaryEnv":"KALSHI_API_KEY","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/payments/kalshi/cmd/kalshi-pp-cli@latest","bins":["kalshi-pp-cli"],"label":"Install via go install"}]}}'
---

# Kalshi — Printing Press CLI

Trade prediction markets, track portfolios, and analyze odds on Kalshi from the command line. Covers all 91 Kalshi API endpoints with RSA-PSS signature authentication, a SQLite data layer with full-text search and cursor-based sync across 20,000+ markets, and 9 transcendence commands that require local state no other Kalshi client provides.

## When to Use This CLI

Reach for this when a user wants to place or analyze trades on Kalshi event contracts, track their position P&L, review settlement calendars, or investigate market activity and cross-market correlations. Use the demo sandbox (`KALSHI_ENV=demo`) when exploring without risking real money.

Don't reach for this when the user wants a read-only market data summary from a random website — Kalshi's API requires an API key and private-key file regardless. Also not for non-Kalshi prediction markets like Polymarket or Manifold; this is Kalshi-specific.

## Unique Capabilities

These aren't available in any other Kalshi tool.

### Portfolio analytics that compound on local state

- **`portfolio attribution [--by category|series|event] [--period 30d]`** — P&L broken down by market category, series, or event over any time window.

  _Turns "did I make money?" into "which categories was I profitable in, and which drained me?" — the actionable version._

- **`portfolio winrate [--by category]`** — Win/loss ratio, expected value, and ROI across all settled positions.

- **`portfolio exposure`** — Concentration risk by category. Surfaces when you're over-weighted in a single event or series.

- **`portfolio calendar`** — Upcoming settlements with your positions, expected payouts, and category breakdown. The only way to know what's about to resolve without clicking through 50 market pages.

- **`portfolio stale [--days N]`** — Positions approaching expiry where you haven't touched them recently. Prompts action before you lose the chance to manage.

### Market intelligence that requires synced history

- **`markets movers`** — Biggest price swings since your last sync. Uses the local history to compute deltas that the API doesn't directly expose.

  _Live opportunity detection. Daily sync + movers becomes a morning scan._

- **`markets heatmap`** — Category-level activity visualization (volume, open interest, average price). Spots hot/cold sectors at a glance.

- **`markets correlate <ticker1> <ticker2>`** — Compares price histories of two markets to surface correlated outcomes. Useful for identifying arbitrage pairs or hedging.

### Operational resilience

- **SQLite sync across 20,000+ markets** with cursor-based pagination — offline analysis works even when Kalshi is rate-limiting.

- **Demo sandbox support** via `KALSHI_ENV=demo` — every command works against `demo-api.kalshi.co`, safe for dev/testing.

- **Batch order operations** (`portfolio batch-create-orders`, `batch-cancel-orders`) — atomic multi-leg trade construction with rollback on partial failure.

## Command Reference

Trading and portfolio:

- `kalshi-pp-cli portfolio` — Current positions
- `kalshi-pp-cli portfolio create-order --ticker <t> --action buy|sell --side yes|no --count N --yes-price PRICE` — Place order (add `--dry-run` to preview)
- `kalshi-pp-cli portfolio amend-order <order_id>` / `cancel-order <order_id>` / `decrease-order <order_id>` — Modify orders
- `kalshi-pp-cli portfolio batch-create-orders` / `batch-cancel-orders` — Multi-order ops

Market discovery:

- `kalshi-pp-cli markets` — Browse markets (filter by `--status`, `--series-ticker`)
- `kalshi-pp-cli markets get-market <ticker>` — Single market detail including orderbook
- `kalshi-pp-cli events [--status open]` — Events (market groupings)
- `kalshi-pp-cli series get <series_ticker>` — Series metadata
- `kalshi-pp-cli search "<query>"` — Full-text search across synced markets

Exchange + live data:

- `kalshi-pp-cli exchange` — Exchange status
- `kalshi-pp-cli live-data` — Real-time price/orderbook feed
- `kalshi-pp-cli historical` — Historical candlesticks
- `kalshi-pp-cli multivariate-event-collections` — Multi-outcome event groupings

Local data plumbing:

- `kalshi-pp-cli sync` — Sync all resources to local SQLite
- `kalshi-pp-cli export <resource> [--format jsonl]` / `import <resource>` — Dump/restore
- `kalshi-pp-cli archive` — Archive cold data

Auth + health:

- `kalshi-pp-cli auth` / `api-keys` — Manage API keys
- `kalshi-pp-cli account` / `get-balance` — Account info
- `kalshi-pp-cli doctor` — Verify setup

Unique commands (see Unique Capabilities above): `portfolio attribution/winrate/exposure/calendar/stale`, `markets movers/heatmap/correlate`.

## Recipes

### Morning portfolio scan

```bash
kalshi-pp-cli sync
kalshi-pp-cli portfolio calendar --agent        # what settles this week
kalshi-pp-cli portfolio stale --days 3 --agent  # stale positions
kalshi-pp-cli markets movers --agent            # biggest overnight price swings
```

Sync pulls the latest state, `calendar` surfaces upcoming payouts, `stale` prompts action on positions approaching expiry, `movers` identifies opportunities from overnight activity.

### Preview then place a Yes order

```bash
# Preview — dry-run shows the exact request without placing
kalshi-pp-cli portfolio create-order \
  --ticker INXD-26APR25-B5525 --action buy --side yes \
  --count 10 --yes-price 65 --dry-run

# Place it
kalshi-pp-cli portfolio create-order \
  --ticker INXD-26APR25-B5525 --action buy --side yes \
  --count 10 --yes-price 65 --yes
```

`--dry-run` returns the JSON request body without hitting the exchange. Flip to `--yes` (or omit the prompt flags) to actually execute.

### Win-rate analysis over 30 days

```bash
kalshi-pp-cli portfolio attribution --by category --period 30d --agent
kalshi-pp-cli portfolio winrate --by category --agent
```

Attribution shows WHERE P&L came from; winrate shows HOW RELIABLY — paired, they tell you which market categories to keep trading and which to avoid.

### Safe dev on sandbox

```bash
export KALSHI_ENV=demo
kalshi-pp-cli doctor  # verifies against demo-api.kalshi.co
kalshi-pp-cli portfolio create-order --ticker <demo-ticker> --action buy --side yes --count 10 --yes-price 50
```

Every command works against the sandbox. Unset `KALSHI_ENV` (or set to empty) to return to production.

## Auth Setup

Kalshi uses **RSA-PSS signature auth** — API key UUID + private key PEM file, three signed headers per request (`KALSHI-ACCESS-KEY`, `KALSHI-ACCESS-SIGNATURE`, `KALSHI-ACCESS-TIMESTAMP`). Get credentials at [kalshi.com/api](https://kalshi.com/api).

```bash
export KALSHI_API_KEY="your-api-key-uuid"
export KALSHI_PRIVATE_KEY_PATH="~/.kalshi/private_key.pem"
kalshi-pp-cli doctor  # verify
```

Alternatives: `KALSHI_PRIVATE_KEY` (inline PEM), `KALSHI_BASE_URL` (override), `KALSHI_ENV=demo` (sandbox).

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes` — structured output, no prompts. Also supports `--select <fields>` for cherry-picking, `--dry-run` to preview requests, and `--no-cache` to bypass the 5-minute GET cache.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Resource not found |
| 4 | Authentication required (missing or invalid key/PEM) |
| 5 | API error |
| 7 | Rate limited |
| 10 | Config error |

## Installation

### CLI

1. `go install github.com/mvanhorn/printing-press-library/library/payments/kalshi/cmd/kalshi-pp-cli@latest`
2. Set `KALSHI_API_KEY` and `KALSHI_PRIVATE_KEY_PATH` (see Auth Setup)
3. `kalshi-pp-cli doctor` to verify

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/kalshi/cmd/kalshi-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/payments/kalshi/cmd/kalshi-pp-mcp@main
claude mcp add -e KALSHI_API_KEY=<key> -e KALSHI_PRIVATE_KEY_PATH=<path> kalshi-pp-mcp -- kalshi-pp-mcp
claude mcp list
```

89 MCP tools exposed for agent integration.

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `kalshi-pp-cli --help`
2. **`install`** → CLI install; **`install mcp`** → MCP install
3. **Anything else** → check `which kalshi-pp-cli` (offer to install if missing), verify `KALSHI_API_KEY` is set (prompt for setup if not), match the user's intent to a command, run with `--agent` for structured output. Drill into subcommand help if ambiguous.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
kalshi-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
kalshi-pp-cli --profile <name> <command>

# List / inspect / remove
kalshi-pp-cli profile list
kalshi-pp-cli profile show <name>
kalshi-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
kalshi-pp-cli <command> --deliver file:/path/to/out.json
kalshi-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
kalshi-pp-cli feedback "what surprised you or tripped you up"
kalshi-pp-cli feedback list         # show local entries
kalshi-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.kalshi-pp-cli/feedback.jsonl` as JSON lines. When `KALSHI_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `KALSHI_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

