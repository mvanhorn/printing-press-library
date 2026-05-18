---
name: pp-sec-edgar
description: "Every SEC filing, every XBRL fact, every insider trade — synced into a local SQLite store you can pivot, search, and watch offline. Trigger phrases: `look up an SEC filing`, `check insider trading on`, `compare quarterly financials for`, `watch SEC filings for`, `use sec-edgar`."
author: "Chris Drit"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sec-edgar-pp-cli
    install:
      - kind: go
        bins: [sec-edgar-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/sec-edgar/cmd/sec-edgar-pp-cli
---

# SEC EDGAR — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sec-edgar-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install sec-edgar --cli-only
   ```
2. Verify: `sec-edgar-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/sec-edgar/cmd/sec-edgar-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

An agent-native CLI for the entire SEC EDGAR surface — data.sec.gov XBRL, efts.sec.gov full-text search, and the live Atom feed. The synced SQLite store enables joins no single SEC endpoint supports: insider-cluster detection across issuers, XBRL peer-group benchmarks by SIC, 13F holdings deltas across quarters, and live filing watches with multi-dimensional filters. All free — SEC provides no API key, just a mandatory User-Agent header.

## When to Use This CLI

Pick this CLI when an agent needs to query SEC filings, XBRL financial facts, insider trading (Form 4), or institutional holdings (13F) across many companies or many periods — anything that benefits from a local SQLite store rather than a per-call API roundtrip. For one-off single-filing reads, the typed endpoint mirrors (filings get, facts get) work without sync. For cross-company pivots, insider clustering, peer-group benchmarks, or live filing watches, the transcendence commands turn what would be ten API calls + a Python loop into one CLI invocation.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**companies** — Company directory (ticker → CIK → name)

- `sec-edgar-pp-cli companies tickers` — Full ticker-to-CIK map for every public-equity issuer with a listed ticker (~10k entries)
- `sec-edgar-pp-cli companies tickers_exchange` — Ticker-to-CIK map keyed by exchange (Nasdaq, NYSE, etc.)
- `sec-edgar-pp-cli companies tickers_mf` — Mutual fund ticker-to-CIK map (one row per share class)

**facts** — XBRL financial facts (data.sec.gov)

- `sec-edgar-pp-cli facts company` — Get all XBRL facts for a company across every reporting period
- `sec-edgar-pp-cli facts frame` — Get one XBRL concept value reported by every public filer for one period (cross-company slice)
- `sec-edgar-pp-cli facts get` — Get a single XBRL concept time series for one company (one tag across all of its filings)

**submissions** — Filer (company) filing history

- `sec-edgar-pp-cli submissions <cik>` — Get full submissions history for a filer by 10-digit zero-padded CIK


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sec-edgar-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Apple income-statement trend, agent-friendly

```bash
sec-edgar-pp-cli facts statement --cik 0000320193 --kind income --periods last8 --agent --select 'facts.us-gaap.Revenues,facts.us-gaap.OperatingIncomeLoss,facts.us-gaap.NetIncomeLoss'
```

XBRL responses are deeply nested; --select with dotted paths picks only the three concepts the agent needs across the last 8 periods.

### Cluster-buying watchlist scan

```bash
sec-edgar-pp-cli insider-cluster --within 14d --min-insiders 3 --code P --since 90d --json
```

Open-market insider buying (Form 4 code P) clustered across 3+ insiders in 14 days, last 90 days. The store-backed JOIN is what makes this one command.

### Quarterly comp-set revenue pivot

```bash
sec-edgar-pp-cli cross-section --tag us-gaap:Revenues --cik 0000320193,0000789019,0001652044 --periods last8 --json
```

Apple vs Microsoft vs Alphabet revenue, last 8 quarters, as a single pivot. Both axes constrained — neither edgartools nor the frames endpoint covers this shape.

### Watchlist Monday triage

```bash
sec-edgar-pp-cli watchlist items --since 30d --item 2.05,5.02,4.02 --cik 0000320193,0000789019,0001652044 --json --agent
```

Cross-CIK 8-K Item pivot — Maya's exact Monday ritual. The watchlist file is one CIK per line.

### Late-filer compliance scan

```bash
sec-edgar-pp-cli late-filers --since 60d --form 10-K --json
```

Surfaces NT 10-K filings (companies that warned they'd miss their reporting deadline). Single SELECT against the local filings table.

## Auth Setup

SEC EDGAR requires no API key. Every request must include a User-Agent header naming a human and contact email (e.g. 'Alex Researcher alex@example.com'). Set SEC_EDGAR_USER_AGENT in your environment; the CLI refuses to run without it. The SEC enforces a 10-req/sec rate cap per host; this CLI caps itself at 8 req/sec with jitter.

Run `sec-edgar-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sec-edgar-pp-cli facts get --cik example-value --taxonomy example-value --tag example-value --agent --select id,name,status
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
sec-edgar-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sec-edgar-pp-cli feedback --stdin < notes.txt
sec-edgar-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.sec-edgar-pp-cli/feedback.jsonl`. They are never POSTed unless `SEC_EDGAR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SEC_EDGAR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sec-edgar-pp-cli profile save briefing --json
sec-edgar-pp-cli --profile briefing facts get --cik example-value --taxonomy example-value --tag example-value
sec-edgar-pp-cli profile list --json
sec-edgar-pp-cli profile show briefing
sec-edgar-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sec-edgar-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/sec-edgar/cmd/sec-edgar-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sec-edgar-pp-mcp -- sec-edgar-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sec-edgar-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sec-edgar-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sec-edgar-pp-cli <command> --help`.
