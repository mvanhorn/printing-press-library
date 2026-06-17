---
name: pp-world-bank
description: "Every World Bank indicator, queryable offline, with cross-country SQL and rankings no wrapper has. Trigger phrases: `world bank gdp for`, `compare gdp across countries`, `find world bank indicator`, `rank countries by`, `development indicators for`, `use world-bank`, `run world-bank`."
author: "Luke J"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - world-bank-pp-cli
    install:
      - kind: go
        bins: [world-bank-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/world-bank/cmd/world-bank-pp-cli
---

# World Bank — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `world-bank-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install world-bank --cli-only
   ```
2. Verify: `world-bank-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/world-bank/cmd/world-bank-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A single-binary CLI for the World Bank Open Data API. Mirrors the ~16,000-indicator catalog and observations into local SQLite for offline search, then adds cross-country compare, rankings, trend stats, and pipeline exports the Python wrappers and MCP shims don't offer. No API key — World Bank is fully public.

## When to Use This CLI

Reach for this CLI when an agent or script needs World Bank development indicators: GDP, population, emissions, poverty, health, education time-series; cross-country comparisons; or rankings. It is ideal for data-pipeline extracts and offline indicator discovery.

## Anti-triggers

Do not use this CLI for:
- Real-time market/financial prices (use a markets API)
- US-specific Federal Reserve series (use FRED)
- Sub-national or city-level data not in World Bank indicators

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Offline catalog
- **`indicators find`** — Search the full ~16,000-indicator catalog by keyword in a single call.

  _Agents discover the exact indicator code before fetching values, in one local call._

  ```bash
  world-bank-pp-cli indicators find "co2 emissions" --json
  ```

### Local joins
- **`compare`** — Line up one indicator across many countries in a single aligned table with deltas vs a baseline.

  _One call answers 'how do these economies compare' without N separate fetches._

  ```bash
  world-bank-pp-cli compare NY.GDP.MKTP.CD USA,CHN,IND --date 2024 --agent
  ```
- **`trend`** — CAGR, year-over-year change, min/max/latest for one country+indicator series.

  _Agents get the trajectory summary directly instead of parsing decades of rows._

  ```bash
  world-bank-pp-cli trend USA NY.GDP.MKTP.CD --window 10 --agent
  ```
- **`rank`** — Rank all economies by an indicator for a year, filtered by region or income level.

  _Find leaders/laggards on any indicator in one call._

  ```bash
  world-bank-pp-cli rank NY.GDP.PCAP.CD --year 2024 --top 10 --income HIC --agent
  ```

### Data pipeline
- **`export`** — Bulk country x indicator pull emitted as pipeline-ready wide or long CSV.

  _Drops straight into notebooks, sheets, and ETL without reshaping._

  ```bash
  world-bank-pp-cli export USA,CHN NY.GDP.MKTP.CD,SP.POP.TOTL --wide --csv
  ```
- **`data diff`** — Compare a fresh pull against the last synced snapshot to surface revised observations.

  _Detect World Bank data revisions between vintages automatically._

  ```bash
  world-bank-pp-cli data diff USA NY.GDP.MKTP.CD --agent
  ```

## Command Reference

**countries** — Economies (countries and aggregates) with region, income level, lending type

- `world-bank-pp-cli countries get` — Get one economy by ISO code (e.g. USA, BRA)
- `world-bank-pp-cli countries list` — List economies

**data** — Indicator observations (the time-series). Polished output via hand-authored fetch.

- `world-bank-pp-cli data <country> <indicator>` — Fetch indicator observations for one or more countries (raw envelope)

**income_levels** — Income level classifications

- `world-bank-pp-cli income-levels` — List income levels

**indicators** — The ~16,000 indicator catalog (series definitions)

- `world-bank-pp-cli indicators get` — Get indicator metadata by code (e.g. NY.GDP.MKTP.CD)
- `world-bank-pp-cli indicators list` — List indicators

**lending_types** — Lending type classifications

- `world-bank-pp-cli lending-types` — List lending types

**regions** — Regions

- `world-bank-pp-cli regions` — List regions

**sources** — Data sources (e.g. World Development Indicators)

- `world-bank-pp-cli sources` — List data sources

**topics** — Topics (e.g. Health, Education) and their indicators

- `world-bank-pp-cli topics indicators` — List indicators under a topic
- `world-bank-pp-cli topics list` — List topics


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
world-bank-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find then fetch

```bash
world-bank-pp-cli indicators find "life expectancy" --json --select id,name
```

Discover the code, then pass it to data fetch.

### Compact cross-country GDP

```bash
world-bank-pp-cli compare NY.GDP.MKTP.CD USA,CHN,IND,DEU --date 2024 --agent --select country.value,date,value
```

Narrow a deeply nested response to just the fields that matter.

### Top economies by GDP per capita

```bash
world-bank-pp-cli rank NY.GDP.PCAP.CD --year 2024 --top 10 --income HIC
```

Rank and filter in one call.

### Pipeline export

```bash
world-bank-pp-cli export USA,CHN NY.GDP.MKTP.CD,SP.POP.TOTL --wide --csv
```

Wide CSV ready for a notebook.

## Auth Setup

No authentication required.

Run `world-bank-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  world-bank-pp-cli countries list --agent --select id,name,status
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
world-bank-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
world-bank-pp-cli feedback --stdin < notes.txt
world-bank-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/world-bank-pp-cli/feedback.jsonl`. They are never POSTed unless `WORLD_BANK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WORLD_BANK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
world-bank-pp-cli profile save briefing --json
world-bank-pp-cli --profile briefing countries list
world-bank-pp-cli profile list --json
world-bank-pp-cli profile show briefing
world-bank-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `world-bank-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/world-bank/cmd/world-bank-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add world-bank-pp-mcp -- world-bank-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which world-bank-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   world-bank-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `world-bank-pp-cli <command> --help`.
