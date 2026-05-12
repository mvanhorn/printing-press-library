---
name: pp-bls
description: "The first feature-rich CLI for the BLS Public Data API — with offline series search, a release calendar, footnote... Trigger phrases: `fetch BLS series`, `look up a BLS series ID`, `BLS unemployment rate`, `what's the latest CPI`, `U.S. macro snapshot`, `next BLS release`, `decode a BLS footnote`, `use bls`, `run bls`."
author: "Amanda Huang"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bls-pp-cli
    install:
      - kind: go
        bins: [bls-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/bls/cmd/bls-pp-cli
---

# BLS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bls-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install bls --cli-only
   ```
2. Verify: `bls-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bls/cmd/bls-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

BLS publishes hundreds of thousands of labor and economic time series behind packed structural IDs like CUUR0000SA0 and LNS14000000, and the live API has no way to search for them. bls-pp-cli ships a locally-synced series catalog with FTS5 search, a curated U.S. macro snapshot, the release calendar, and footnote decoding — the things every analyst, reporter, and agent needs but no existing wrapper provides. Every command is also an MCP tool, so LLM agents can resolve series IDs and fetch values without a hand-curated dictionary.

## When to Use This CLI

Reach for bls-pp-cli when you need authoritative U.S. labor or price-stability data and you don't want to bounce between FRED, bls.gov data tools, and Python notebooks. The CLI is best for: (1) workflows where series-ID discovery is the bottleneck; (2) cross-survey snapshots that touch multiple BLS surveys at once; (3) agent contexts where every command is also an MCP tool. Skip it if you need exotic vintage/revision history beyond what BLS publishes through the API — only the live values are mirrored.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Discovery the live API can't do

- **`series search`** — Find the right BLS series ID by plain-English title, survey, or area without leaving the terminal.

  _Reach for this when you need the canonical BLS series ID for a concept (any indicator, any area, any survey) before fetching values. Without it, agents either hallucinate IDs or rely on a hand-curated dictionary._

  ```bash
  bls-pp-cli series search "Los Angeles CPI all items" --json
  ```
- **`footnotes decode`** — Decode BLS footnote codes (P, R, C, ...) into plain-English explanations.

  _Reach for this when an observation comes back with footnotes you don't recognize — preliminary vs revised vs corrected matters for the analyst writeup._

  ```bash
  bls-pp-cli footnotes decode P R --json
  ```

### Workflow shortcuts

- **`snapshot macro`** — One command returns the current state of the U.S. macro economy: headline + core CPI, U3, payrolls, JOLTS openings, PPI, ECI, productivity, with YoY and MoM changes.

  _Use when you want a single read of the U.S. labor and price-stability picture without composing a 15-series request yourself._

  ```bash
  bls-pp-cli snapshot macro --csv > macro.csv
  ```
- **`releases next`** — List upcoming BLS releases (CPI, employment situation, JOLTS, PPI...) with date, time, and news-release URL.

  _Use to plan around BLS data drops; combine with --watch to poll until the next print lands._

  ```bash
  bls-pp-cli releases next --within 14d --json
  ```

### Local-cache queries

- **`series extremum`** — Compute max, min, and percentile rank of a series's latest observation across a configurable window from the local cache.

  _Use for release-day writeups, agent tool-calls that need historical context, and quickly answering "have we been here before" without scrolling FRED charts._

  ```bash
  bls-pp-cli series extremum LNS14000000 --since 2005 --json
  ```
- **`series compare-sa`** — Show seasonally-adjusted and not-seasonally-adjusted variants of a series side-by-side.

  _Use when you need to disambiguate whether a trend you're seeing is genuine or a seasonal artifact._

  ```bash
  bls-pp-cli series compare-sa CUUR0000SA0 --json
  ```

## Command Reference

**series** — Fetch BLS time-series observations (CPI, employment, unemployment, JOLTS, PPI, ECI, productivity, and more).

- `bls-pp-cli series batch` — Fetch up to 50 BLS series in one call. Pass --ids comma-separated; the CLI partitions IDs >50 across multiple requests.
- `bls-pp-cli series get` — Fetch a single BLS time-series by ID.
- `bls-pp-cli series popular` — List BLS's most-popular series, optionally filtered by survey.

**surveys** — BLS survey directory (CPI, CES, CPS, JOLTS, PPI, ECI, productivity, and more).

- `bls-pp-cli surveys get` — Show detail for a single BLS survey (allowed calculations, annual averages, etc.).
- `bls-pp-cli surveys list` — List every BLS survey by abbreviation and name.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bls-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Series-ID archaeology, gone

```bash
bls-pp-cli series search "unemployment rate California" --survey LAUS --json --select results.id,results.title,results.units
```

Use --select with dotted paths to narrow the FTS5 response to just the fields an agent or downstream script needs.

### Release-day reaction prep

```bash
bls-pp-cli snapshot macro --json --select id,label,value,yoy_pct
```

Snapshot returns 15 macro indicators with server-side YoY/MoM in one call. Pipe through jq for further shaping.

### Inflation-adjust a price

```bash
bls-pp-cli inflation adjust --amount 100 --from-year 2010 --to-year 2024
```

Local CPI cache means this works offline once you have synced.

### Watch for the next CPI print

```bash
bls-pp-cli releases next --survey CPI --watch --within 24h
```

Polls the local release calendar until the next CPI release window opens.

### Agent-friendly batch fetch

```bash
bls-pp-cli series batch --ids CUUR0000SA0,CUSR0000SA0L1E,LNS14000000,CES0000000001 --start 2020 --end 2025 --agent --select Results.series.seriesID,Results.series.data.year,Results.series.data.value
```

Pull 4 macro indicators in one call with structured agent output and dotted-path field selection.

## Auth Setup

BLS API works without a key (25 queries/day, 3 series per request, 10 years of history). A free registration key from https://data.bls.gov/registrationEngine/ unlocks 500 queries/day, 50 series per request, 20 years, and the calculations / catalog / annual-average flags. Set with `bls-pp-cli auth set-token <key>` or `BLS_API_KEY=<key>` in your environment.

Run `bls-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bls-pp-cli series get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
bls-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bls-pp-cli feedback --stdin < notes.txt
bls-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bls-pp-cli/feedback.jsonl`. They are never POSTed unless `BLS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BLS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bls-pp-cli profile save briefing --json
bls-pp-cli --profile briefing series get mock-value
bls-pp-cli profile list --json
bls-pp-cli profile show briefing
bls-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bls-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/bls/cmd/bls-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bls-pp-mcp -- bls-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bls-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bls-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bls-pp-cli <command> --help`.
