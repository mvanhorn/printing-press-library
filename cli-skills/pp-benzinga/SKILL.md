---
name: pp-benzinga
description: "Every Benzinga calendar, news, fundamentals, and signal endpoint as a typed command — plus an offline SQLite store, full-text search, cross-entity queries, and the first Benzinga MCP server. Trigger phrases: `benzinga analyst ratings for AAPL`, `what changed on my watchlist`, `why is NVDA moving`, `this week's earnings calendar`, `unusual options activity`, `economic calendar this week`, `use benzinga`, `run benzinga`."
author: "waveriderai"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - benzinga-pp-cli
    install:
      - kind: go
        bins: [benzinga-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/benzinga/cmd/benzinga-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/benzinga/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Benzinga — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `benzinga-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install benzinga --cli-only
   ```
2. Verify: `benzinga-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/benzinga/cmd/benzinga-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Benzinga's licensed financial-data API is powerful but fragmented across ~60 endpoints and has only one complete client — a Python library with no CLI, no offline store, and no agent surface. This CLI covers the full documented REST surface as first-class commands, delta-syncs the calendar/news/signal families into a local database via the API's own updated cursors, and adds cross-entity commands the REST API cannot express in one call: watch a ticker set for overnight changes, explain why one name is moving, and rank analysts by accuracy.

## When to Use This CLI

Use this CLI when an agent or user needs Benzinga financial data — analyst ratings, earnings/economic/dividend calendars, breaking news, fundamentals, unusual options and other signals — as structured JSON, or when they want cross-entity views (overnight watchlist changes, a single-ticker catalyst timeline, analyst accuracy) that no single Benzinga endpoint provides. It is also the right choice for offline full-text search over synced news and calendar data.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for real-time tick/quote streaming or Squawk audio — it covers REST + delayed data, not the WebSocket/TCP live feeds.
- Do not use it to place trades or manage a brokerage account — Benzinga is data-only.
- Do not use it for tickers or products outside your Benzinga license; gated endpoints return 403 by design.
- Do not use it as a general stock-price API for unlimited free quotes — every endpoint requires a paid Benzinga token.

## Unique Capabilities

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

## Command Reference

**analyst** — Manage analyst

- `benzinga-pp-cli analyst` — Returns analyst insights and research perspectives on securities

**bars** — Manage bars

- `benzinga-pp-cli bars` — Retrieves historical OHLCV (Open, High, Low, Close, Volume) price bar data for specified securities.

**bulls-bears-say** — Manage bulls bears say

- `benzinga-pp-cli bulls-bears-say` — Returns the latest bullish and bearish investment cases for a given stock ticker symbol.

**calendar** — Manage calendar

- `benzinga-pp-cli calendar get-conference-calls` — Returns Conference call data for a selected period and/or security.
- `benzinga-pp-cli calendar get-dividends` — Returns dividends data for a selected period and/or security.
- `benzinga-pp-cli calendar get-dividends-v22` — Returns dividends data for a selected period and/or security, including both confirmed and unconfirmed dividend dates.
- `benzinga-pp-cli calendar get-earnings` — Returns earnings data for a selected period and/or security.
- `benzinga-pp-cli calendar get-economics` — Returns economic calendar data including economic indicators, releases, and reports from various countries.
- `benzinga-pp-cli calendar get-events` — Returns corporate events including investor meetings, conferences, and special announcements
- `benzinga-pp-cli calendar get-fda` — Returns FDA approvals, clinical trials, and PDUFA (Prescription Drug User Fee Act)
- `benzinga-pp-cli calendar get-guidance` — Returns company guidance data including forward-looking earnings and revenue projections provided by company management.
- `benzinga-pp-cli calendar get-ipos-v2` — Returns Initial Public Offering (IPO) data including pricing information, underwriters, deal status
- `benzinga-pp-cli calendar get-ipos-v21` — Returns Initial Public Offering (IPO) data including pricing information, underwriters, deal status
- `benzinga-pp-cli calendar get-ma` — Returns mergers and acquisitions (M&A) data including deal announcements, completions
- `benzinga-pp-cli calendar get-offerings` — Returns secondary offering data for public companies issuing additional shares after their IPO.
- `benzinga-pp-cli calendar get-ratings` — Returns analyst ratings data including upgrades, downgrades, initiations
- `benzinga-pp-cli calendar get-ratings-analysts` — Returns the full list of analysts that are providing ratings
- `benzinga-pp-cli calendar get-ratings-firms` — Returns the available firms providing analyst ratings
- `benzinga-pp-cli calendar get-splits` — Returns stock split data including split ratios, announcement dates, ex-dates, and distribution dates.

**calendar-removed** — Manage calendar removed

- `benzinga-pp-cli calendar-removed` — Returns calendar events that have been removed or cancelled from the specified event types

**consensus-ratings** — Manage consensus ratings

- `benzinga-pp-cli consensus-ratings` — Returns aggregated consensus analyst ratings data for a given ticker symbol.

**erx-gaps** — Manage erx gaps

- `benzinga-pp-cli erx-gaps` — Returns earnings reaction gap data, which tracks significant price gaps following earnings announcements

**fundamentals** — Manage fundamentals

- `benzinga-pp-cli fundamentals get-alpha-beta-v21` — Retrieve Alpha and Beta metrics for specified symbols.
- `benzinga-pp-cli fundamentals get-asset-classification-v21` — Retrieve asset classification details for specified symbols, including sector, industry
- `benzinga-pp-cli fundamentals get-balance-sheet-v3` — Retrieve balance sheet data for specified symbols. Includes assets, liabilities, and equity details.
- `benzinga-pp-cli fundamentals get-company-profile-v21` — Retrieves comprehensive company profile information including business description, industry classification
- `benzinga-pp-cli fundamentals get-company-v21` — Retrieves detailed company-specific financial data including key metrics, operational statistics
- `benzinga-pp-cli fundamentals get-derived-figures-and-ratios-v3` — Retrieve derived financial figures and ratios for a list of symbols.
- `benzinga-pp-cli fundamentals get-earning-ratios-v21` — Retrieve earning ratios for a list of symbols. Includes metrics like P/E ratio, EPS, and other earnings-related ratios.
- `benzinga-pp-cli fundamentals get-earnings-reports-v21` — Retrieves detailed earnings reports for specified securities including revenue, earnings per share (EPS), EBITDA
- `benzinga-pp-cli fundamentals get-income-statement-v3` — Retrieves comprehensive income statement data for specified securities.
- `benzinga-pp-cli fundamentals get-operation-ratios-v21` — Retrieve operation ratios for a list of symbols.
- `benzinga-pp-cli fundamentals get-share-price-ratios-v3` — Retrieve share price ratios for specified symbols.
- `benzinga-pp-cli fundamentals get-v2` — Retrieves comprehensive financial fundamentals data for specified securities.
- `benzinga-pp-cli fundamentals get-v21` — Retrieves enhanced financial fundamentals data for specified securities.
- `benzinga-pp-cli fundamentals get-v3` — Retrieves the latest generation of financial fundamentals data powered by Benzinga's enhanced data pipeline.
- `benzinga-pp-cli fundamentals get-valuation-ratios-v21` — Retrieve valuation ratios for a list of symbols.
- `benzinga-pp-cli fundamentals list` — Retrieve financial statements for specified symbols.
- `benzinga-pp-cli fundamentals list-cashflow` — Retrieve detailed cash flow statement data for specified symbols.
- `benzinga-pp-cli fundamentals list-shareclass` — Retrieve share class information for specific symbols.
- `benzinga-pp-cli fundamentals list-shareclassprofile` — Retrieve profile information for share classes

**gov** — Manage gov

- `benzinga-pp-cli gov get-government-trade-reports` — Returns detailed government trade disclosure reports including periodic transaction reports filed by congressional
- `benzinga-pp-cli gov get-government-trades` — Returns government official trades including transactions by members of the US House and Senate

**logos** — Manage logos

- `benzinga-pp-cli logos bulk-sync` — Bulk logos sync.
- `benzinga-pp-cli logos get-search` — Search Logos

**market** — Manage market

- `benzinga-pp-cli market` — Retrieves market movers data based on specified filters.

**news** — Manage news

- `benzinga-pp-cli news get` — This REST API returns structured data for news.
- `benzinga-pp-cli news get-channels` — Returns a list of all available news channels that can be used to filter news items.

**news-quantified** — Manage news quantified

- `benzinga-pp-cli news-quantified` — Quantitative news analytics: sentiment, volume ratios, and per-ticker price reactions (30s–120m). Filter with `--symbols`, `--date`/`--date-from`/`--date-to`, `--updated-since`.

**news-removed** — Manage news removed

- `benzinga-pp-cli news-removed` — Returns the removed news data.

**quote-delayed** — Manage quote delayed

- `benzinga-pp-cli quote-delayed get-v1` — Get delayed quotes for a list of symbols, ISINs, or CIKs
- `benzinga-pp-cli quote-delayed get-v2` — Get delayed quotes for a list of symbols

**search** — Offline full-text search

- `benzinga-pp-cli search "<query>"` — Search synced local data (or the logos search API in live/auto mode). Local query tokens are alphanumeric only (letters, digits, underscore); every token is required (implicit AND). FTS5 operators such as AND, OR, NOT and quoted phrases are not supported.

**sec** — Manage sec

- `benzinga-pp-cli sec get-insider-transaction` — Returns insider transaction data from SEC Form 4 filings.
- `benzinga-pp-cli sec get-insider-transaction-owner` — Returns information about insider transaction owners, including company officers, directors, and beneficial owners

**shortinterest** — Manage shortinterest

- `benzinga-pp-cli shortinterest` — Retrieves short interest data for specified securities.

**signal** — Manage signal

- `benzinga-pp-cli signal get-block-trade-v1` — Returns block trade data, which includes unusually large trades that may indicate institutional trading activity
- `benzinga-pp-cli signal get-halt-resume-v1` — Returns trading halt and resume information for securities, including halt reasons and expected resumption times
- `benzinga-pp-cli signal get-option-activity-v1` — Returns unusual options activity data, including large or unusual options trades that may signal informed trading

**trending-tickers** — Manage trending tickers

- `benzinga-pp-cli trending-tickers get-ticker-trend-data` — Retrieve trending data for specific tickers, including rank and change.
- `benzinga-pp-cli trending-tickers get-ticker-trend-list-data` — Retrieve a list of trending tickers based on various metrics.

**wiims** — Manage wiims (Why Is It Moving)

- `benzinga-pp-cli wiims` — Why Is It Moving (WIIM): concise structured explanations of why a security is moving, tagged to the affected tickers. Preset over the news feed's WIIM channel (`GET /api/v2/news?channels=WIIM`). Filter with `--tickers`, `--date`/`--date-from`/`--date-to`, `--updated-since`. Add `--with-points` to attach each ticker's delayed-quote move (`price_move: [{ticker, points, percent, last}]`) via `/api/v1/quoteDelayed`. Requires a WIIM-entitled Benzinga Pro token (market-data tokens return nothing); the same token covers `--with-points`. Complements `why` with editorial context.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
benzinga-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Benzinga uses a static API token passed as a query parameter. Set BENZINGA_API_KEY in your environment (or run benzinga-pp-cli auth set-token) and every command attaches it as ?token=. Access is tier-gated per product, so a valid token can still return 403 on an endpoint your plan does not include — that is a licensing boundary, not a CLI bug.

Run `benzinga-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  benzinga-pp-cli calendar get-earnings --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `BENZINGA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `BENZINGA_CONFIG_DIR`, `BENZINGA_DATA_DIR`, `BENZINGA_STATE_DIR`, `BENZINGA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `BENZINGA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `benzinga-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `BENZINGA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `BENZINGA_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
benzinga-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
benzinga-pp-cli feedback --stdin < notes.txt
benzinga-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `BENZINGA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BENZINGA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
benzinga-pp-cli profile save briefing --json
benzinga-pp-cli --profile briefing calendar get-earnings
benzinga-pp-cli profile list --json
benzinga-pp-cli profile show briefing
benzinga-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `benzinga-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/benzinga/cmd/benzinga-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add benzinga-pp-mcp -- benzinga-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which benzinga-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   benzinga-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `benzinga-pp-cli <command> --help`.
