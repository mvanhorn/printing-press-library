# SEC EDGAR CLI

**Every SEC filing, every XBRL fact, every insider trade — synced into a local SQLite store you can pivot, search, and watch offline.**

An agent-native CLI for the entire SEC EDGAR surface — data.sec.gov XBRL, efts.sec.gov full-text search, and the live Atom feed. The synced SQLite store enables joins no single SEC endpoint supports: insider-cluster detection across issuers, XBRL peer-group benchmarks by SIC, 13F holdings deltas across quarters, and live filing watches with multi-dimensional filters. All free — SEC provides no API key, just a mandatory User-Agent header.

## Install

The recommended path installs both the `sec-edgar-pp-cli` binary and the `pp-sec-edgar` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install sec-edgar
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install sec-edgar --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/sec-edgar/cmd/sec-edgar-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sec-edgar-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sec-edgar --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sec-edgar --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-sec-edgar skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-sec-edgar. The skill defines how its required CLI can be installed.
```

## Authentication

SEC EDGAR requires no API key. Every request must include a User-Agent header naming a human and contact email (e.g. 'Alex Researcher alex@example.com'). Set SEC_EDGAR_USER_AGENT in your environment; the CLI refuses to run without it. The SEC enforces a 10-req/sec rate cap per host; this CLI caps itself at 8 req/sec with jitter.

## Quick Start

```bash
# Confirm the User-Agent is set and SEC hosts are reachable.
sec-edgar-pp-cli doctor


# Resolve a ticker to its CIK from the local company-ticker map.
sec-edgar-pp-cli companies lookup AAPL --json


# Apple's last five 10-Ks from the synced submissions store.
sec-edgar-pp-cli filings list --cik 0000320193 --form 10-K --limit 5 --json


# Apple's last four quarters of income-statement XBRL facts.
sec-edgar-pp-cli facts statement --cik 0000320193 --kind income --periods last4 --json


# Flag issuers with 3+ insiders selling in any 5-day window over the last 30 days.
sec-edgar-pp-cli insider-cluster --within 5d --min-insiders 3 --code S --since 30d --json


# Stream live filings matching multi-dim filters as NDJSON.
sec-edgar-pp-cli watch --form 8-K --item 2.05 --keyword 'going concern' --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watchlist items`** — Across a saved watchlist of CIKs, surface 8-Ks filed in the window grouped by Item code (2.05 restructuring, 5.02 exec change, 4.02 non-reliance).

  _Equity analysts triage filings by Item code, not by company. Pick this when an agent needs cross-company 8-K filtering by Item._

  ```bash
  sec-edgar-pp-cli watchlist items --since 30d --item 2.05,5.02,4.02 --cik 0000320193,0000789019,0001652044 --json
  ```
- **`insider-cluster`** — Flag issuers where 3+ distinct insiders filed Form 4 with the same transaction code (open-market buy or sale) within a rolling N-day window.

  _Clustered insider selling is a textbook forensic signal. Pick this when an agent needs derived signals over raw Form 4 data._

  ```bash
  sec-edgar-pp-cli insider-cluster --within 5d --min-insiders 3 --code S --since 90d --json
  ```
- **`industry-bench`** — Take an XBRL concept for one reporting period, group by SIC code, and emit percentile statistics (p10/p50/p90) across every public-company filer in that SIC.

  _Quant researchers and equity analysts need peer-group benchmarks. Pick this when an agent should answer 'how does X compare to its industry peers'._

  ```bash
  sec-edgar-pp-cli industry-bench --tag us-gaap:Revenues --period CY2024Q4 --sic 7372 --stat p50,p90 --json
  ```
- **`cross-section`** — Pivot a single XBRL concept across an explicit company list and the last N reporting periods. Output as a wide pivot table (one row per company, one column per period).

  _Comp-set analysis is one of the highest-frequency analyst rituals. Pick this when an agent compares fundamentals across an explicit ticker list._

  ```bash
  sec-edgar-pp-cli cross-section --tag us-gaap:Revenues --ticker AAPL,MSFT,GOOGL --periods last8 --json
  ```
- **`holdings delta`** — Diff an institutional investor's 13F holdings across two consecutive quarters. Categorize each issuer as ADD / EXIT / INCREASE / DECREASE with share-count delta.

  _Tracking institutional money flow is a classic factor signal. Pick this when an agent compares a fund's positioning across quarters._

  ```bash
  sec-edgar-pp-cli holdings delta --filer-cik 0001067983 --period 2024Q4 --vs 2024Q3 --json
  ```

### Agent-native plumbing
- **`watch`** — Stream the SEC Atom getcurrent feed with multi-dimensional filtering: form type, 8-K item, CIK watchlist, and keyword regex. Emits one NDJSON line per match.

  _Agents that monitor for specific filing events should subscribe to this stream rather than polling submissions per CIK._

  ```bash
  sec-edgar-pp-cli watch --form 8-K --item 2.05 --cik 0000320193,0000789019 --keyword 'going concern' --one-shot --json
  ```

### SEC-specific signals
- **`restatements`** — Surface 8-K Item 4.02 (non-reliance on prior financials) plus all 10-K/A and 10-Q/A amendments filed in the window — the textbook accounting-irregularity signal.

  _Forensic accounting agents look for restatements as a leading indicator of trouble. Pick this when an agent needs accounting-quality signals._

  ```bash
  sec-edgar-pp-cli restatements --since 90d --json
  ```
- **`late-filers`** — Find issuers that filed an NT 10-K, NT 10-Q, or NT 20-F in the window — the SEC's 'I'm going to miss my reporting deadline' notification.

  _Late filings are leading indicators of operational or accounting trouble. Pick this when an agent needs to flag at-risk companies._

  ```bash
  sec-edgar-pp-cli late-filers --since 90d --form 10-K --json
  ```

## Usage

Run `sec-edgar-pp-cli --help` for the full command reference and flag list.

## Commands

### companies

Company directory (ticker → CIK → name)

- **`sec-edgar-pp-cli companies tickers`** - Full ticker-to-CIK map for every public-equity issuer with a listed ticker (~10k entries)
- **`sec-edgar-pp-cli companies tickers_exchange`** - Ticker-to-CIK map keyed by exchange (Nasdaq, NYSE, etc.)
- **`sec-edgar-pp-cli companies tickers_mf`** - Mutual fund ticker-to-CIK map (one row per share class)

### facts

XBRL financial facts (data.sec.gov)

- **`sec-edgar-pp-cli facts company`** - Get all XBRL facts for a company across every reporting period
- **`sec-edgar-pp-cli facts frame`** - Get one XBRL concept value reported by every public filer for one period (cross-company slice)
- **`sec-edgar-pp-cli facts get`** - Get a single XBRL concept time series for one company (one tag across all of its filings)

### submissions

Filer (company) filing history

- **`sec-edgar-pp-cli submissions get`** - Get full submissions history for a filer by 10-digit zero-padded CIK


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sec-edgar-pp-cli facts get --cik example-value --taxonomy example-value --tag example-value

# JSON for scripting and agents
sec-edgar-pp-cli facts get --cik example-value --taxonomy example-value --tag example-value --json

# Filter to specific fields
sec-edgar-pp-cli facts get --cik example-value --taxonomy example-value --tag example-value --json --select id,name,status

# Dry run — show the request without sending
sec-edgar-pp-cli facts get --cik example-value --taxonomy example-value --tag example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sec-edgar-pp-cli facts get --cik example-value --taxonomy example-value --tag example-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-sec-edgar -g
```

Then invoke `/pp-sec-edgar <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/sec-edgar/cmd/sec-edgar-pp-mcp@latest
```

Then register it:

```bash
claude mcp add sec-edgar sec-edgar-pp-mcp -e SEC_EDGAR_USER_AGENT=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sec-edgar-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SEC_EDGAR_USER_AGENT` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/sec-edgar/cmd/sec-edgar-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sec-edgar": {
      "command": "sec-edgar-pp-mcp",
      "env": {
        "SEC_EDGAR_USER_AGENT": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
sec-edgar-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/sec-edgar-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SEC_EDGAR_USER_AGENT` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sec-edgar-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SEC_EDGAR_USER_AGENT`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 403 from data.sec.gov** — The User-Agent header is missing or generic. Set SEC_EDGAR_USER_AGENT='Your Name your@email.com' and retry.
- **HTTP 429 (rate-limited)** — Reduce concurrency. The CLI caps at 8 req/sec by default; --concurrency 4 halves it. Sync commands resume from cursor.
- **CIK not found** — CIKs are 10 digits with leading zeros. Use 'companies lookup <ticker>' to resolve; the CLI also accepts un-padded CIKs and pads them.
- **Empty 'facts company' output for a known filer** — Not all filers report XBRL (e.g. older filings, foreign private issuers using 20-F). 'filings list' shows what they did file.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**edgartools**](https://github.com/dgunning/edgartools) — Python
- [**sec-edgar-mcp**](https://github.com/stefanoamorelli/sec-edgar-mcp) — Python
- [**secedgar**](https://github.com/sec-edgar/sec-edgar) — Python
- [**sec-api-python**](https://github.com/janlukasschroeder/sec-api-python) — Python
- [**sec-edgar-toolkit**](https://github.com/stefanoamorelli/sec-edgar-toolkit) — TypeScript
- [**bellingcat-edgar**](https://github.com/bellingcat/EDGAR) — Python
- [**tumarkin-edgar**](https://github.com/tumarkin/edgar) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
