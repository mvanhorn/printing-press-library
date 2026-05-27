# BSE Filings CLI

**Mirrors BSE corporate filings into a local SQLite store and answers the cross-holding, tone-drift, and calendar questions the BSE site cannot.**

BSE Filings syncs every tracked holding's announcements, results, board meetings, and concall transcripts into a local FTS5 database. It turns eight quarters of concall language into a greppable, drift-detectable signal across an entire portfolio — concall-grep finds a phrase across every holding, thesis-drift shows when management's language is shifting, and due-soon merges two BSE calendars the site keeps apart. Built for IMstockbox Council bots as a subprocess and for an operator at the terminal.

Learn more at [BSE Filings](https://www.bseindia.com/).

## Install

The recommended path installs both the `bse-filings-pp-cli` binary and the `pp-bse-filings` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install bse-filings
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install bse-filings --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/bse-filings/cmd/bse-filings-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bse-filings-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bse-filings --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bse-filings --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-bse-filings skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-bse-filings. The skill defines how its required CLI can be installed.
```

## Authentication

No API key or login. BSE's JSON API is reachable with browser Referer and User-Agent headers, which the client sets automatically. If a request 301-redirects to a members page, the headers were stripped — re-run with the bundled client, not a bare curl.

## Quick Start

```bash
# see the seeded holding universe (scrip codes) before syncing
bse-filings-pp-cli holdings list


# pull every filing for every holding since the last cursor into ~/.bse-filings/filings.db
bse-filings-pp-cli sync


# which holdings have results or board meetings in the next week
bse-filings-pp-cli due-soon --within 7d


# find the phrase across every synced concall
bse-filings-pp-cli concall-grep "margin pressure" --since 90d


# watch how RELIANCE's guidance language moved over four quarters
bse-filings-pp-cli thesis-drift 500325 --terms margin,demand,debt --last 4

```

## Unique Features

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

## Usage

Run `bse-filings-pp-cli --help` for the full command reference and flag list.

## Commands

### announcements

Corporate announcements / filings feed (per scrip). The core syncable resource.

- **`bse-filings-pp-cli announcements list`** - List corporate announcements for a scrip within a date window, optionally filtered by category.

### corp_actions

Forthcoming corporate actions — board meetings, AGM/EGM, dividends, ex-dates.

- **`bse-filings-pp-cli corp_actions list`** - List forthcoming corporate actions (board meetings, AGM, dividends) for a scrip or segment.

### quote

Latest OHLC quote / scrip header data.

- **`bse-filings-pp-cli quote get`** - Fetch the latest OHLC quote and header data for a scrip.

### results_calendar

Forthcoming results calendar — companies scheduled to report.

- **`bse-filings-pp-cli results_calendar list`** - List forthcoming result announcements, optionally scoped to a scrip or date window.

### results_snapshot

Quarterly financial-results snapshot numbers for a scrip.

- **`bse-filings-pp-cli results_snapshot get`** - Fetch the quarterly financial-results snapshot (revenue, profit, etc.) for a scrip.

### scrips

Scrip-code lookup by company name, symbol, or ISIN.

- **`bse-filings-pp-cli scrips search`** - Resolve a company name, ticker symbol, or ISIN to its BSE scrip code.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bse-filings-pp-cli announcements

# JSON for scripting and agents
bse-filings-pp-cli announcements --json

# Filter to specific fields
bse-filings-pp-cli announcements --json --select id,name,status

# Dry run — show the request without sending
bse-filings-pp-cli announcements --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bse-filings-pp-cli announcements --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-bse-filings -g
```

Then invoke `/pp-bse-filings <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/other/bse-filings/cmd/bse-filings-pp-mcp@latest
```

Then register it:

```bash
claude mcp add bse-filings bse-filings-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bse-filings-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/bse-filings/cmd/bse-filings-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bse-filings": {
      "command": "bse-filings-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
bse-filings-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/bse-filings-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Command returns empty and you expected filings** — Run `bse-filings-pp-cli sync` first — concall-grep, cross, stale, and sql read the local store, which is empty until sync populates it.
- **Sync returns nothing for a scrip** — Confirm the scrip code with `bse-filings-pp-cli holdings list`; BSE keys on numeric scrip_code, not the ticker symbol.
- **A request 301-redirects to a members page** — The Referer/User-Agent headers were stripped; use the CLI's own client (it sets them) rather than a bare HTTP call.
- **A concall command shows an empty transcript** — That PDF is a scanned image; the parse path flags it as needing OCR and skips it rather than crashing.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**BseIndiaApi (bse)**](https://github.com/BennyThadikaran/BseIndiaApi) — Python
- [**awesome-stock-skills**](https://github.com/samyakjain0606/awesome-stock-skills) — Markdown
- [**nse-bse-mcp**](https://github.com/bshada/nse-bse-mcp) — TypeScript
- [**finstack-mcp**](https://github.com/finstacklabs/finstack-mcp) — Python
- [**Indian-Stock-Exchange-MCP**](https://github.com/anuragkrishna/Indian-Stock-Exchange-MCP) — Python
- [**bsedata**](https://github.com/sdabhi23/bsedata) — Python
- [**bseindia**](https://github.com/RuchiTanmay/bseindia) — Python
- [**Live-NSE-BSE-MCP**](https://github.com/GirishKumarDV/Live-NSE-BSE-MCP) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
