# Polymarket CLI

Every Polymarket feature the official Rust CLI ships, plus six novel commands no Polymarket tool offers: resolution radar, reward-yield ranker, position drift, price diff, research bundle, batch redeem. Mirrors Gamma + CLOB + Data API surfaces with agent-native JSON, local SQLite + FTS5, and an MCP server.

Printed by [@mcsyauqi](https://github.com/mcsyauqi) (Ahmad Thariq Syauqi).

## Install

The recommended path installs both the `polymarket-pp-cli` binary and the `pp-polymarket` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install polymarket
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install polymarket --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install polymarket --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install polymarket --agent claude-code
npx -y @mvanhorn/printing-press-library install polymarket --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/polymarket-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-polymarket --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-polymarket --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-polymarket skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-polymarket. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
polymarket-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/polymarket-current).
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
    "polymarket": {
      "command": "polymarket-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to  in Chrome, then:

```bash
polymarket-pp-cli auth login --chrome
```

Requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
polymarket-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
polymarket-pp-cli api_keys list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Portfolio that compounds locally
- **`radar resolutions`** — List every market resolving in the next N days, ranked by your position value when a wallet is provided. Stops you missing redemption deadlines.

  _When the agent is asked 'do I have any winnings to claim or positions about to settle', this is the single-command answer._

  ```bash
  polymarket-pp-cli radar resolutions --within 168h --wallet 0xYOUR --min-value 10 --agent
  ```
- **`rewards rank`** — Rank reward-eligible markets by expected daily payout per dollar of capital at risk, given a target spread.

  _When an agent is asked 'where should a market-maker park 10k for the next week', this is the answer with the math shown._

  ```bash
  polymarket-pp-cli rewards rank --capital 10000 --days 7 --min-spread 0.02 --agent
  ```
- **`portfolio drift`** — For each position: entry price, current mid, min/max over a window, unrealized P&L drift, and a thawed/frozen flag based on current spread + book depth.

  _When the agent must answer 'which of my positions are easy to close right now and which are stuck', this returns the full set with one call._

  ```bash
  polymarket-pp-cli portfolio drift --wallet 0xYOUR --since 168h --agent
  ```

### Time-window deltas
- **`diff prices`** — Find tokens whose implied probability moved by more than a threshold over a time window. Supports a slug watchlist and arbitrary windows down to a few minutes.

  _When the agent is asked 'what moved on Polymarket in the last hour', it gets a deterministic list rather than scrolling activity._

  ```bash
  polymarket-pp-cli diff prices --since 24h --min-move 0.05 --watch 2026-election,fed-may --agent
  ```

### Reproducible research
- **`bundle export`** — Export every market matching a tag, event, or id list as a portable zip: markets + events + tags + full price history + book snapshot + holders. bundle import rehydrates into a fresh SQLite.

  _When an agent needs to defend a claim with a snapshot ('on May 23 the market priced X at 62 percent'), this produces the archive proving it._

  ```bash
  polymarket-pp-cli bundle export --tag 2026-election --out ./election-snapshot.zip
  ```

### Trading

v0.2 wired live on-chain broadcast paths via go-ethereum + Polygon RPC (set `POLYGON_RPC_URL` in env). Status grid:

| Command | Live broadcast | Notes |
|---------|----------------|-------|
| `auth derive` | ✅ Yes | EIP-712 ClobAuth, returns L2 HMAC creds |
| `approve set --broadcast` | ✅ Yes | 6-tx idempotent (USDC.e ×3 + CTF setApprovalForAll ×3), skips already-set |
| `orders list` | ✅ Yes | L2 HMAC GET /data/orders |
| `ctf redeem --broadcast` | ✅ Yes | redeemPositions on CTF or NegRiskAdapter |
| `orders create --broadcast` | ⚠️ Code-complete, platform-gated | EIP-712 sig verifies locally + HMAC accepted on reads, but CLOB POST /order returns `{"error":"Invalid order payload"}` (likely platform onboarding gate, not code bug). Pipe the signed JSON through the official Polymarket Rust CLI as fallback for actual broadcast. |
| `ctf split / merge --broadcast` | 📋 Stub (v0.3) | Same go-ethereum pattern as redeem |

- **`redeem all`** — Discover every position in resolved markets and report the redeemable set with totals. Default `--dry-run` produces the exact call set; `--broadcast` wires into live `ctf redeem` per market.

  ```bash
  polymarket-pp-cli redeem all --dry-run --min-value 1 --agent
  ```

- **`approve set --broadcast`** — Idempotently set the 6 ERC-20 + ERC-1155 approvals required to trade. Reads current state from chain, broadcasts only what's missing. ~$0.01 total gas on Polygon mainnet.

  ```bash
  polymarket-pp-cli approve status --agent                          # read-only check
  polymarket-pp-cli approve set --broadcast --yes --wait --agent    # live broadcast
  ```

- **`ctf redeem --broadcast`** — Call `redeemPositions(USDC, parent=0, conditionId, indexSets)` on ConditionalTokens (or NegRiskAdapter with `--neg-risk`). Default `--index-sets 1,2` tries both YES/NO; the contract auto-skips outcomes the caller holds zero of.

  ```bash
  polymarket-pp-cli ctf redeem 0xCONDITION_ID --broadcast --yes --wait --agent
  ```

## Usage

Run `polymarket-pp-cli --help` for the full command reference and flag list.

## Commands

### api_keys

Manage L2 API credentials.

- **`polymarket-pp-cli api_keys delete`** - Delete the L2 API key currently in use
- **`polymarket-pp-cli api_keys list`** - List API keys for the authenticated wallet

### balance

Read on-chain USDC + outcome-token balances.

- **`polymarket-pp-cli balance`** - Get balance and allowance for the authenticated wallet

### clob

CLOB order book reads, market metadata, server health, and reward configs.

- **`polymarket-pp-cli clob book`** - Get the full order book for an outcome token
- **`polymarket-pp-cli clob books`** - Batch order book fetch
- **`polymarket-pp-cli clob market`** - Get CLOB metadata for a single market
- **`polymarket-pp-cli clob markets`** - List CLOB markets
- **`polymarket-pp-cli clob midpoint`** - Get the midpoint price
- **`polymarket-pp-cli clob neg-risk`** - Get neg-risk markets
- **`polymarket-pp-cli clob ok`** - CLOB API liveness probe
- **`polymarket-pp-cli clob price`** - Get the best bid or ask price
- **`polymarket-pp-cli clob price-history`** - Historical price series for an outcome token
- **`polymarket-pp-cli clob spread`** - Get the current bid-ask spread
- **`polymarket-pp-cli clob time`** - Get the CLOB server time

### comments

Public comments on markets and events.

- **`polymarket-pp-cli comments`** - List comments

### data

Polymarket Data API for positions with valuation, portfolio value, activity history, and market holders.

- **`polymarket-pp-cli data activity`** - User activity feed: trades, splits, merges, redeems, rewards, conversions
- **`polymarket-pp-cli data holders`** - Get holder distribution for a market
- **`polymarket-pp-cli data positions`** - List positions for a wallet address with current price + value + P&L
- **`polymarket-pp-cli data value`** - Get the total portfolio value for a wallet

### events

Browse and inspect events (groups of related markets) via the Gamma API.

- **`polymarket-pp-cli events get`** - Fetch a single event by id or slug
- **`polymarket-pp-cli events list`** - List events with filtering
- **`polymarket-pp-cli events tags`** - List the tags attached to an event

### markets

Discover, search, and inspect prediction markets via the Gamma API. Markets are individual yes/no contracts; events group related markets.

- **`polymarket-pp-cli markets get`** - Fetch a single market by id or slug.
- **`polymarket-pp-cli markets list`** - List prediction markets with filtering. Active, recently-resolved, by tag, by event, etc.
- **`polymarket-pp-cli markets search`** - Search markets by text.

### orders

Place, cancel, list, and inspect orders on the CLOB. Requires L1 wallet PK + L2 HMAC credentials.

- **`polymarket-pp-cli orders cancel`** - Cancel a single order by id
- **`polymarket-pp-cli orders cancel-all`** - Cancel every open order for the authenticated user
- **`polymarket-pp-cli orders cancel-batch`** - Cancel multiple orders by id list
- **`polymarket-pp-cli orders cancel-market`** - Cancel all open orders for a single market
- **`polymarket-pp-cli orders get`** - Get a single order by id
- **`polymarket-pp-cli orders list`** - List the authenticated user's open orders

### rewards

Liquidity-rewards data: configs, payouts, earnings, and order scoring.

- **`polymarket-pp-cli rewards current`** - Get current live reward eligibility
- **`polymarket-pp-cli rewards earnings`** - Get the authenticated user's daily reward earnings
- **`polymarket-pp-cli rewards earnings-markets`** - Get reward earnings per market
- **`polymarket-pp-cli rewards list`** - List all currently reward-eligible markets
- **`polymarket-pp-cli rewards market-reward`** - Get the reward config for a single market
- **`polymarket-pp-cli rewards order-scoring`** - Get the reward score for a single open order
- **`polymarket-pp-cli rewards orders-scoring`** - Get reward scores for multiple open orders
- **`polymarket-pp-cli rewards percentages`** - Get current reward distribution percentages

### series

Recurring event series.

- **`polymarket-pp-cli series get`** - Fetch a single series by id
- **`polymarket-pp-cli series list`** - List event series

### tags

Browse Polymarket category tags.

- **`polymarket-pp-cli tags`** - List all tags

### trades

Read trades the authenticated user has made on the CLOB.

- **`polymarket-pp-cli trades`** - List the authenticated user's trade history


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
polymarket-pp-cli api_keys list

# JSON for scripting and agents
polymarket-pp-cli api_keys list --json

# Filter to specific fields
polymarket-pp-cli api_keys list --json --select id,name,status

# Dry run — show the request without sending
polymarket-pp-cli api_keys list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
polymarket-pp-cli api_keys list --agent
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
polymarket-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/polymarket-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `POLYMARKET_PRIVATE_KEY` | per_call | Yes | Set to your API credential. |
| `POLYMARKET_API_KEY` | per_call | Yes | Set to your API credential. |
| `POLYMARKET_API_SECRET` | per_call | Yes | Set to your API credential. |
| `POLYMARKET_API_PASSPHRASE` | per_call | Yes | Set to your API credential. |
| `POLYMARKET_FUNDER` | per_call | Yes | Set to your API credential. |
| `POLYMARKET_SIGNATURE_TYPE` | per_call | Yes | Set to your API credential. |
| `POLYMARKET_CHAIN_ID` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `polymarket-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $POLYMARKET_PRIVATE_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
