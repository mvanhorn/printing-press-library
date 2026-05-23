---
name: pp-polymarket
description: "Printing Press CLI for Polymarket. Every Polymarket feature the official Rust CLI ships, plus six novel commands no Polymarket tool offers"
author: "Ahmad Thariq Syauqi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - polymarket-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/polymarket/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Polymarket — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `polymarket-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install polymarket --cli-only
   ```
2. Verify: `polymarket-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/polymarket/cmd/polymarket-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Polymarket feature the official Rust CLI ships, plus six novel commands no Polymarket tool offers: resolution radar, reward-yield ranker, position drift, price diff, research bundle, batch redeem. Mirrors Gamma + CLOB + Data API surfaces with agent-native JSON, local SQLite + FTS5, and an MCP server.

## Unique Capabilities

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
- **`redeem all`** — Discover every position in resolved markets and report the redeemable set with totals. In this build --broadcast is a documented stub (real on-chain ctf redeem needs go-ethereum + Polygon RPC, wired in v0.2); the default --dry-run produces the exact call set you can replay through the official Polymarket Rust CLI for live broadcast.

  _When the agent is told 'claim everything I've won', this turns a 4-step ritual into one command with a dry-run preview._

  ```bash
  polymarket-pp-cli redeem all --dry-run --min-value 1 --agent
  ```

## Command Reference

**api_keys** — Manage L2 API credentials.

- `polymarket-pp-cli api_keys delete` — Delete the L2 API key currently in use
- `polymarket-pp-cli api_keys list` — List API keys for the authenticated wallet

**balance** — Read on-chain USDC + outcome-token balances.

- `polymarket-pp-cli balance` — Get balance and allowance for the authenticated wallet

**clob** — CLOB order book reads, market metadata, server health, and reward configs.

- `polymarket-pp-cli clob book` — Get the full order book for an outcome token
- `polymarket-pp-cli clob books` — Batch order book fetch
- `polymarket-pp-cli clob market` — Get CLOB metadata for a single market
- `polymarket-pp-cli clob markets` — List CLOB markets
- `polymarket-pp-cli clob midpoint` — Get the midpoint price
- `polymarket-pp-cli clob neg-risk` — Get neg-risk markets
- `polymarket-pp-cli clob ok` — CLOB API liveness probe
- `polymarket-pp-cli clob price` — Get the best bid or ask price
- `polymarket-pp-cli clob price-history` — Historical price series for an outcome token
- `polymarket-pp-cli clob spread` — Get the current bid-ask spread
- `polymarket-pp-cli clob time` — Get the CLOB server time

**comments** — Public comments on markets and events.

- `polymarket-pp-cli comments` — List comments

**data** — Polymarket Data API for positions with valuation, portfolio value, activity history, and market holders.

- `polymarket-pp-cli data activity` — User activity feed: trades, splits, merges, redeems, rewards, conversions
- `polymarket-pp-cli data holders` — Get holder distribution for a market
- `polymarket-pp-cli data positions` — List positions for a wallet address with current price + value + P&L
- `polymarket-pp-cli data value` — Get the total portfolio value for a wallet

**events** — Browse and inspect events (groups of related markets) via the Gamma API.

- `polymarket-pp-cli events get` — Fetch a single event by id or slug
- `polymarket-pp-cli events list` — List events with filtering
- `polymarket-pp-cli events tags` — List the tags attached to an event

**markets** — Discover, search, and inspect prediction markets via the Gamma API. Markets are individual yes/no contracts; events group related markets.

- `polymarket-pp-cli markets get` — Fetch a single market by id or slug.
- `polymarket-pp-cli markets list` — List prediction markets with filtering. Active, recently-resolved, by tag, by event, etc.
- `polymarket-pp-cli markets search` — Search markets by text.

**orders** — Place, cancel, list, and inspect orders on the CLOB. Requires L1 wallet PK + L2 HMAC credentials.

- `polymarket-pp-cli orders cancel` — Cancel a single order by id
- `polymarket-pp-cli orders cancel-all` — Cancel every open order for the authenticated user
- `polymarket-pp-cli orders cancel-batch` — Cancel multiple orders by id list
- `polymarket-pp-cli orders cancel-market` — Cancel all open orders for a single market
- `polymarket-pp-cli orders get` — Get a single order by id
- `polymarket-pp-cli orders list` — List the authenticated user's open orders

**rewards** — Liquidity-rewards data: configs, payouts, earnings, and order scoring.

- `polymarket-pp-cli rewards current` — Get current live reward eligibility
- `polymarket-pp-cli rewards earnings` — Get the authenticated user's daily reward earnings
- `polymarket-pp-cli rewards earnings-markets` — Get reward earnings per market
- `polymarket-pp-cli rewards list` — List all currently reward-eligible markets
- `polymarket-pp-cli rewards market-reward` — Get the reward config for a single market
- `polymarket-pp-cli rewards order-scoring` — Get the reward score for a single open order
- `polymarket-pp-cli rewards orders-scoring` — Get reward scores for multiple open orders
- `polymarket-pp-cli rewards percentages` — Get current reward distribution percentages

**series** — Recurring event series.

- `polymarket-pp-cli series get` — Fetch a single series by id
- `polymarket-pp-cli series list` — List event series

**tags** — Browse Polymarket category tags.

- `polymarket-pp-cli tags` — List all tags

**trades** — Read trades the authenticated user has made on the CLOB.

- `polymarket-pp-cli trades` — List the authenticated user's trade history


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
polymarket-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to  in Chrome, then:

```bash
polymarket-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `polymarket-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  polymarket-pp-cli api_keys list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
polymarket-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
polymarket-pp-cli feedback --stdin < notes.txt
polymarket-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.polymarket-pp-cli/feedback.jsonl`. They are never POSTed unless `POLYMARKET_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POLYMARKET_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
polymarket-pp-cli profile save briefing --json
polymarket-pp-cli --profile briefing api_keys list
polymarket-pp-cli profile list --json
polymarket-pp-cli profile show briefing
polymarket-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `polymarket-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add polymarket-pp-mcp -- polymarket-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which polymarket-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   polymarket-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `polymarket-pp-cli <command> --help`.
