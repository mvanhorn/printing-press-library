---
name: pp-loopnet
description: "LoopNet shows you today — this CLI remembers, building the price-cut, days-on-market, and supply trends LoopNet... Trigger phrases: `search loopnet for commercial real estate`, `find industrial properties for sale in`, `track price cuts on loopnet`, `what is the cap rate distribution in`, `screen for distressed commercial properties`, `use loopnet`, `run loopnet-pp-cli`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - loopnet-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/loopnet/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# LoopNet — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `loopnet-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install loopnet --cli-only
   ```
2. Verify: `loopnet-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/loopnet/cmd/loopnet-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or analyst needs commercial real estate inventory, pricing, or market-intelligence data from LoopNet in a structured, scriptable form. It is the right choice for building a CRE data pipeline, tracking a submarket over time, screening for distressed or mispriced assets, or assembling comp sets. It is not for residential rentals (use Apartments.com) and it cannot access CoStar's subscriber-only analytics.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Change tracking only the local store enables
- **`price-cuts`** — Surfaces every synced listing whose asking price dropped between syncs, with old price, new price, percent cut, and days on market at the cut.

  _Reach for this to find motivated sellers and re-priced assets — a price cut is the single strongest deal-sentiment signal LoopNet itself never exposes._

  ```bash
  loopnet-pp-cli price-cuts worcester-ma --type industrial-properties --agent
  ```
- **`dom`** — Computes true days on market for every live listing from the date the CLI first saw it, and flags aged inventory past a threshold.

  _Use this to tell a fresh comp from a stale one and to find listings that have languished — both invisible on LoopNet._

  ```bash
  loopnet-pp-cli dom worcester-ma --min-days 90 --agent
  ```
- **`velocity`** — Reports absorption for a submarket: new listings, delistings, median days on market, and net supply change per period.

  _Reach for this to gauge whether a submarket is heating up or cooling — the supply-and-demand pulse for a market-intelligence brief._

  ```bash
  loopnet-pp-cli velocity worcester-ma --agent
  ```
- **`delisted`** — Lists listings present in a prior sync but absent now — sold, withdrawn, or expired.

  _Use this to track which assets cleared the market — a proxy for transaction velocity LoopNet never publishes._

  ```bash
  loopnet-pp-cli delisted worcester-ma --since 30d --agent
  ```

### Pricing, yield and distress intelligence
- **`caprate`** — Reports the cap-rate, NOI, and price-per-square-foot distribution (count, min, median, quartiles, max) for synced for-sale listings in a submarket, and flags listings whose cap rate falls outside the interquartile range.

  _Use this to benchmark a single asset's yield against its submarket and spot mispriced cap rates._

  ```bash
  loopnet-pp-cli caprate worcester-ma --type industrial-properties --agent
  ```
- **`distress`** — Flags listings carrying motivation signals: price-reduced and must-sell keyword hits in the description, Ten-X auction listings, and recent price cuts.

  _Reach for this to find distressed and motivated-seller assets in one sweep instead of reading every listing's free text._

  ```bash
  loopnet-pp-cli distress worcester-ma --agent
  ```

### Analyst and pipeline workflows
- **`digest`** — Rolls a synced submarket into one report: live supply count, recent price cuts, median days on market, new and delisted counts, and distress hits.

  _Reach for this for a one-command market-intelligence snapshot of a submarket instead of running five separate analysis commands._

  ```bash
  loopnet-pp-cli digest worcester-ma --type industrial-properties --agent
  ```
- **`feed`** — Exports the latest synced submarket as a run-stamped JSON or CSV file, with records mapped to the six CRE market-intelligence data categories.

  _Reach for this to drop LoopNet data straight into a downstream CRE pipeline (e.g. a data/raw ingest folder) without glue code._

  ```bash
  loopnet-pp-cli feed worcester-ma --format csv --out ./loopnet-worcester.csv
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 3 total network entries
- Protocols: ssr_embedded_data (85% confidence)
- Generation hints: requires_protected_client

## Command Reference

**inventory** — Search LoopNet commercial real estate inventory by location, property type, and sale/lease.

- `loopnet-pp-cli inventory <location> [--type <property_type>] [--listing for-sale|for-lease]` — Search LoopNet listings for a location, property type, and sale-or-lease. Returns a server-rendered results page...

**property** — Fetch the full detail record for a single LoopNet listing.

- `loopnet-pp-cli property <id>` — Fetch one LoopNet listing's detail page. The page carries a schema.org RealEstateListing/Product JSON-LD block plus...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
loopnet-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Seed and screen a submarket

```bash
loopnet-pp-cli sync worcester-ma --type industrial --listing for-sale && loopnet-pp-cli caprate worcester-ma --type industrial --agent
```

Pull a submarket into the store, then read its cap-rate and yield distribution in one pass.

### Narrow a verbose detail record for an agent

```bash
loopnet-pp-cli property 38523625 --agent --select name,price,cap_rate,total_assessment,broker_name
```

Detail records carry tens of fields; --select returns only the address, price, cap rate, assessment, and broker so an agent does not burn context on the full payload.

### Find every price cut this month

```bash
loopnet-pp-cli price-cuts worcester-ma --since 30d --agent
```

After repeated syncs, list listings whose asking price dropped in the last 30 days — the core deal-sentiment signal.

### Export a market for a downstream pipeline

```bash
loopnet-pp-cli feed worcester-ma --type industrial --format csv --out ./loopnet-worcester.csv
```

Write the latest synced submarket as a run-stamped CSV ready for a CRE market-intelligence ingest folder.

### Roll up a submarket in one command

```bash
loopnet-pp-cli digest worcester-ma --type industrial-properties --agent
```

Supply, recent price cuts, median days on market, new and delisted counts, and distress hits for a submarket in a single report.

## Auth Setup

No API key is required, but LoopNet's data pages are protected by Akamai Bot
Manager. Live commands (`inventory`, `property`, `sync`) need short-lived
clearance cookies:

- `loopnet-pp-cli auth refresh` — mint cookies (briefly drives a browser).
- `loopnet-pp-cli auth set --cookies "<header>"` — paste a Cookie header copied from browser DevTools.
- `loopnet-pp-cli auth status` — check the current cookie state.

Cookies last a few hours; refresh when a fetch fails with an Akamai
bot-challenge (HTTP 403) error. The history commands (`price-cuts`, `dom`,
`velocity`, `delisted`, `digest`, `caprate`) read the local SQLite store and
need no cookies — but need a prior `sync` to have populated it.

Run `loopnet-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  loopnet-pp-cli inventory worcester-ma --type industrial --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response shape

Under `--json` each command emits a flat object shaped for its own task —
there is no shared envelope. List-style commands carry a `results` array plus
a `count`; for example `search`:

```json
{
  "count": 12,
  "query": "warehouse",
  "results": [ ... ]
}
```

`sync` returns a summary object (`listings_synced`, `properties_synced`,
`total_results`, ...); the intelligence commands (`caprate`, `price-cuts`,
`digest`, ...) return their own report objects. Inspect a command's `--json`
output directly rather than assuming a common wrapper key.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
loopnet-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
loopnet-pp-cli feedback --stdin < notes.txt
loopnet-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.loopnet-pp-cli/feedback.jsonl`. They are never POSTed unless `LOOPNET_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LOOPNET_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
loopnet-pp-cli profile save briefing --json
loopnet-pp-cli --profile briefing inventory worcester-ma --type industrial
loopnet-pp-cli profile list --json
loopnet-pp-cli profile show briefing
loopnet-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `loopnet-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add loopnet-pp-mcp -- loopnet-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which loopnet-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   loopnet-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `loopnet-pp-cli <command> --help`.
