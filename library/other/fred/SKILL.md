---
name: pp-fred
description: "Every FRED endpoint, plus a local SQLite store, offline search, and macro commands no other FRED tool has. Trigger phrases: `what's the unemployment rate`, `pull CPI from FRED`, `fred series UNRATE`, `macro dashboard`, `compare GDP and inflation`, `use fred`, `run fred`."
author: "Luke J"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - fred-pp-cli
    install:
      - kind: go
        bins: [fred-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/fred/cmd/fred-pp-cli
---

# FRED — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `fred-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install fred --cli-only
   ```
2. Verify: `fred-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/fred/cmd/fred-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search and pull U.S. and global economic time series from the St. Louis Fed's FRED API. Beyond mirroring every endpoint, it adds a macro dashboard, multi-series compare, a latest-value shortcut, a persistent watchlist that reports what changed, and a release calendar — all agent-native with --json and --select.

## When to Use This CLI

Use this CLI when an agent or analyst needs U.S. or global macroeconomic time series — unemployment, inflation, GDP, interest rates, and tens of thousands more — programmatically. It is ideal for fetching current indicator values, pulling historical series for analysis, comparing indicators, and tracking a watchlist over time.

## Anti-triggers

Do not use this CLI for:
- Company financials or stock prices — use an equities/SEC tool.
- FRED's GeoFRED/Maps regional-choropleth API — this CLI covers the core time-series API.
- Writing or publishing data — FRED is read-only.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Aggregation only we do
- **`dashboard`** — Latest value of a curated set of headline U.S. indicators (unemployment, CPI, GDP, fed funds, 10Y, payrolls) in one call.

  _Reach for this when an agent needs a quick read on the U.S. economy without choosing and fetching individual series IDs._

  ```bash
  fred-pp-cli dashboard --json
  ```
- **`series compare`** — Pull observations for multiple series and align them by date into one table or JSON for correlation.

  _Use when comparing or correlating two or more indicators over the same window._

  ```bash
  fred-pp-cli series compare UNRATE CPIAUCSL --start 2020-01-01 --json
  ```

### Agent-native shortcuts
- **`series latest`** — The single most recent observation (date + value) for a series, as a one-liner.

  _Use when you only need the current print of an indicator, not its history._

  ```bash
  fred-pp-cli series latest UNRATE --json
  ```

### Local state that compounds
- **`watchlist sync`** — Persist a set of series locally, sync their latest observations into SQLite, and report which ones moved since the last sync.

  _Use to track a personal set of indicators and surface only the ones that changed._

  ```bash
  fred-pp-cli watchlist sync --json
  ```
- **`release calendar`** — Recent and upcoming economic data release dates within a day window, aggregated across all releases.

  _Use to see what economic data is dropping soon without scanning hundreds of releases._

  ```bash
  fred-pp-cli release calendar --days 7 --json
  ```

## Command Reference

**category** — Browse the FRED category tree

- `fred-pp-cli category children` — List child categories under a category
- `fred-pp-cli category get` — Get a category by ID (root category is 0)
- `fred-pp-cli category related` — List categories related to a category
- `fred-pp-cli category series` — List the series within a category
- `fred-pp-cli category tags` — List tags for the series in a category

**release** — Data releases and their schedules

- `fred-pp-cli release dates` — List release dates for all releases (the economic release calendar)
- `fred-pp-cli release get` — Get a single release by ID
- `fred-pp-cli release list` — List all data releases on FRED
- `fred-pp-cli release release-dates` — List release dates for a single release
- `fred-pp-cli release series` — List the series in a release
- `fred-pp-cli release sources` — List the sources for a release

**series** — Economic data series — search, metadata, and observations

- `fred-pp-cli series categories` — List the categories a series belongs to
- `fred-pp-cli series get` — Get metadata for a single series by ID (e.g. UNRATE, GDP, CPIAUCSL)
- `fred-pp-cli series observations` — Pull the observation values (the actual time series) for a series
- `fred-pp-cli series release` — Get the release that a series belongs to
- `fred-pp-cli series search` — Search for series by full-text query (e.g. 'unemployment rate')
- `fred-pp-cli series tags` — List FRED tags attached to a series
- `fred-pp-cli series updates` — List series recently updated on FRED
- `fred-pp-cli series vintagedates` — List real-time vintage dates for a series (when data was revised)

**source** — Sources of economic data

- `fred-pp-cli source get` — Get a single source by ID
- `fred-pp-cli source list` — List all sources of economic data on FRED
- `fred-pp-cli source releases` — List the releases for a source

**tags** — Discover series via FRED tags

- `fred-pp-cli tags list` — List FRED tags, optionally filtered
- `fred-pp-cli tags related` — List tags related to a set of tags
- `fred-pp-cli tags series` — List series matching a set of tags


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
fred-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Current unemployment rate

```bash
fred-pp-cli series latest UNRATE --json
```

One-liner for the most recent print of any indicator.

### Year-over-year CPI inflation

```bash
fred-pp-cli series observations CPIAUCSL --units pc1 --sort-order desc --limit 12 --json --select observations.date,observations.value
```

Uses FRED's pc1 transform for YoY percent change and --select to trim the payload.

### Compare unemployment and inflation

```bash
fred-pp-cli series compare UNRATE CPIAUCSL --start 2020-01-01 --json
```

Aligns multiple series by date into one structure for correlation.

### This week's data releases

```bash
fred-pp-cli release calendar --days 7 --json
```

Windowed view of upcoming and recent economic releases.

## Auth Setup
Run `fred-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export FRED_API_KEY="<your-key>"
```

Or persist it in `~/.config/fred-pp-cli/config.toml`.

Run `fred-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  fred-pp-cli category get 0 --agent --select id,name,status
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
fred-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
fred-pp-cli feedback --stdin < notes.txt
fred-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/fred-pp-cli/feedback.jsonl`. They are never POSTed unless `FRED_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FRED_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
fred-pp-cli profile save briefing --json
fred-pp-cli --profile briefing category get 0
fred-pp-cli profile list --json
fred-pp-cli profile show briefing
fred-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `fred-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/fred/cmd/fred-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add fred-pp-mcp -- fred-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which fred-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   fred-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `fred-pp-cli <command> --help`.
