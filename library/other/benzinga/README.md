# Benzinga CLI

**Every Benzinga calendar, news, fundamentals, and signal endpoint as a typed command — plus an offline SQLite store, full-text search, cross-entity queries, and the first Benzinga MCP server.**

Benzinga's licensed financial-data API is powerful but fragmented across ~60 endpoints and has only one complete client — a Python library with no CLI, no offline store, and no agent surface. This CLI covers the full documented REST surface as first-class commands, delta-syncs the calendar/news/signal families into a local database via the API's own updated cursors, and adds cross-entity commands the REST API cannot express in one call: watch a ticker set for overnight changes, explain why one name is moving, and rank analysts by accuracy.

Created by [@waveriderai](https://github.com/waveriderai) (waveriderai).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

Local `search` keeps alphanumeric tokens only (letters, digits, underscore) and requires every token (implicit AND). FTS5 operators such as AND, OR, NOT and quoted phrases are not supported.

## Install

The recommended path installs both the `benzinga-pp-cli` binary and the `pp-benzinga` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install benzinga
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install benzinga --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install benzinga --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install benzinga --agent claude-code
npx -y @mvanhorn/printing-press-library install benzinga --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/benzinga/cmd/benzinga-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/benzinga-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install benzinga --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-benzinga --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-benzinga --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install benzinga --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/benzinga-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BENZINGA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/benzinga/cmd/benzinga-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "benzinga": {
      "command": "benzinga-pp-mcp",
      "env": {
        "BENZINGA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Benzinga uses a static API token passed as a query parameter. Set BENZINGA_API_KEY in your environment (or run benzinga-pp-cli auth set-token) and every command attaches it as ?token=. Access is tier-gated per product, so a valid token can still return 403 on an endpoint your plan does not include — that is a licensing boundary, not a CLI bug.

### Per-product tokens (news/Pro vs market)

Benzinga entitlements are split across two token families and no single token covers everything:

- **News/Pro token** — news, WIIM (`wiims`), fundamentals, signals, and delayed quotes (`--with-points`). Set as the default token (`BENZINGA_API_KEY` / `auth set-token`).
- **Market/super-token** — market movers (`market`), bars, short interest, logos, and the calendar. Set it separately so both work at once, without swapping:

```bash
# Default (news/Pro) token — used by wiims, news, fundamentals, signals
benzinga-pp-cli auth set-token YOUR_PRO_TOKEN

# Market/super-token — used by market movers, bars, short interest, logos, calendar
benzinga-pp-cli auth set-token --market YOUR_MARKET_TOKEN     # or export BENZINGA_MARKET_API_KEY=...
```

The CLI routes each request to the right token by endpoint path (`market`/`bars`/`shortinterest`/`logos`/`calendar` → market token; everything else → default). If you set only the default token, every command uses it — single-token setups behave exactly as before. WIIM in particular needs a Pro token: the market/super-tokens are not entitled to the WIIM channel and return no items.

## Quick Start

```bash
# Confirm the binary and config resolve before adding a token.
benzinga-pp-cli doctor --dry-run

# Pull recent analyst rating changes for one ticker.
benzinga-pp-cli calendar get-ratings --parameters-tickers AAPL

# Latest headlines for a ticker.
benzinga-pp-cli news get --tickers NVDA --display-output headline --page-size 10

# Build the local store so offline search and cross-entity commands work.
benzinga-pp-cli sync --resources calendar-ratings,calendar-earnings,news --since 7d

# See everything that changed on a watchlist since yesterday.
benzinga-pp-cli watch AAPL,NVDA,TSLA --since 24h --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-entity local state
- **`watch`** — See everything that changed on your tickers since you last looked — new ratings, price-target moves, breaking news, and signals in one diff.

  _Reach for this when an agent needs a single 'what moved on my names' digest instead of fanning out across ratings, news, and signals endpoints._

  ```bash
  benzinga-pp-cli watch AAPL,NVDA,TSLA --since 24h --agent
  ```
- **`why`** — Build one time-ordered catalyst timeline for a ticker by merging unusual options, block trades, halts, rating changes, and news.

  _Use when the question is 'why is X moving right now' and the answer needs options + halts + ratings + headlines stitched in order._

  ```bash
  benzinga-pp-cli why NVDA --window 1d --agent
  ```
- **`catalysts`** — One forward-dated agenda per ticker set unioning earnings, dividends, splits, IPOs, FDA dates, conference calls, guidance, and offerings.

  _Reach for this to get every upcoming dated event on a watchlist in one ordered list rather than querying eight calendar endpoints._

  ```bash
  benzinga-pp-cli catalysts AAPL,LLY,MRNA --ahead 14d --agent
  ```
- **`insider-cluster`** — Flag tickers where several distinct members of Congress filed purchases within a window — cluster detection beyond a single disclosure.

  _Use to surface conviction signals where multiple unrelated members of Congress bought the same name, not just one routine filing._

  ```bash
  benzinga-pp-cli insider-cluster --window 30d --min 3 --agent
  ```

### Analyst signal quality
- **`analyst-accuracy`** — Rank rating-issuing firms and analysts by Benzinga's historical accuracy, and tag today's rating changes with the issuer's hit rate.

  _Use when an agent must judge whether a fresh upgrade/downgrade comes from an analyst with a real track record._

  ```bash
  benzinga-pp-cli analyst-accuracy --ticker AAPL --agent
  ```

### Earnings intelligence
- **`earnings-season`** — Compute EPS and revenue beat/miss and surprise % from the earnings calendar and link each name to its conference call and transcript.

  _Reach for this during earnings season to rank the week's reports by surprise magnitude with calls/transcripts attached._

  ```bash
  benzinga-pp-cli earnings-season --from 7d --agent --select ticker,eps_surprise_pct,beat
  ```

## Recipes

### Rank the week's earnings by surprise

```bash
benzinga-pp-cli earnings-season --from 7d --agent --select ticker,eps_surprise_pct,beat
```

Computes beat/miss and surprise % offline from synced earnings rows and projects just the decision fields.

### Explain a sudden move

```bash
benzinga-pp-cli why NVDA --window 1d --agent
```

Stitches unusual options, halts, rating changes, and news for NVDA into one chronological catalyst timeline.

### Morning watchlist diff

```bash
benzinga-pp-cli watch AAPL,NVDA,TSLA --since 24h --agent --select ticker,kind,headline
```

Cross-entity diff of ratings, news, and signals on a ticker set, narrowed to the most relevant nested fields with --select.

### Weekly US economic calendar

```bash
benzinga-pp-cli calendar get-economics --country USA
```

Recent and latest macro releases for the US with actual vs consensus (country codes are 3-digit, e.g. USA).

### Vet a rating change

```bash
benzinga-pp-cli analyst-accuracy --ticker AAPL --today --agent
```

Ranks the firms/analysts issuing fresh AAPL ratings by their historical accuracy.

## Usage

Run `benzinga-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BENZINGA_CONFIG_DIR`, `BENZINGA_DATA_DIR`, `BENZINGA_STATE_DIR`, or `BENZINGA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BENZINGA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BENZINGA_HOME=/srv/benzinga
benzinga-pp-cli doctor
```

Under `BENZINGA_HOME=/srv/benzinga`, the four dirs resolve to `/srv/benzinga/config`, `/srv/benzinga/data`, `/srv/benzinga/state`, and `/srv/benzinga/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "benzinga": {
      "command": "benzinga-pp-mcp",
      "env": {
        "BENZINGA_HOME": "/srv/benzinga"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BENZINGA_DATA_DIR` overrides an explicit `--home` for that kind. Use `BENZINGA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BENZINGA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `benzinga-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### analyst

Manage analyst

- **`benzinga-pp-cli analyst`** - Returns analyst insights and research perspectives on securities, including detailed analysis and recommendations from financial analysts

### bars

Manage bars

- **`benzinga-pp-cli bars`** - Retrieves historical OHLCV (Open, High, Low, Close, Volume) price bar data for specified securities. Returns aggregated price data based on the specified interval. Supports multiple ticker symbols and various time ranges including relative dates.

### bulls-bears-say

Manage bulls bears say

- **`benzinga-pp-cli bulls-bears-say`** - Returns the latest bullish and bearish investment cases for a given stock ticker symbol. Bull cases present positive arguments for buying a stock, while bear cases present negative arguments against it.

### calendar

Manage calendar

- **`benzinga-pp-cli calendar get-conference-calls`** - Returns Conference call data for a selected period and/or security. Conference calls are scheduled calls where company management discusses quarterly or annual financial results, business updates, and answers questions from analysts and investors.
- **`benzinga-pp-cli calendar get-dividends`** - Returns dividends data for a selected period and/or security. Includes dividend amounts, ex-dividend dates, payment dates, dividend yields, and other relevant dividend information for stocks.
- **`benzinga-pp-cli calendar get-dividends-v22`** - Returns dividends data for a selected period and/or security, including both confirmed and unconfirmed dividend dates. V2.2 includes additional fields: confirmed, period, and year. This version provides more detailed information about dividend confirmation status and periodicity.
- **`benzinga-pp-cli calendar get-earnings`** - Returns earnings data for a selected period and/or security. Includes actual EPS and revenue figures, estimates, surprises, and historical comparisons. Earnings data is crucial for investors to assess company performance.
- **`benzinga-pp-cli calendar get-economics`** - Returns economic calendar data including economic indicators, releases, and reports from various countries. Includes actual values, consensus estimates, and prior values for economic events such as GDP, employment data, inflation metrics, and more.
- **`benzinga-pp-cli calendar get-events`** - Returns corporate events including investor meetings, conferences, and special announcements
- **`benzinga-pp-cli calendar get-fda`** - Returns FDA approvals, clinical trials, and PDUFA (Prescription Drug User Fee Act) dates for pharmaceutical and biotech companies. Includes information about drug development stages, trial results, approval outcomes, and regulatory milestones.
- **`benzinga-pp-cli calendar get-guidance`** - Returns company guidance data including forward-looking earnings and revenue projections provided by company management. Includes EPS guidance ranges (min/max), revenue guidance ranges, and comparisons to prior guidance. Guidance is crucial for understanding management's expectations for future performance.
- **`benzinga-pp-cli calendar get-ipos-v2`** - Returns Initial Public Offering (IPO) data including pricing information, underwriters, deal status, and offering details. IPOs represent when a private company first offers shares to the public. Note that for the IPOs endpoint, new tickers may not return results right away as they are not automatically linked to the underlying company's data. To obtain the most recent rows, send queries without the tickers parameter specified.
- **`benzinga-pp-cli calendar get-ipos-v21`** - Returns Initial Public Offering (IPO) data including pricing information, underwriters, deal status, and offering details
- **`benzinga-pp-cli calendar get-ma`** - Returns mergers and acquisitions (M&A) data including deal announcements, completions, and details about acquiring and target companies. Includes deal size, payment type, deal status, and expected/completed dates for corporate consolidation activities.
- **`benzinga-pp-cli calendar get-offerings`** - Returns secondary offering data for public companies issuing additional shares after their IPO. Includes offering price, proceeds, number of shares, shelf offerings, and whether securities are sold in portions over time or at the initial offering date.
- **`benzinga-pp-cli calendar get-ratings`** - Returns analyst ratings data including upgrades, downgrades, initiations, and price target changes from Wall Street analysts. Includes current and prior ratings, price targets, analyst information, and ratings accuracy metrics when available.
- **`benzinga-pp-cli calendar get-ratings-analysts`** - Returns the full list of analysts that are providing ratings
- **`benzinga-pp-cli calendar get-ratings-firms`** - Returns the available firms providing analyst ratings
- **`benzinga-pp-cli calendar get-splits`** - Returns stock split data including split ratios, announcement dates, ex-dates, and distribution dates. Stock splits occur when a company increases or decreases the number of outstanding shares to adjust the stock price. Includes information about whether the stock is optionable.

### calendar-removed

Manage calendar removed

- **`benzinga-pp-cli calendar-removed`** - Returns calendar events that have been removed or cancelled from the specified event types

### consensus-ratings

Manage consensus ratings

- **`benzinga-pp-cli consensus-ratings`** - Returns aggregated consensus analyst ratings data for a given ticker symbol. This endpoint provides consensus price targets, aggregate ratings distribution, and analyst counts based on recent analyst ratings.

### erx-gaps

Manage erx gaps

- **`benzinga-pp-cli erx-gaps`** - Returns earnings reaction gap data, which tracks significant price gaps following earnings announcements

### fundamentals

Manage fundamentals

- **`benzinga-pp-cli fundamentals get-alpha-beta-v21`** - Retrieve Alpha and Beta metrics for specified symbols. These metrics indicate volatility and performance relative to the market.
- **`benzinga-pp-cli fundamentals get-asset-classification-v21`** - Retrieve asset classification details for specified symbols, including sector, industry, and other classification metadata. Useful for portfolio categorization and analysis.
- **`benzinga-pp-cli fundamentals get-balance-sheet-v3`** - Retrieve balance sheet data for specified symbols. Includes assets, liabilities, and equity details.
- **`benzinga-pp-cli fundamentals get-company-profile-v21`** - Retrieves comprehensive company profile information including business description, industry classification, sector details, headquarters location, key executives, and other corporate metadata. Essential for understanding company background and organizational structure.
- **`benzinga-pp-cli fundamentals get-company-v21`** - Retrieves detailed company-specific financial data including key metrics, operational statistics, and historical financial performance. Provides a comprehensive view of company financials beyond basic fundamentals.
- **`benzinga-pp-cli fundamentals get-derived-figures-and-ratios-v3`** - Retrieve derived financial figures and ratios for a list of symbols. Includes calculated metrics essential for financial analysis.
- **`benzinga-pp-cli fundamentals get-earning-ratios-v21`** - Retrieve earning ratios for a list of symbols. Includes metrics like P/E ratio, EPS, and other earnings-related ratios.
- **`benzinga-pp-cli fundamentals get-earnings-reports-v21`** - Retrieves detailed earnings reports for specified securities including revenue, earnings per share (EPS), EBITDA, net income, and other key financial results from quarterly and annual reports. Essential for analyzing company financial performance and earnings trends over time.
- **`benzinga-pp-cli fundamentals get-income-statement-v3`** - Retrieves comprehensive income statement data for specified securities. Includes revenue, cost of goods sold, operating expenses, operating income, interest expense, taxes, net income, and earnings per share. Essential for analyzing company profitability and operational performance over time.
- **`benzinga-pp-cli fundamentals get-operation-ratios-v21`** - Retrieve operation ratios for a list of symbols. Includes metrics like operating margin, profit margin, ROA, ROE, and others.
- **`benzinga-pp-cli fundamentals get-share-price-ratios-v3`** - Retrieve share price ratios for specified symbols. Includes metrics like price-to-earnings, price-to-sales, and other price-based ratios.
- **`benzinga-pp-cli fundamentals get-v2`** - Retrieves comprehensive financial fundamentals data for specified securities. Returns key financial metrics, ratios, and company information from MorningStar data sources. Use this endpoint to access income statement, balance sheet, and cash flow statement data.
- **`benzinga-pp-cli fundamentals get-v21`** - Retrieves enhanced financial fundamentals data for specified securities. This is an improved version of the V2 endpoint with additional data fields and better performance. Returns comprehensive financial metrics, ratios, and company information from updated data sources.
- **`benzinga-pp-cli fundamentals get-v3`** - Retrieves the latest generation of financial fundamentals data powered by Benzinga's enhanced data pipeline. Provides comprehensive financial statements, metrics, and ratios with improved data quality and coverage. Supports flexible date range queries and relative date specifications.
- **`benzinga-pp-cli fundamentals get-valuation-ratios-v21`** - Retrieve valuation ratios for a list of symbols. Includes metrics like P/E, P/B, P/S, and other valuation metrics essential for investment analysis.
- **`benzinga-pp-cli fundamentals list`** - Retrieve financial statements for specified symbols. Includes data from balance sheets, income statements, and cash flow statements.
- **`benzinga-pp-cli fundamentals list-cashflow`** - Retrieve detailed cash flow statement data for specified symbols. Includes operating, investing, and financing cash flows.
- **`benzinga-pp-cli fundamentals list-shareclass`** - Retrieve share class information for specific symbols. Returns detailed share structure data including share class IDs, currency, and other related metadata.
- **`benzinga-pp-cli fundamentals list-shareclassprofile`** - Retrieve profile information for share classes, providing details about the share class characteristics and associated metadata.

### gov

Manage gov

- **`benzinga-pp-cli gov get-government-trade-reports`** - Returns detailed government trade disclosure reports including periodic transaction reports filed by congressional members
- **`benzinga-pp-cli gov get-government-trades`** - Returns government official trades including transactions by members of the US House and Senate

### logos

Manage logos

- **`benzinga-pp-cli logos bulk-sync`** - Bulk logos sync. Walks the full logo dataset via page/pagesize (optionally narrowed by updated_since); it is not a per-identifier lookup. To fetch logos for specific securities, use GET /api/v2/logos/search.
- **`benzinga-pp-cli logos get-search`** - Search Logos

### market

Manage market

- **`benzinga-pp-cli market`** - Retrieves market movers data based on specified filters. Returns stocks that have moved significantly during the specified session and time range. Supports custom screener and movers queries for advanced filtering.

### news

Manage news

- **`benzinga-pp-cli news get`** - This REST API returns structured data for news. For optimal performance, limit the scope of the query using parameters such as tickers, date, and channels, or use updatedSince for deltas. Page offsets are limited from 0 - 100000.
- **`benzinga-pp-cli news get-channels`** - Returns a list of all available news channels that can be used to filter news items. Channels can have sub-channels, but they will all be listed as their own item.

### news-quantified

Manage news quantified

- **`benzinga-pp-cli news-quantified`** - Quantitative news analytics with sentiment scores, relevance metrics, and market-impact indicators (per-ticker price moves at 30s-120m horizons, volume ratios, VWAP, support/resistance, analyst and EPS actions). Filter with `--symbols`, `--date`/`--date-from`/`--date-to`, or `--updated-since` for deltas.

### news-removed

Manage news removed

- **`benzinga-pp-cli news-removed`** - Returns the removed news data. Filters the results to only include items that have been updated since the specified timestamp.

### quote-delayed

Manage quote delayed

- **`benzinga-pp-cli quote-delayed get-v1`** - Get delayed quotes for a list of symbols, ISINs, or CIKs
- **`benzinga-pp-cli quote-delayed get-v2`** - Get delayed quotes for a list of symbols

### sec

Manage sec

- **`benzinga-pp-cli sec get-insider-transaction`** - Returns insider transaction data from SEC Form 4 filings. Use /filings endpoint for grouped filing view (transactions nested under each filing) or /transactions endpoint for flattened individual transaction view. Both endpoints support the same query parameters.
- **`benzinga-pp-cli sec get-insider-transaction-owner`** - Returns information about insider transaction owners, including company officers, directors, and beneficial owners

### shortinterest

Manage shortinterest

- **`benzinga-pp-cli shortinterest`** - Retrieves short interest data for specified securities. Includes information about shares sold short, days to cover, and short interest ratios. Supports optional FINRA report data and date range filtering.

### signal

Manage signal

- **`benzinga-pp-cli signal get-block-trade-v1`** - Returns block trade data, which includes unusually large trades that may indicate institutional trading activity
- **`benzinga-pp-cli signal get-halt-resume-v1`** - Returns trading halt and resume information for securities, including halt reasons and expected resumption times
- **`benzinga-pp-cli signal get-option-activity-v1`** - Returns unusual options activity data, including large or unusual options trades that may signal informed trading

### trending-tickers

Manage trending tickers

- **`benzinga-pp-cli trending-tickers get-ticker-trend-data`** - Retrieve trending data for specific tickers, including rank and change. Returns aggregated trend scores and activity levels across different time intervals.
- **`benzinga-pp-cli trending-tickers get-ticker-trend-list-data`** - Retrieve a list of trending tickers based on various metrics. Returns securities ordered by trending score across different time intervals.

### wiims

Manage wiims (Why Is It Moving)

- **`benzinga-pp-cli wiims`** - Why Is It Moving (WIIM): concise, structured explanations of why a security is moving, tagged to the affected tickers. A preset over the news feed's WIIM channel (`GET /api/v2/news?channels=WIIM`). Filter with `--tickers`, `--date`/`--date-from`/`--date-to`, or `--updated-since`. Add `--with-points` to attach each ticker's delayed-quote price move (`price_move: [{ticker, points, percent, last}]`, the day's move in points) via `GET /api/v1/quoteDelayed`. Requires a Benzinga Pro token entitled to the WIIM channel (the market-data tokens are not entitled and return no items); the same Pro token also covers the `--with-points` quote lookup. Complements `why` with editorial context.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
benzinga-pp-cli calendar get-earnings

# JSON for scripting and agents
benzinga-pp-cli calendar get-earnings --json

# Filter to specific fields
benzinga-pp-cli calendar get-earnings --json --select id,name,status

# Dry run — show the request without sending
benzinga-pp-cli calendar get-earnings --dry-run

# Agent mode — JSON + compact + no prompts in one flag
benzinga-pp-cli calendar get-earnings --agent
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
benzinga-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `benzinga-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/benzinga-pp-cli/config.toml`; `--home`, `BENZINGA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BENZINGA_API_KEY` | per_call | No | Set to your API credential. |
| `CALENDAR_API_KEY` | per_call | No | Set to your API credential. |
| `BENZINGA_MARKET_API_KEY` | per_call | No | Optional market/super-token for calendar, movers, bars, short interest, and logos. Falls back to `BENZINGA_API_KEY` when unset. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `benzinga-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `benzinga-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BENZINGA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 403 on an endpoint that other commands hit fine** — Your token works but that product is not in your Benzinga plan; this is per-endpoint licensing, not an error. Use a product your tier covers or contact Benzinga sales.
- **HTTP 401 / auth_failed on every command** — The token is missing or invalid. Set BENZINGA_API_KEY=<key> or run benzinga-pp-cli auth set-token, then benzinga-pp-cli doctor.
- **watch / why / catalysts return nothing** — Those commands read the local store; run benzinga-pp-cli sync --resources ratings,news,earnings,option-activity --since 7d first.
- **Responses feel large or slow** — Narrow with --tickers and --date-from, and use --select to project only the fields you need (the docs recommend narrowing every query).

## Known Gaps

These reflect upstream Benzinga API state observed at build time (2026-06). The CLI issues correct requests, retries 5xx, and surfaces clear errors.

- **Earnings-call transcripts omitted.** The standalone `/api/v1/earnings-call-transcripts*` endpoints returned HTTP 503 ("failure to get a peer from the ring-balancer") across every token at build time — a Benzinga delivery-service outage with no healthy backend — so the `earnings-call-transcripts` commands were not shipped. The transcript *data* is also served from Benzinga's delivery API (`/api/v1/transcripts/calls`), which was healthy; a future build can add that source.
- **Operation ratios use the v2.1 endpoint.** The deprecated `/api/v2/fundamentals/operationRatios` path 500s upstream, so only **`fundamentals get-operation-ratios-v21`** (`/api/v2.1/...`, returns 200) is shipped.
- **Tokens are product-scoped.** A valid token can return 403 on a product not in your plan (a licensing boundary, not a bug). Set `BENZINGA_API_KEY` to a broad token for full coverage; market-data endpoints (bars, movers, quote, short interest, logos) require a market-data-licensed token.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**benzinga-python-client**](https://github.com/Benzinga/benzinga-python-client) — Python (31 stars)
- [**openbb-benzinga**](https://github.com/OpenBB-finance/OpenBB) — Python
- [**go-bztcp**](https://github.com/Benzinga/go-bztcp) — Go
- [**python-bztcp**](https://github.com/Benzinga/python-bztcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
