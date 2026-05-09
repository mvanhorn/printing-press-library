---
name: pp-iqos
description: "Every IQOS product, store, and flavor across 30+ global markets — synced locally, searchable offline. Trigger phrases: `find iqos stores near me`, `what heets flavors are available in`, `compare iqos products between countries`, `check iqos catalog for`, `track iqos price changes`, `use iqos`, `run iqos`."
author: "Nicolas Correa"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - iqos-pp-cli
---

# IQOS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `iqos-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install iqos --cli-only
   ```
2. Verify: `iqos-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/iqos/cmd/iqos-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

iqos.com spans 30+ country markets with different product lineups, store lists, and flavor availability. This CLI syncs the full catalog locally so you can diff markets, track changes over time, find flavors by profile, and locate stores — all without a browser.

## When to Use This CLI

Use this CLI when you need structured data from the IQOS global product catalog without a browser. Best for: comparing market availability, tracking new product launches, finding stores by location, or building a reference dataset of IQOS products across countries.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`diff`** — See which products exist in one country but not another — find market-exclusive devices, flavors, or accessories instantly.

  _Use this when you need to know if a product is available in a specific market before traveling or ordering internationally._

  ```bash
  iqos-pp-cli diff --from us --to gb --json
  ```
- **`changes`** — Detect products added, removed, or changed in price since your last sync — track IQOS catalog evolution over time.

  _Use this to monitor for new product launches or discontinuations in a specific market._

  ```bash
  iqos-pp-cli changes --since 7d --country gb --json
  ```

### Agent-native plumbing
- **`flavors find`** — Search HEETS and TEREA sticks by flavor profile across all markets — find menthol, tobacco, or fruity variants available near you.

  _Use this when helping a user find a specific flavor profile available in their country._

  ```bash
  iqos-pp-cli flavors find --profile menthol --json
  ```
- **`products export`** — Export the complete product catalog for one or all markets as CSV or JSON — aggregate 30+ sitemaps in one command.

  _Use this to build a complete reference dataset of IQOS products across all global markets._

  ```bash
  iqos-pp-cli products export --country all --format csv > iqos-catalog.csv
  ```
- **`stores nearest`** — Find the closest IQOS stores to any GPS coordinate — offline, using geocoordinates extracted from store pages.

  _Use this when an agent needs to recommend an IQOS store without triggering a browser._

  ```bash
  iqos-pp-cli stores nearest --lat 30.27 --lon -97.74 --limit 3 --json
  ```

## Command Reference

**flavors** — HEETS, TEREA, and other tobacco stick flavors

- `iqos-pp-cli flavors` — List consumable sticks filtered by type (heets, terea, levia)

**products** — IQOS product catalog — devices, sticks, accessories, and vaping products

- `iqos-pp-cli products get` — Get product details from its shop page, extracting schema.org JSON-LD
- `iqos-pp-cli products list` — List all products for a market by parsing the product sitemap

**stores** — Physical IQOS retail locations

- `iqos-pp-cli stores get` — Get store details including address, hours, and coordinates from schema.org/Store JSON-LD
- `iqos-pp-cli stores list` — List all stores for a market from the stores sitemap

**support** — IQOS support articles and FAQs

- `iqos-pp-cli support faqs` — Get support FAQ content for a market
- `iqos-pp-cli support troubleshooting` — Get device troubleshooting guides


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
iqos-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find ILUMA devices in the UK

```bash
iqos-pp-cli products list --country gb --category devices --select name,price,url --agent
```

List all UK devices with just name, price, and URL

### Export all HEETS flavors worldwide

```bash
iqos-pp-cli flavors list --type heets --country all --format csv > heets-global.csv
```

Build a global HEETS flavor reference in one command

### Track UK catalog changes

```bash
iqos-pp-cli changes --since 30d --country gb --json | jq '[.[] | select(.type == "added")]'
```

Find products added to the UK in the last month

### Nearest store from coordinates

```bash
iqos-pp-cli stores nearest --lat 51.50 --lon -0.12 --limit 5 --agent
```

Find 5 nearest stores to central London, agent-friendly output

### Cross-market exclusive products

```bash
iqos-pp-cli diff --from us --to gb --select name,category --agent
```

Products available in GB but not US, structured for agents

## Auth Setup

No authentication required.

Run `iqos-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  iqos-pp-cli flavors --country example-value --lang example-value --agent --select id,name,status
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
iqos-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
iqos-pp-cli feedback --stdin < notes.txt
iqos-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.iqos-pp-cli/feedback.jsonl`. They are never POSTed unless `IQOS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IQOS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
iqos-pp-cli profile save briefing --json
iqos-pp-cli --profile briefing flavors --country example-value --lang example-value
iqos-pp-cli profile list --json
iqos-pp-cli profile show briefing
iqos-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `iqos-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add iqos-pp-mcp -- iqos-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which iqos-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   iqos-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `iqos-pp-cli <command> --help`.
