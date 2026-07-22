# Defillama CLI

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

Created by [@kierandotai](https://github.com/kierandotai) (kierandotai).

## Install

The recommended path installs both the `defillama-pp-cli` binary and the `pp-defillama` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install defillama
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install defillama --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install defillama --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install defillama --agent claude-code
npx -y @mvanhorn/printing-press-library install defillama --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/defillama/cmd/defillama-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/defillama-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install defillama --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-defillama --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-defillama --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install defillama --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/defillama-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/defillama/cmd/defillama-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "defillama": {
      "command": "defillama-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
defillama-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
defillama-pp-cli batch-historical --coins example-value
```

## Usage

Run `defillama-pp-cli --help` for the full command reference and flag list.

## Commands

### batch-historical

Manage batch historical

- **`defillama-pp-cli batch-historical`** - Strings accepted by period and searchWidth:
Can use regular chart candle notion like ‘4h’ etc where:
W = week, D = day, H = hour, M = minute (not case sensitive)

### block

Manage block

- **`defillama-pp-cli block <chain> <timestamp>`** - Runs binary search over a blockchain's blocks to get the closest one to a timestamp.
Every time this is run we add new data to our database, so each query permanently speeds up future queries.

### bridges

Manage bridges

- **`defillama-pp-cli bridges get`** - Get summary of bridge volume and volume breakdown by chain
- **`defillama-pp-cli bridges get-bridgedaystats`** - Get a 24hr token and address volume breakdown for a bridge
- **`defillama-pp-cli bridges get-bridgevolume`** - Get historical volumes for a bridge, chain, or bridge on a particular chain
- **`defillama-pp-cli bridges get-transactions`** - Get all transactions for a bridge within a date range
- **`defillama-pp-cli bridges list`** - List all bridges along with summaries of recent bridge volumes.

### categories

Manage categories

- **`defillama-pp-cli categories`** - Overview of all categories accross all protocols

### chain-assets

Manage chain assets

- **`defillama-pp-cli chain-assets`** - Get assets of all chains

### chains

Manage chains

- **`defillama-pp-cli chains`** - Get current TVL of all chains

### chart

Manage chart

- **`defillama-pp-cli chart get`** - Strings accepted by period and searchWidth:
Can use regular chart candle notion like ‘4h’ etc where:
W = week, D = day, H = hour, M = minute (not case sensitive)
- **`defillama-pp-cli chart get-fork`** - Returns an array of [timestamp, value] pairs representing TVL over time for all forks of a specific protocol.
- **`defillama-pp-cli chart get-metric`** - Returns an array of [timestamp, value] pairs representing the total metric value over time.
- **`defillama-pp-cli chart get-oracle`** - Returns an array of [timestamp, value] pairs representing oracle TVL over time for a specific chain.
- **`defillama-pp-cli chart get-oracle-2`** - Returns an array of [timestamp, value] pairs representing TVL over time for a specific oracle/protocol.
- **`defillama-pp-cli chart get-oracle-3`** - Returns an array of objects with a timestamp and TVL values broken down by oracle/protocol for a specific chain.
- **`defillama-pp-cli chart get-oracle-4`** - Returns an array of objects with a timestamp and TVL values broken down by chain for a specific oracle/protocol.
- **`defillama-pp-cli chart get-pool`** - Get historical APY and TVL of a pool
- **`defillama-pp-cli chart get-treasury`** - Returns an array of [timestamp, value] pairs representing the protocol's treasury value over time. By default excludes the protocol's own tokens (OwnTokens).
- **`defillama-pp-cli chart get-treasury-2`** - Returns an array of [timestamp, { chain: value }] pairs showing treasury value per chain over time. By default excludes the protocol's own tokens.
- **`defillama-pp-cli chart get-treasury-3`** - Returns an array of [timestamp, { token: value }] pairs showing treasury value per token over time. By default excludes OwnTokens and values are in USD. Use key and currency params to customize.
- **`defillama-pp-cli chart get-tvl`** - Returns an array of [timestamp, value] pairs representing the protocol's TVL over time. By default returns the base TVL metric. Use the `key` parameter to select a different metric or aggregate all metrics.
- **`defillama-pp-cli chart get-tvl-2`** - Returns an array of [timestamp, { chain: value }] pairs showing the selected metric per chain over time.
- **`defillama-pp-cli chart get-tvl-3`** - Returns an array of [timestamp, { token: value }] pairs showing TVL per token over time. Values are in USD by default, set currency=tokens for raw token amounts.
- **`defillama-pp-cli chart list`** - Returns an array of [timestamp, value] pairs representing total TVL across all oracles over time.
- **`defillama-pp-cli chart list-fork`** - Returns an array of objects with a timestamp and TVL values broken down by fork protocol.
- **`defillama-pp-cli chart list-oracle`** - Returns an array of objects with a timestamp and TVL values broken down by chain.
- **`defillama-pp-cli chart list-oracle-2`** - Returns an array of objects with a timestamp and TVL values broken down by oracle/protocol.

### dat

Manage dat

- **`defillama-pp-cli dat get`** - Returns detailed data for a specific institution, including mNAV calculations (realized, realistic, maximum) as described in the [DAT Methodology](https://docs.llama.fi/analysts/dat-methodology)
- **`defillama-pp-cli dat list`** - Returns comprehensive data about institutions holding digital assets, including mNAV calculations (realized, realistic, maximum) as described in the [DAT Methodology](https://docs.llama.fi/analysts/dat-methodology)

### emission

Manage emission

- **`defillama-pp-cli emission <protocol>`** - Unlocks data for a given token/protocol. You can find a list of available slugs to query by querying /emissions and then extracting the key `gecko_id`

### emissions

Manage emissions

- **`defillama-pp-cli emissions`** - List of all tokens along with basic info for each

### entities

Manage entities

- **`defillama-pp-cli entities`** - List all entities

### equities

Manage equities

- **`defillama-pp-cli equities list`** - Returns a list of all publicly traded companies tracked by DefiLlama, with current market summary data for each, sorted by market capitalization (largest first)
- **`defillama-pp-cli equities list-v1`** - Returns a list of SEC filings (10-K, 10-Q, etc.) for the given ticker, sorted by filing date descending (newest first).
- **`defillama-pp-cli equities list-v1-2`** - Returns daily OHLCV bars as six-number arrays: Unix timestamp in seconds (UTC), open, high, low, close, volume. Sorted by time descending (newest first). Optional `timeframe` filters how far back data goes; omit or empty for full history (`MAX`).
- **`defillama-pp-cli equities list-v1-3`** - Returns daily closing prices as two-element arrays: ISO 8601 date-time string, then numeric price. Sorted by date descending (newest first).
- **`defillama-pp-cli equities list-v1-4`** - Returns income statement, balance sheet, and cash flow statement for the given ticker, broken down by quarterly and annual periods.
- **`defillama-pp-cli equities list-v1-5`** - Returns current market data for a single ticker. This is a compact snapshot (no `ticker` / `name` fields); use `GET /equities/v1/companies` for the list shape that includes company identity and balance-sheet highlights.

### etfs

Manage etfs

- **`defillama-pp-cli etfs list`** - Historical Flows at the Asset Level
- **`defillama-pp-cli etfs list-snapshot`** - Get ETFs and their metrics (aum, flows, fees...)

### fdv

Manage fdv

- **`defillama-pp-cli fdv <period>`** - Get chart of narratives based on category performance (with individual coins weighted by mcap)

### forks

Manage forks

- **`defillama-pp-cli forks`** - Overview of all forks accross all protocols

### hacks

Manage hacks

- **`defillama-pp-cli hacks`** - Overview of all hacks on our Hacks dashboard

### historical-chain-tvl

Manage historical chain tvl

- **`defillama-pp-cli historical-chain-tvl get`** - Get historical TVL (excludes liquid staking and double counted tvl) of a chain
- **`defillama-pp-cli historical-chain-tvl list`** - Get historical TVL (excludes liquid staking and double counted tvl) of DeFi on all chains

### historical-liquidity

Manage historical liquidity

- **`defillama-pp-cli historical-liquidity <token>`** - Provides the name of contracts on a determined chain

### inflows

Manage inflows

- **`defillama-pp-cli inflows <protocol> <timestamp>`** - Lists the amount of inflows and outflows for a protocol at a given date

### metrics

Manage metrics

- **`defillama-pp-cli metrics get`** - Returns aggregate metrics for the specified dimension including totals and percentage changes across different time periods.
- **`defillama-pp-cli metrics get-financialstatement`** - Returns protocol metadata, methodology details, and aggregated financial statement data (yearly, quarterly, monthly). Each period contains line items such as Gross Protocol Revenue, Cost Of Revenue, Gross Profit, Token Holder Net Income, Incentives, and Earnings, with values and optional label breakdowns.

When querying a parent protocol (e.g. `aave`), the response includes a `childProtocols` array with per-version methodology. When querying a child protocol (e.g. `aave-v3`), methodology and breakdownMethodology are at the top level.
- **`defillama-pp-cli metrics get-treasury`** - Returns protocol metadata along with current treasury figures and chain breakdowns.
- **`defillama-pp-cli metrics get-tvl`** - Returns protocol metadata along with current TVL figures, chain breakdowns, and other aggregate metrics.
- **`defillama-pp-cli metrics list`** - Returns an object mapping fork names to arrays of protocol names that are forks of each protocol.
- **`defillama-pp-cli metrics list-oracle`** - Returns an object mapping oracle names to arrays of protocol names that use each oracle.

### oracles

Manage oracles

- **`defillama-pp-cli oracles`** - Overview of all oracles accross all protocols

### overview

Manage overview

- **`defillama-pp-cli overview get`** - List all dexs along with summaries of their volumes and dataType history data filtering by chain
- **`defillama-pp-cli overview get-fees`** - List all protocols along with summaries of their fees and revenue and dataType history data by chain
- **`defillama-pp-cli overview get-options`** - List all options dexs along with summaries of their volumes and dataType history data filtering by chain
- **`defillama-pp-cli overview list`** - List all dexs along with summaries of their volumes and dataType history data
- **`defillama-pp-cli overview list-derivatives`** - Lists all derivatives along summaries of their volumes filtering by chain
- **`defillama-pp-cli overview list-fees`** - List all protocols along with summaries of their fees and revenue and dataType history data
- **`defillama-pp-cli overview list-openinterest`** - List all open interest dex exchanges along with summaries of their open interest
- **`defillama-pp-cli overview list-options`** - List all options dexs along with summaries of their volumes and dataType history data

### percentage

Manage percentage

- **`defillama-pp-cli percentage <coins>`** - Strings accepted by period:
Can use regular chart candle notion like ‘4h’ etc where:
W = week, D = day, H = hour, M = minute (not case sensitive)

### pools

Manage pools

- **`defillama-pp-cli pools`** - Retrieve the latest data for all pools, including enriched information such as predictions

### prices

Manage prices

- **`defillama-pp-cli prices get`** - The goal of this API is to price as many tokens as possible, including exotic ones that never get traded, which makes them impossible to price by looking at markets.

The base of our data are prices pulled from coingecko, which is then extended through multiple means:
- We price all bridged tokens by using the price of the token in it's original chain, so we fetch all bridged versions of USDC on arbitrum, fantom, avax... and price all them using the price for the token on Ethereum, which we know. Right now we support 10 different bridging protocols.
- We have multiple adapters to price specialized sets of tokens by running custom code:
  - We price yearn's yToken LPs by checking how much underlying token can be withdrawn for each LP
  - Aave, compound and euler LP tokens are also priced based on their relationship against underlying tokens
  - Uniswap, curve, balancer and stargate LPs are priced using the underlying tokens in each pair
  - GMX's GLP token is priced based on the value of tokens given on withdrawal (which includes calculations based on trader's PnL)
  
  - Synthetix tokens are priced using forex prices of the coin they are pegged to
- For tokens that we haven't been able to price in any other way, we find the pool with most liquidity for each on uniswap, curve and serum and then use the prices provided on those exchanges.
  
  Unlike all the other tokens, we can't confirm that these prices are correct, so we only ingest the ones that have sufficient liquidity and, even in that case, we attach a `confidence` value to them that is related to the depth of liquidity and which represents our confidence in the quality of each price. API consumers can choose to filter out prices with low confidence values.
  
 Our API server is fully open source and we are constantly adding more pricing adapters, extending the amount of tokens we support.

  
Tokens are queried using {chain}:{address}, where chain is an identifier such as ethereum, bsc, polygon, avax... You can also get tokens by coingecko id by setting `coingecko` as the chain, eg: coingecko:ethereum, coingecko:bitcoin. Examples:
  - ethereum:0xdF574c24545E5FfEcb9a659c229253D4111d87e1
  - bsc:0x762539b45a1dcce3d36d080f74d1aed37844b878
  - coingecko:ethereum
  - arbitrum:0x4277f8f2c384827b5273592ff7cebd9f2c1ac258
- **`defillama-pp-cli prices get-first`** - Get earliest timestamp price record for coins
- **`defillama-pp-cli prices get-historical`** - See /prices/current for explanation on how prices are sourced.

### protocol

Manage protocol

- **`defillama-pp-cli protocol <protocol>`** - Get historical TVL of a protocol and breakdowns by token and chain

### protocols

Manage protocols

- **`defillama-pp-cli protocols`** - List all protocols on defillama along with their tvl

### raises

Manage raises

- **`defillama-pp-cli raises`** - Overview of all raises on our Raises dashboard

### rwa

Manage rwa

- **`defillama-pp-cli rwa get`** - Returns current RWA assets that have onchain market cap, active market cap, or DeFi active TVL on the requested chain.
- **`defillama-pp-cli rwa get-chart`** - Returns historical onchain market cap, active market cap, and DeFi active TVL totals for a chain.
- **`defillama-pp-cli rwa list`** - Returns current Real World Asset rows with per-chain onchain market cap, active market cap, and DeFi active TVL maps.
- **`defillama-pp-cli rwa list-chart`** - Returns time series rows with one column per chain for the selected RWA metric.
- **`defillama-pp-cli rwa list-list`** - Returns lightweight RWA lists used for discovery, search, and filters.
- **`defillama-pp-cli rwa list-stats`** - Returns RWA aggregates. For the default per-chain table, read byChain[chain].base: assetIssuers.length, assetCount, activeMcap, onChainMcap, and defiActiveTvl. Add stablecoinsOnly, governanceOnly, and stablecoinsAndGovernance when those buckets should be included.

### stablecoin

Data from our stablecoins dashboard

- **`defillama-pp-cli stablecoin <asset>`** - Get historical mcap and historical chain distribution of a stablecoin

### stablecoinchains

Manage stablecoinchains

- **`defillama-pp-cli stablecoinchains`** - Get current mcap sum of all stablecoins on each chain

### stablecoincharts

Manage stablecoincharts

- **`defillama-pp-cli stablecoincharts get`** - Get historical mcap sum of all stablecoins in a chain
- **`defillama-pp-cli stablecoincharts list`** - Get historical mcap sum of all stablecoins

### stablecoinprices

Manage stablecoinprices

- **`defillama-pp-cli stablecoinprices`** - Get historical prices of all stablecoins

### stablecoins

Data from our stablecoins dashboard

- **`defillama-pp-cli stablecoins get`** - Get stablecoin dominance per chain along with the info about the larges coin in a chain
- **`defillama-pp-cli stablecoins list`** - List all stablecoins along with their circulating amounts

### summary

Manage summary

- **`defillama-pp-cli summary get`** - Get summary of dex volume with historical data
- **`defillama-pp-cli summary get-derivatives`** - Volume Details about a specific perp protocol
- **`defillama-pp-cli summary get-fees`** - Get summary of protocol fees and revenue with historical data
- **`defillama-pp-cli summary get-options`** - Get summary of options dex volume with historical data

### token-protocols

Manage token protocols

- **`defillama-pp-cli token-protocols <symbol>`** - Lists the amount of a certain token within all protocols. Data for the Token Usage page

### treasuries

Manage treasuries

- **`defillama-pp-cli treasuries`** - List all protocols on our Treasuries dashboard

### tvl

Retrieve TVL data

- **`defillama-pp-cli tvl <protocol>`** - Simplified endpoint that only returns a number, the current TVL of a protocol

### usage

Manage usage

- **`defillama-pp-cli usage`** - Get amount of credits left in the api key, these reset on the 1st of each month

### yields

Data from our yields/APY dashboard

- **`defillama-pp-cli yields get`** - Historical borrow cost APY from a pool on a lending market, pool ids should be obtained from /poolsBorrow
- **`defillama-pp-cli yields list`** - APY rates of multiple LSDs
- **`defillama-pp-cli yields list-perps`** - Funding rates and Open Interest of perps across exchanges, including both Decentralized and Centralized
- **`defillama-pp-cli yields list-poolsborrow`** - Borrow costs APY of assets from lending markets
- **`defillama-pp-cli yields list-poolsold`** - Same as /pools but it also includes a new parameter `pool_old` which usually contains pool address (but not guaranteed)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
defillama-pp-cli batch-historical --coins example-value

# JSON for scripting and agents
defillama-pp-cli batch-historical --coins example-value --json

# Filter to specific fields
defillama-pp-cli batch-historical --coins example-value --json --select id,name,status

# Dry run — show the request without sending
defillama-pp-cli batch-historical --coins example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
defillama-pp-cli batch-historical --coins example-value --agent
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
defillama-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/defillama-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
