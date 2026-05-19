---
name: pp-tennis-warehouse
description: "Every racquet Tennis Warehouse sells — new and used — searchable offline with spec compare, substitute finder,... Trigger phrases: `find a tennis racquet similar to`, `compare these tennis racquets`, `tennis racquet under $`, `used wilson blade`, `racquets with 16x19 string pattern`, `tennis warehouse used inventory`, `use tennis-warehouse`, `run tennis-warehouse`."
author: "blake johnson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tennis-warehouse-pp-cli
---

# Tennis Warehouse — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tennis-warehouse-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install tennis-warehouse --cli-only
   ```
2. Verify: `tennis-warehouse-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/cmd/tennis-warehouse-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Tennis Warehouse has the deepest racquet catalog and used inventory in the U.S., but the web UI is browse-only and the data is locked behind page navigation. This CLI mirrors the entire catalog into a local SQLite store, then exposes the spec-driven and price-driven queries the website cannot answer — `racquets similar <sku>`, `racquets compare <sku> <sku>`, `used deals --min-discount-pct 40`, `used drops --since 7d`, `used new --since 7d`, `used depth --grade A`, `used watch <pcode>`, `used grip-availability --size 4_3/8`.

## When to Use This CLI

Pick this CLI when you (or your agent) need to query the Tennis Warehouse catalog in a way the website itself doesn't support — spec-driven similarity, multi-racquet comparison, used-vs-new discount triage, or price-drop tracking. Use it for pre-purchase research, substitute-finder workflows after a discontinued frame cracks, or to drive a daily "new arrivals + drops" digest. Don't use it for placing orders, demos, or anything that requires a logged-in Tennis Warehouse account.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Spec-driven discovery
- **`racquets similar`** — Find current racquets whose specs match a target SKU within a tolerance band — head size, strung weight, swingweight, and exact string pattern.

  _When a player's racquet cracks or is discontinued, picking a replacement requires triangulating across 9 spec fields. This command does it in one call._

  ```bash
  tennis-warehouse-pp-cli racquets similar WB9810 --tolerance tight --json
  ```
- **`racquets compare`** — Render an aligned spec-by-spec table for 2–5 racquets with diff highlighting; --json emits a row-per-spec matrix.

  _Replaces opening 2–5 browser tabs and copy-pasting specs into a spreadsheet._

  ```bash
  tennis-warehouse-pp-cli racquets compare WB9810 WB9818 --json
  ```
- **`used grip-availability`** — Find used units in a specific grip size grouped by model and grade — wrong grip = unplayable, so this is a hard filter most shoppers care about.

  _Saves the shopper from clicking through every model to see if their grip size is even in stock._

  ```bash
  tennis-warehouse-pp-cli used grip-availability --size 4_3/8 --grade A --brand wilson --json
  ```

### Buying signals
- **`used deals`** — Find used listings whose price is a steep discount versus the new-racquet MSRP — joins used inventory against the new catalog.

  _Surfaces the actual bargain hunt — "40% off new for a Grade A unit" is the buy signal, not "$150 vs unknown."_

  ```bash
  tennis-warehouse-pp-cli used deals --min-discount-pct 40 --grade A --brand wilson --json
  ```
- **`used drops`** — List used listings whose latest price snapshot dropped below a prior snapshot beyond a threshold within a time window.

  _Catches deals the website cannot expose because it stores no historical pricing._

  ```bash
  tennis-warehouse-pp-cli used drops --since 7d --min-drop-pct 10 --json
  ```
- **`used new`** — Used listings whose first_seen_at falls within a recent time window — "what is new since I last looked."

  _Used inventory churns fast and good Grade A units sell in hours — the new-arrival feed is how bargain hunters actually shop._

  ```bash
  tennis-warehouse-pp-cli used new --since 7d --brand babolat --json
  ```
- **`used depth`** — Aggregate per-physical-unit counts grouped by model and condition grade — answers "how many Grade A Blade 98s are in stock right now?"

  _Depth is a buying-confidence signal. One unit at Grade A is a gamble; twelve units is a healthy market._

  ```bash
  tennis-warehouse-pp-cli used depth --min-units 3 --grade A --json
  ```
- **`used watch`** — Save SKUs to a local watchlist; view current state; combine with drops to alert on watched-only items.

  _Lets a player track a small set of candidates without manually refreshing pages._

  ```bash
  tennis-warehouse-pp-cli used watchlist drops --since 30d --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**racquets** — Current (new) racquet catalog across all stocked brands

- `tennis-warehouse-pp-cli racquets` — Fetch the all-racquets landing page (featured + best-sellers across every brand)

**used** — Used racquet inventory — Grade A/B/C and Unused units across all stocked brands

- `tennis-warehouse-pp-cli used get` — Fetch the detail page for a used model (full spec sheet + individual unit listings)
- `tennis-warehouse-pp-cli used list` — List used-racquet models stocked for a brand


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tennis-warehouse-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find a substitute for a cracked racquet

```bash
tennis-warehouse-pp-cli racquets similar WB9810 --tolerance tight --limit 10 --json --select sku,model,head_size,strung_weight,swingweight,string_pattern
```

Given your current racquet's SKU, returns the closest current models by spec — narrows a season-long replacement decision into one query.

### Catch a Grade A bargain on a Wilson Blade 98

```bash
tennis-warehouse-pp-cli used deals --brand wilson --grade A --min-discount-pct 35 --json --select pcode,model,grade,price,msrp,discount_pct
```

Lists Grade A Wilson listings selling 35%+ off new MSRP — the joined-vs-MSRP view the website cannot show.

### Compare three racquet generations side-by-side

```bash
tennis-warehouse-pp-cli racquets compare WB9810 WB9818 WB9816 --json
```

Aligned spec-by-spec table for Blade 98 v10, v8, and v9. Replaces three browser tabs.

### Watch a model and check daily drops

```bash
tennis-warehouse-pp-cli used watch WB9818 && tennis-warehouse-pp-cli used watchlist drops --since 7d --min-drop-pct 5 --json
```

Saves the SKU; lists every Grade A/B/C listing whose price dropped at least 5% in the last week.

### Filter by grip size before anything else

```bash
tennis-warehouse-pp-cli used grip-availability --size 4_3/8 --grade A --json
```

Wrong grip is unplayable — start with grip, then narrow by brand/spec from there.

## Auth Setup

No authentication required.

Run `tennis-warehouse-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tennis-warehouse-pp-cli used list --ccode example-value --agent --select id,name,status
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
tennis-warehouse-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tennis-warehouse-pp-cli feedback --stdin < notes.txt
tennis-warehouse-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.tennis-warehouse-pp-cli/feedback.jsonl`. They are never POSTed unless `TENNIS_WAREHOUSE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TENNIS_WAREHOUSE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tennis-warehouse-pp-cli profile save briefing --json
tennis-warehouse-pp-cli --profile briefing used list --ccode example-value
tennis-warehouse-pp-cli profile list --json
tennis-warehouse-pp-cli profile show briefing
tennis-warehouse-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tennis-warehouse-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add tennis-warehouse-pp-mcp -- tennis-warehouse-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tennis-warehouse-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tennis-warehouse-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tennis-warehouse-pp-cli <command> --help`.
