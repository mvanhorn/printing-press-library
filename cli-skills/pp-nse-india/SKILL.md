---
name: pp-NSE India
description: "Every NSE equity data point in a single Go binary — live quotes, order book, filings, and a local SQLite store no... Trigger phrases: `quote ADANIPORTS`, `check NSE market status`, `show me the NIFTY 50 constituents`, `which stocks are delivering the most today`, `filings for RELIANCE`, `use nse-india`, `run nse-india`."
author: "Mayank Lavania"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nse-india-pp-cli
    install:
      - kind: go
        bins: [nse-india-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/cmd/nse-india-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/nse-india/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# NSE India — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nse-india-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install nse-india --cli-only
   ```
2. Verify: `nse-india-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/cmd/nse-india-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

nse-india-pp-cli fetches real-time NSE equity quotes, index constituents, corporate filings, and market status with just browser headers — no API key, no Python runtime, no setup friction. After sync, the local SQLite store powers cross-symbol analysis that is invisible in any single API call: delivery spikes, sector breadth, portfolio margin health, and index attribution.

## When to Use This CLI

Use nse-india-pp-cli when you need Indian equity data in a script, cron job, or AI agent workflow without Python dependencies. It is the right choice for portfolio monitoring, corporate action alerts, sector rotation analysis, and any workflow that needs to join quote data across multiple symbols or time periods.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`delivery-spike`** — Flags stocks where today's delivery-to-traded ratio is significantly above their 20-session rolling average — early signal of institutional accumulation.

  _Use when you want to detect institutional accumulation before price follows — the delivery spike precedes moves 2-3 sessions earlier than price action alone._

  ```bash
  nse-india-pp-cli delivery-spike --threshold 2.0 --agent
  ```
- **`iep-drift`** — Tracks how accurately each stock's pre-market IEP (Indicative Equilibrium Price) predicts the actual opening price over 30 sessions.

  _Use in pre-market to know which IEP signals are reliable for that day's gap-open strategy._

  ```bash
  nse-india-pp-cli iep-drift --lookback 30 --min-gap 1.5 --agent
  ```
- **`announcement-flood`** — Surfaces companies whose filing cadence has spiked above their own historical baseline — a leading indicator of imminent corporate actions like rights issues or mergers.

  _Use when monitoring for surprise corporate actions — a flood of exchange filings reliably precedes announcements by 3-7 days._

  ```bash
  nse-india-pp-cli announcement-flood --window 7d --threshold 3 --agent
  ```
- **`portfolio pnl`** — Tracks unrealized P&L, daily delta, and drawdown for a personal holdings file against the synced quote history.

  _Use when an agent needs to report portfolio performance or trigger rebalancing alerts without a brokerage integration._

  ```bash
  nse-india-pp-cli portfolio pnl --holdings holdings.csv --agent
  ```
- **`portfolio margin-health`** — Aggregates VaR margin, extreme loss margin, and adhoc margin across a full portfolio — shows total margin-at-risk and which holdings are the biggest margin consumers.

  _Use before market open to check whether overnight VaR changes have pushed margin utilization above a safe threshold._

  ```bash
  nse-india-pp-cli portfolio margin-health --holdings holdings.csv --agent
  ```
- **`delivery-divergence`** — Detects when price is rising but delivery % is falling (distribution signal) or price falling while delivery is rising (accumulation signal) — separates smart money from retail.

  _Use when evaluating whether a trend is sustainable — delivery divergence at a peak is the single clearest distribution signal available from public data._

  ```bash
  nse-india-pp-cli delivery-divergence --lookback 10 --agent
  ```

### Agent-native plumbing
- **`sector-breadth`** — Computes advance/decline ratio, median pChange, and delivery breadth for every constituent of a named index — far richer than the index headline number.

  _Use when deciding whether an index move is broad-based or driven by one or two large caps — changes the conviction of a trade._

  ```bash
  nse-india-pp-cli sector-breadth --sector IT --agent
  ```
- **`index-driver`** — Decomposes any day's index move into per-stock point contributions — identifies the 3-5 stocks driving 80%+ of the index change.

  _Use when the index move looks misleading — tells an agent whether a rally is concentrated (single-stock risk) or genuinely broad._

  ```bash
  nse-india-pp-cli index-driver --index "NIFTY 50" --agent
  ```

## Command Reference

**corporate** — Corporate actions, filings, and financial data

- `nse-india-pp-cli corporate actions` — Corporate actions history — dividends, bonuses, splits, rights with ex-dates and record dates
- `nse-india-pp-cli corporate announcements` — Exchange filings — board meetings, results, AGM, investor meets, disclosure intimations
- `nse-india-pp-cli corporate annual_reports` — Annual reports with direct PDF download URLs, going back multiple years
- `nse-india-pp-cli corporate financial_results` — Quarterly and annual financial result filings with XBRL links
- `nse-india-pp-cli corporate insider_trading` — SEBI PIT (Prohibition of Insider Trading) disclosures — promoter/director buy/sell with quantities, values,...

**equity** — Comprehensive equity quote data for NSE-listed stocks

- `nse-india-pp-cli equity derivatives` — F&O data — futures (3 expiries with OI, volume, turnover) and options contracts (CE/PE by strike and expiry)
- `nse-india-pp-cli equity quote` — Full equity quote — last price, 52w H/L, sector PE, order book (5 bid/ask levels), delivery%, VaR margin, index...

**indices** — NSE index data and constituents

- `nse-india-pp-cli indices constituents` — Live prices for all constituent stocks in an index with 52w range, 1Y/30D% change
- `nse-india-pp-cli indices list` — List all available NSE indices with short and long names

**market** — Market status and operational information

- `nse-india-pp-cli market` — Real-time market status for all segments (Capital Market, Currency, F&O, WDM) with current NIFTY level

**movers** — Market activity rankings

- `nse-india-pp-cli movers` — Most active intraday securities ranked by volume or traded value

**symbol_lookup** — Symbol search and autocomplete

- `nse-india-pp-cli symbol_lookup` — Search symbols by company name or ticker — returns equity, MF, index matches


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nse-india-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find stocks near 52-week extremes from a synced index

```bash
nse-india-pp-cli indices constituents --index "NIFTY 50" --json --agent | jq '.results.data[] | select(.nearWKH < 5)'
```

Filters index constituents within 5% of their 52-week high using live NSE data — no extra command needed

### Daily index attribution report

```bash
nse-india-pp-cli index-driver --index "NIFTY 50" --agent --select top_contributors,concentration_ratio,ex_top3_index_move
```

One-liner for whether today's Nifty move is broad-based or misleadingly narrow

### Sector breadth vs price divergence

```bash
nse-india-pp-cli sector-breadth --sector BANKING --agent | jq '{advance_decline: .advance_decline_ratio, delivery: .delivery_breadth}'
```

When NIFTY BANK is green but breadth is weak, the move is a single-stock story

### Portfolio margin health before market open

```bash
nse-india-pp-cli portfolio margin-health --holdings ~/holdings.csv --agent --select total_margin_required,margin_to_value_pct,highest_risk_holdings
```

Runs in 2 seconds from cached data — safe to add to a pre-market cron

### Announcement flood with delivery confirmation

```bash
nse-india-pp-cli announcement-flood --window 7d --agent | jq '.[] | select(.flood_ratio > 3)' | xargs -I{} nse-india-pp-cli delivery-spike --symbol {} --agent
```

Combines filing flood with delivery data — the pair predicts imminent corporate actions with much higher accuracy than either signal alone

## Auth Setup

No authentication required.

Run `nse-india-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nse-india-pp-cli indices list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
nse-india-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nse-india-pp-cli feedback --stdin < notes.txt
nse-india-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.nse-india-pp-cli/feedback.jsonl`. They are never POSTed unless `NSE_INDIA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NSE_INDIA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nse-india-pp-cli profile save briefing --json
nse-india-pp-cli --profile briefing indices list
nse-india-pp-cli profile list --json
nse-india-pp-cli profile show briefing
nse-india-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nse-india-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/cmd/nse-india-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add nse-india-pp-mcp -- nse-india-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nse-india-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nse-india-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nse-india-pp-cli <command> --help`.
