---
name: pp-bse-filings
description: "Mirrors BSE corporate filings into a local SQLite store and answers the cross-holding, tone-drift, and calendar... Trigger phrases: `search BSE concalls for a phrase`, `what's due this week for my holdings`, `thesis drift for RELIANCE`, `which holdings filed critical news`, `beat or miss this quarter`, `use bse-filings`, `run bse-filings`."
author: "Rushyant M"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bse-filings-pp-cli
    install:
      - kind: go
        bins: [bse-filings-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/bse-filings/cmd/bse-filings-pp-cli
---

# BSE Filings — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bse-filings-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install bse-filings --cli-only
   ```
2. Verify: `bse-filings-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/bse-filings/cmd/bse-filings-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

BSE Filings syncs every tracked holding's announcements, results, board meetings, and concall transcripts into a local FTS5 database. It turns eight quarters of concall language into a greppable, drift-detectable signal across an entire portfolio — concall-grep finds a phrase across every holding, thesis-drift shows when management's language is shifting, and due-soon merges two BSE calendars the site keeps apart. Built for IMstockbox Council bots as a subprocess and for an operator at the terminal.

## When to Use This CLI

Reach for BSE Filings when an agent or operator needs to ask compound questions across an Indian-equity portfolio that the BSE website answers one company at a time: searching concall transcripts for a phrase, detecting tone drift across quarters, finding sector-wide language shifts, or merging the results and board-meeting calendars. It is the right tool for earnings-season triage and thesis-decay monitoring, not for live quotes or order placement.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Portfolio-wide signal
- **`concall-grep`** — Full-text search a phrase across every concall transcript in your portfolio and get the scrip, paragraph, and filing date back.

  _Reach for this when an agent needs the exact sentence management used about a theme across many companies, not a 40-page PDF._

  ```bash
  bse-filings-pp-cli concall-grep "margin pressure" --since 90d
  ```
- **`cross`** — Find a phrase appearing across two or more holdings in one quarter, grouped by sector — a sector-wide shift detector.

  _Pick this when the question is sector-level: which companies all started saying the same thing this quarter._

  ```bash
  bse-filings-pp-cli cross "rural recovery" --holdings-only --since 60d
  ```
- **`critical`** — Every holding that filed a Regulation-30 critical-news (material) disclosure in the last N days, in one call.

  _Pick this for material-disclosure triage across the whole portfolio at once._

  ```bash
  bse-filings-pp-cli critical --days 7
  ```

### Thesis decay
- **`thesis-drift`** — Per-quarter frequency of guidance keywords (margin, demand, debt, guidance verbs) across a company's last N concalls, showing which terms are rising or falling.

  _Use this to detect thesis decay — when management's language shifts before the numbers do._

  ```bash
  bse-filings-pp-cli thesis-drift 500325 --terms margin,demand,debt --last 4
  ```
- **`outcomes`** — Beat/miss tagging for results filings, joining the detailed-financials numbers to the headline outcome filing on quarter.

  _Reach for this after a results wave to line up reported numbers against what management said._

  ```bash
  bse-filings-pp-cli outcomes --quarter Q4FY26 --beat
  ```

### Calendar & silence
- **`due-soon`** — Holdings with results, board meetings, or AGM due in the next N days, merged from two BSE endpoints into one list.

  _Use before an earnings week to see every holding with a calendar event coming up._

  ```bash
  bse-filings-pp-cli due-soon --within 7d --kind results,board
  ```
- **`stale`** — Holdings with no filing activity in N days — silence as a signal.

  _Use to surface holdings that have gone unusually quiet, which can precede a surprise._

  ```bash
  bse-filings-pp-cli stale --no-filing-since 90d
  ```

### Transcript plumbing
- **`concall`** — Fetch, parse, and store a single concall PDF, then print only the paragraphs matching a phrase instead of the whole transcript.

  _Use when a bot needs the relevant passage of a call without ingesting the full PDF._

  ```bash
  bse-filings-pp-cli concall 500325 --quarter Q4FY26 --mentions capex
  ```

## Command Reference

**announcements** — Corporate announcements / filings feed (per scrip). The core syncable resource.

- `bse-filings-pp-cli announcements` — List corporate announcements for a scrip within a date window, optionally filtered by category.

**corp_actions** — Forthcoming corporate actions — board meetings, AGM/EGM, dividends, ex-dates.

- `bse-filings-pp-cli corp_actions` — List forthcoming corporate actions (board meetings, AGM, dividends) for a scrip or segment.

**quote** — Latest OHLC quote / scrip header data.

- `bse-filings-pp-cli quote <scripcode>` — Fetch the latest OHLC quote and header data for a scrip.

**results_calendar** — Forthcoming results calendar — companies scheduled to report.

- `bse-filings-pp-cli results_calendar` — List forthcoming result announcements, optionally scoped to a scrip or date window.

**results_snapshot** — Quarterly financial-results snapshot numbers for a scrip.

- `bse-filings-pp-cli results_snapshot <scripcode>` — Fetch the quarterly financial-results snapshot (revenue, profit, etc.) for a scrip.

**scrips** — Scrip-code lookup by company name, symbol, or ISIN.

- `bse-filings-pp-cli scrips <text>` — Resolve a company name, ticker symbol, or ISIN to its BSE scrip code.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bse-filings-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Earnings-week triage

```bash
bse-filings-pp-cli due-soon --within 7d --kind results,board
```

Lists holdings with a results or board-meeting event in the next seven days, merged from both BSE calendars.

### Thesis-decay check before a Council call

```bash
bse-filings-pp-cli thesis-drift 532540 --terms demand,pricing,guidance --last 4
```

Shows the per-quarter frequency of guidance terms across TCS's last four concalls so a softening narrative is visible.

### Sector-wide language shift

```bash
bse-filings-pp-cli cross "input cost" --min-holdings 2 --since 60d
```

Surfaces the same phrase appearing across two or more holdings in the window — an emerging sector theme.

### Agent-narrowed announcements feed

```bash
bse-filings-pp-cli critical --days 14 --agent --select scrip_name,title,filed_at,attachment_url
```

Returns only the four high-gravity fields per critical filing so an agent does not parse the full record.

### Power-user SQL over the mirror

```bash
bse-filings-pp-cli sql "SELECT scrip_name, COUNT(*) c FROM filings GROUP BY scrip_name ORDER BY c DESC"
```

Direct read-only SQLite query against the local filings mirror for ad-hoc aggregation.

## Auth Setup

No authentication required.

Run `bse-filings-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bse-filings-pp-cli announcements --agent --select id,name,status
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
bse-filings-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bse-filings-pp-cli feedback --stdin < notes.txt
bse-filings-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bse-filings-pp-cli/feedback.jsonl`. They are never POSTed unless `BSE_FILINGS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BSE_FILINGS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bse-filings-pp-cli profile save briefing --json
bse-filings-pp-cli --profile briefing announcements
bse-filings-pp-cli profile list --json
bse-filings-pp-cli profile show briefing
bse-filings-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bse-filings-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/bse-filings/cmd/bse-filings-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bse-filings-pp-mcp -- bse-filings-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bse-filings-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bse-filings-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bse-filings-pp-cli <command> --help`.
