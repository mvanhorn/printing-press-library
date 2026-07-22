---
name: pp-autotempest
description: "Every AutoTempest car-search source in your terminal, with a local store, cross-source VIN dedupe Trigger phrases: `search used cars`, `find a honda civic near me`, `compare car prices across sites`, `what cars dropped in price`, `cheapest source for this VIN`, `use autotempest`, `run autotempest`."
author: "richardadonnell"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - autotempest-pp-cli
    install:
      - kind: go
        bins: [autotempest-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/autotempest/cmd/autotempest-pp-cli
---

# AutoTempest — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `autotempest-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install autotempest --cli-only
   ```
2. Verify: `autotempest-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/autotempest/cmd/autotempest-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

AutoTempest already unifies the major used-car marketplaces into one search, but only in a browser. This CLI hits the same aggregated search over plain HTTP, persists listings to a local SQLite store, dedupes the same VIN across sources, and tracks price drops over time. Commands like drops, dedupe, deal, and spread turn one-shot browsing into queryable, compounding car-search state for shoppers and agents.

## When to Use This CLI

Use this CLI to search used-car listings across every AutoTempest source from the terminal or an agent, persist them locally, and answer questions the website cannot: what dropped in price, which source lists a VIN cheapest, and whether a price is good versus comparable cars. It is ideal for recurring searches an agent polls and for piping car data as JSON into downstream workflows.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to buy, bid on, finance, or contact a seller; it is read-only car search.
- Do not use it for new-car MSRP or dealer-invoice pricing; it surfaces existing used-car listings only.
- Do not use it for VIN history or accident reports; it reports listing data, not Carfax-style records.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`drops`** — Surface listings whose price fell since a prior find or watch run, biggest drop first.

  _Reach for this when an agent or shopper needs price movement over time, not a fresh listing dump._

  ```bash
  autotempest-pp-cli drops "civic-fl" --since 7d --min-drop 500 --agent
  ```
- **`watch`** — Register named searches with their filters, then replay them through run so drops and diff have snapshots to compare.

  _Reach for this to set up recurring searches an agent can poll; one-off queries use search instead._

  ```bash
  autotempest-pp-cli watch run --agent
  ```

### Cross-source intelligence
- **`dedupe`** — Collapse the same physical VIN listed on multiple marketplaces into one row with every source and price, cheapest first.

  _Use when the same car appears on eBay and Cars.com and a dealer feed and you need the cheapest source for that exact VIN._

  ```bash
  autotempest-pp-cli dedupe --select vin,min_price,sources.source,sources.price --agent
  ```
- **`deal`** — Rank listings by mechanical price delta from the median of comparable cars (same model, year, mileage band) in your local store.

  _Pick this to answer 'is this price actually good vs comparable cars' with a number, not an opinion._

  ```bash
  autotempest-pp-cli deal "Camry" --select title,price,deal_score --agent
  ```
- **`spread`** — Report min, median, and max price per marketplace for a model so you see which sources run cheap or expensive.

  _Use when deciding which marketplace to shop for a given model._

  ```bash
  autotempest-pp-cli spread "F-150" --agent
  ```
- **`auctions`** — Filter to eBay auction listings with live current bid and bid count, sortable by bid.

  _Use when hunting auction listings rather than fixed-price inventory._

  ```bash
  autotempest-pp-cli auctions --select title,current_bid,bids,url --agent
  ```

## Command Reference

**makes** — List vehicle makes AutoTempest recognizes (slug + display name).

- `autotempest-pp-cli makes` — List makes. Use the returned slug (e.g. 'honda') as the --make value for find.

**models** — List models for a given make (slug + display name + year range).

- `autotempest-pp-cli models <make>` — List models for a make. Pass the make slug (e.g. 'honda'); use the returned model slug as --model for find.

**sources** — List the AutoTempest search sources and their kind.

- `autotempest-pp-cli sources` — Two kinds: **inline** sources (`te`, `hem`, `cs`, `cv`, `cm`, `eb`, `ot`) return parsed per-car listings; **link** sources (`fbm` = Facebook Marketplace, `st` = SearchTempest / Craigslist) are comparison-link-only because those sites block scraping or require login. `find` defaults to the 7 inline sources; pass `--sites fbm,st` to also get their comparison URLs in the `comparison_links` array.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
autotempest-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Clean-title AWD under budget near me

```bash
autotempest-pp-cli find "subaru outback" --zip 33701 --radius 150 --max-price 28000 --title clean --drive awd --json --select title,price_cents,mileage,location,sitecode
```

Narrows a deeply nested multi-source live result to the fields that matter using --select.

### Cheapest source for a specific VIN

```bash
autotempest-pp-cli dedupe --select vin,min_price,sources.source,sources.price --json
```

Shows each physical car once with every marketplace and price it appears at.

### What dropped in price this week

```bash
autotempest-pp-cli drops "my-search" --since 7d --min-drop 500 --json
```

Compares local price snapshots to surface motivated sellers.

### Is this price good

```bash
autotempest-pp-cli deal "tacoma" --select title,price,deal_score --json
```

Scores each listing against the median of comparable cars in your store.

## Auth Setup

No authentication required.

Run `autotempest-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  autotempest-pp-cli makes --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — find/sync commands can use the local SQLite store when available
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
autotempest-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
autotempest-pp-cli feedback --stdin < notes.txt
autotempest-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/autotempest-pp-cli/feedback.jsonl`. They are never POSTed unless `AUTOTEMPEST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AUTOTEMPEST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
autotempest-pp-cli profile save briefing --json
autotempest-pp-cli --profile briefing makes
autotempest-pp-cli profile list --json
autotempest-pp-cli profile show briefing
autotempest-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `autotempest-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/autotempest/cmd/autotempest-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add autotempest-pp-mcp -- autotempest-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which autotempest-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   autotempest-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `autotempest-pp-cli <command> --help`.
