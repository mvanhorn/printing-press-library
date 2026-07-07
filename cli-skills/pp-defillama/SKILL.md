---
name: pp-defillama
description: "Printing Press CLI for Defillama. DefiLlama offers two ways to use our data with AI:"
author: "kierandotai"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - defillama-pp-cli
    install:
      - kind: go
        bins: [defillama-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/defillama/cmd/defillama-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/defillama/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Defillama — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `defillama-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install defillama --cli-only
   ```
2. Verify: `defillama-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/defillama/cmd/defillama-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Using AI?

DefiLlama offers two ways to use our data with AI:

| Resource | For | Description |
|---|---|---|
| <a href='/llms.txt'><b>llms.txt</b></a> | AI assistants (ChatGPT, Claude, Cursor, etc.) | Paste this link into your AI assistant for LLM-optimized docs |
| <a href='https://defillama.com/mcp'><b>MCP Server</b></a> | AI agents | Connect your agent directly to DefiLlama data — 23 tools, requires an API plan |

**Quick start (MCP)** — paste this into your AI agent:
```
Read https://raw.githubusercontent.com/DefiLlama/defillama-skills/refs/heads/master/defillama-setup/SKILL.md and follow the instructions to connect to DefiLlama MCP
```

---

Need higher rate limits or priority support? We offer a premium plan for 300$/mo. To get it, go to https://defillama.com/subscription

## SDK

**JavaScript** — `npm install @defillama/api` — [GitHub](https://github.com/DefiLlama/api-sdk)

**Python** — `pip install defillama-sdk` — [GitHub](https://github.com/DefiLlama/python-sdk)

Quick start (JavaScript):
```ts
import { DefiLlama } from '@defillama/api'

const client = new DefiLlama()
const protocols = await client.tvl.getProtocols()
```

Quick start (Python):
```py
from defillama_sdk import DefiLlama

client = DefiLlama()
protocols = client.tvl.getProtocols()
```

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**batch-historical** — Manage batch historical

- `defillama-pp-cli batch-historical` — Strings accepted by period and searchWidth: Can use regular chart candle notion like ‘4h’ etc where: W = week, D = day

**block** — Manage block

- `defillama-pp-cli block <chain> <timestamp>` — Runs binary search over a blockchain's blocks to get the closest one to a timestamp.

**bridges** — Manage bridges

- `defillama-pp-cli bridges get` — Get summary of bridge volume and volume breakdown by chain
- `defillama-pp-cli bridges get-bridgedaystats` — Get a 24hr token and address volume breakdown for a bridge
- `defillama-pp-cli bridges get-bridgevolume` — Get historical volumes for a bridge, chain, or bridge on a particular chain
- `defillama-pp-cli bridges get-transactions` — Get all transactions for a bridge within a date range
- `defillama-pp-cli bridges list` — List all bridges along with summaries of recent bridge volumes.

**categories** — Manage categories

- `defillama-pp-cli categories` — Overview of all categories accross all protocols

**chain-assets** — Manage chain assets

- `defillama-pp-cli chain-assets` — Get assets of all chains

**chains** — Manage chains

- `defillama-pp-cli chains` — Get current TVL of all chains

**chart** — Manage chart

- `defillama-pp-cli chart get` — Strings accepted by period and searchWidth: Can use regular chart candle notion like ‘4h’ etc where: W = week, D = day
- `defillama-pp-cli chart get-fork` — Returns an array of [timestamp, value] pairs representing TVL over time for all forks of a specific protocol.
- `defillama-pp-cli chart get-metric` — Returns an array of [timestamp, value] pairs representing the total metric value over time.
- `defillama-pp-cli chart get-oracle` — Returns an array of [timestamp, value] pairs representing oracle TVL over time for a specific chain.
- `defillama-pp-cli chart get-oracle-2` — Returns an array of [timestamp, value] pairs representing TVL over time for a specific oracle/protocol.
- `defillama-pp-cli chart get-oracle-3` — Returns an array of objects with a timestamp and TVL values broken down by oracle/protocol for a specific chain.
- `defillama-pp-cli chart get-oracle-4` — Returns an array of objects with a timestamp and TVL values broken down by chain for a specific oracle/protocol.
- `defillama-pp-cli chart get-pool` — Get historical APY and TVL of a pool
- `defillama-pp-cli chart get-treasury` — Returns an array of [timestamp, value] pairs representing the protocol's treasury value over time.
- `defillama-pp-cli chart get-treasury-2` — Returns an array of [timestamp, { chain: value }] pairs showing treasury value per chain over time.
- `defillama-pp-cli chart get-treasury-3` — Returns an array of [timestamp, { token: value }] pairs showing treasury value per token over time.
- `defillama-pp-cli chart get-tvl` — Returns an array of [timestamp, value] pairs representing the protocol's TVL over time.
- `defillama-pp-cli chart get-tvl-2` — Returns an array of [timestamp, { chain: value }] pairs showing the selected metric per chain over time.
- `defillama-pp-cli chart get-tvl-3` — Returns an array of [timestamp, { token: value }] pairs showing TVL per token over time.
- `defillama-pp-cli chart list` — Returns an array of [timestamp, value] pairs representing total TVL across all oracles over time.
- `defillama-pp-cli chart list-fork` — Returns an array of objects with a timestamp and TVL values broken down by fork protocol.
- `defillama-pp-cli chart list-oracle` — Returns an array of objects with a timestamp and TVL values broken down by chain.
- `defillama-pp-cli chart list-oracle-2` — Returns an array of objects with a timestamp and TVL values broken down by oracle/protocol.

**dat** — Manage dat

- `defillama-pp-cli dat get` — Returns detailed data for a specific institution, including mNAV calculations (realized, realistic, maximum)
- `defillama-pp-cli dat list` — Returns comprehensive data about institutions holding digital assets, including mNAV calculations (realized, realistic

**emission** — Manage emission

- `defillama-pp-cli emission <protocol>` — Unlocks data for a given token/protocol.

**emissions** — Manage emissions

- `defillama-pp-cli emissions` — List of all tokens along with basic info for each

**entities** — Manage entities

- `defillama-pp-cli entities` — List all entities

**equities** — Manage equities

- `defillama-pp-cli equities list` — Returns a list of all publicly traded companies tracked by DefiLlama, with current market summary data for each
- `defillama-pp-cli equities list-v1` — Returns a list of SEC filings (10-K, 10-Q, etc.) for the given ticker, sorted by filing date descending (newest first).
- `defillama-pp-cli equities list-v1-2` — Returns daily OHLCV bars as six-number arrays: Unix timestamp in seconds (UTC), open, high, low, close, volume.
- `defillama-pp-cli equities list-v1-3` — Returns daily closing prices as two-element arrays: ISO 8601 date-time string, then numeric price.
- `defillama-pp-cli equities list-v1-4` — Returns income statement, balance sheet, and cash flow statement for the given ticker
- `defillama-pp-cli equities list-v1-5` — Returns current market data for a single ticker.

**etfs** — Manage etfs

- `defillama-pp-cli etfs list` — Historical Flows at the Asset Level
- `defillama-pp-cli etfs list-snapshot` — Get ETFs and their metrics (aum, flows, fees...)

**fdv** — Manage fdv

- `defillama-pp-cli fdv <period>` — Get chart of narratives based on category performance (with individual coins weighted by mcap)

**forks** — Manage forks

- `defillama-pp-cli forks` — Overview of all forks accross all protocols

**hacks** — Manage hacks

- `defillama-pp-cli hacks` — Overview of all hacks on our Hacks dashboard

**historical-chain-tvl** — Manage historical chain tvl

- `defillama-pp-cli historical-chain-tvl get` — Get historical TVL (excludes liquid staking and double counted tvl) of a chain
- `defillama-pp-cli historical-chain-tvl list` — Get historical TVL (excludes liquid staking and double counted tvl) of DeFi on all chains

**historical-liquidity** — Manage historical liquidity

- `defillama-pp-cli historical-liquidity <token>` — Provides the name of contracts on a determined chain

**inflows** — Manage inflows

- `defillama-pp-cli inflows <protocol> <timestamp>` — Lists the amount of inflows and outflows for a protocol at a given date

**metrics** — Manage metrics

- `defillama-pp-cli metrics get` — Returns aggregate metrics for the specified dimension including totals and percentage changes across different time
- `defillama-pp-cli metrics get-financialstatement` — Returns protocol metadata, methodology details, and aggregated financial statement data (yearly, quarterly, monthly).
- `defillama-pp-cli metrics get-treasury` — Returns protocol metadata along with current treasury figures and chain breakdowns.
- `defillama-pp-cli metrics get-tvl` — Returns protocol metadata along with current TVL figures, chain breakdowns, and other aggregate metrics.
- `defillama-pp-cli metrics list` — Returns an object mapping fork names to arrays of protocol names that are forks of each protocol.
- `defillama-pp-cli metrics list-oracle` — Returns an object mapping oracle names to arrays of protocol names that use each oracle.

**oracles** — Manage oracles

- `defillama-pp-cli oracles` — Overview of all oracles accross all protocols

**overview** — Manage overview

- `defillama-pp-cli overview get` — List all dexs along with summaries of their volumes and dataType history data filtering by chain
- `defillama-pp-cli overview get-fees` — List all protocols along with summaries of their fees and revenue and dataType history data by chain
- `defillama-pp-cli overview get-options` — List all options dexs along with summaries of their volumes and dataType history data filtering by chain
- `defillama-pp-cli overview list` — List all dexs along with summaries of their volumes and dataType history data
- `defillama-pp-cli overview list-derivatives` — Lists all derivatives along summaries of their volumes filtering by chain
- `defillama-pp-cli overview list-fees` — List all protocols along with summaries of their fees and revenue and dataType history data
- `defillama-pp-cli overview list-openinterest` — List all open interest dex exchanges along with summaries of their open interest
- `defillama-pp-cli overview list-options` — List all options dexs along with summaries of their volumes and dataType history data

**percentage** — Manage percentage

- `defillama-pp-cli percentage <coins>` — Strings accepted by period: Can use regular chart candle notion like ‘4h’ etc where: W = week, D = day, H = hour

**pools** — Manage pools

- `defillama-pp-cli pools` — Retrieve the latest data for all pools, including enriched information such as predictions

**prices** — Manage prices

- `defillama-pp-cli prices get` — The goal of this API is to price as many tokens as possible, including exotic ones that never get traded
- `defillama-pp-cli prices get-first` — Get earliest timestamp price record for coins
- `defillama-pp-cli prices get-historical` — See /prices/current for explanation on how prices are sourced.

**protocol** — Manage protocol

- `defillama-pp-cli protocol <protocol>` — Get historical TVL of a protocol and breakdowns by token and chain

**protocols** — Manage protocols

- `defillama-pp-cli protocols` — List all protocols on defillama along with their tvl

**raises** — Manage raises

- `defillama-pp-cli raises` — Overview of all raises on our Raises dashboard

**rwa** — Manage rwa

- `defillama-pp-cli rwa get` — Returns current RWA assets that have onchain market cap, active market cap, or DeFi active TVL on the requested chain.
- `defillama-pp-cli rwa get-chart` — Returns historical onchain market cap, active market cap, and DeFi active TVL totals for a chain.
- `defillama-pp-cli rwa list` — Returns current Real World Asset rows with per-chain onchain market cap, active market cap, and DeFi active TVL maps.
- `defillama-pp-cli rwa list-chart` — Returns time series rows with one column per chain for the selected RWA metric.
- `defillama-pp-cli rwa list-list` — Returns lightweight RWA lists used for discovery, search, and filters.
- `defillama-pp-cli rwa list-stats` — Returns RWA aggregates. For the default per-chain table, read byChain[chain].base: assetIssuers.

**stablecoin** — Data from our stablecoins dashboard

- `defillama-pp-cli stablecoin <asset>` — Get historical mcap and historical chain distribution of a stablecoin

**stablecoinchains** — Manage stablecoinchains

- `defillama-pp-cli stablecoinchains` — Get current mcap sum of all stablecoins on each chain

**stablecoincharts** — Manage stablecoincharts

- `defillama-pp-cli stablecoincharts get` — Get historical mcap sum of all stablecoins in a chain
- `defillama-pp-cli stablecoincharts list` — Get historical mcap sum of all stablecoins

**stablecoinprices** — Manage stablecoinprices

- `defillama-pp-cli stablecoinprices` — Get historical prices of all stablecoins

**stablecoins** — Data from our stablecoins dashboard

- `defillama-pp-cli stablecoins get` — Get stablecoin dominance per chain along with the info about the larges coin in a chain
- `defillama-pp-cli stablecoins list` — List all stablecoins along with their circulating amounts

**summary** — Manage summary

- `defillama-pp-cli summary get` — Get summary of dex volume with historical data
- `defillama-pp-cli summary get-derivatives` — Volume Details about a specific perp protocol
- `defillama-pp-cli summary get-fees` — Get summary of protocol fees and revenue with historical data
- `defillama-pp-cli summary get-options` — Get summary of options dex volume with historical data

**token-protocols** — Manage token protocols

- `defillama-pp-cli token-protocols <symbol>` — Lists the amount of a certain token within all protocols. Data for the Token Usage page

**treasuries** — Manage treasuries

- `defillama-pp-cli treasuries` — List all protocols on our Treasuries dashboard

**tvl** — Retrieve TVL data

- `defillama-pp-cli tvl <protocol>` — Simplified endpoint that only returns a number, the current TVL of a protocol

**usage** — Manage usage

- `defillama-pp-cli usage` — Get amount of credits left in the api key, these reset on the 1st of each month

**yields** — Data from our yields/APY dashboard

- `defillama-pp-cli yields get` — Historical borrow cost APY from a pool on a lending market, pool ids should be obtained from /poolsBorrow
- `defillama-pp-cli yields list` — APY rates of multiple LSDs
- `defillama-pp-cli yields list-perps` — Funding rates and Open Interest of perps across exchanges, including both Decentralized and Centralized
- `defillama-pp-cli yields list-poolsborrow` — Borrow costs APY of assets from lending markets
- `defillama-pp-cli yields list-poolsold` — Same as /pools but it also includes a new parameter `pool_old` which usually contains pool address (but not guaranteed)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
defillama-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `defillama-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  defillama-pp-cli batch-historical --coins example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
defillama-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
defillama-pp-cli feedback --stdin < notes.txt
defillama-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/defillama-pp-cli/feedback.jsonl`. They are never POSTed unless `DEFILLAMA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DEFILLAMA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
defillama-pp-cli profile save briefing --json
defillama-pp-cli --profile briefing batch-historical --coins example-value
defillama-pp-cli profile list --json
defillama-pp-cli profile show briefing
defillama-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `defillama-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/defillama/cmd/defillama-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add defillama-pp-mcp -- defillama-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which defillama-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   defillama-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `defillama-pp-cli <command> --help`.
