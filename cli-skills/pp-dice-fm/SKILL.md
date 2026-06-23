---
name: pp-dice-fm
description: "Every ticket, fan, and pound of revenue from your DICE events — queryable, exportable, and joinable across shows. Trigger phrases: `door list for tonight's show`, `export opted-in fans from DICE`, `revenue report from DICE events`, `find repeat buyers on DICE`, `ticket velocity for my DICE event`, `sync my DICE data`, `use dice-fm`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dice-fm-pp-cli
    install:
      - kind: go
        bins: [dice-fm-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/cmd/dice-fm-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/dice-fm/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# DICE — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dice-fm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install dice-fm --cli-only
   ```
2. Verify: `dice-fm-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/cmd/dice-fm-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need to query your DICE event data programmatically — building door lists before shows, generating financial reports, segmenting fan lists for Mailchimp, or identifying repeat buyers across your event history. It is the right tool for any workflow that requires cross-event aggregation or data that the DICE dashboard cannot combine in one view.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Access management
- **`door list`** — Generate a valid-ticket-holder list for any event, with transferred tickets showing the new holder's name — ready for door access management.

  _Use this before every show to get the definitive 'who can enter' list including all transfers and minus all returns._

  ```bash
  dice-fm-pp-cli door list --event RXZlbnQ6MTIzNDU= --json
  ```

### Financial intelligence
- **`revenue summary`** — Aggregate gross revenue, Dice fees, and net earnings per event or across a date range — ready for CFO reports.

  _Use this for weekly financial reporting without manually totaling per-event dashboards in a spreadsheet._

  ```bash
  dice-fm-pp-cli revenue summary --from 2026-01-01 --json
  ```
- **`velocity show`** — Show cumulative ticket sales by day or hour relative to the on-sale date — see whether an event is tracking fast or slow.

  _Use within the first 72 hours after an on-sale to decide whether an event needs promotional push._

  ```bash
  dice-fm-pp-cli velocity show --event RXZlbnQ6MTIzNDU= --bucket day --json
  ```
- **`returns anomalies`** — Flag events with unusually high refund rates — a pricing or marketing signal that deserves immediate attention.

  _Use after sales close each week to surface events that may have pricing problems before the show date._

  ```bash
  dice-fm-pp-cli returns anomalies --threshold 0.08 --agent
  ```

### Audience intelligence
- **`fans repeat`** — Find fans who bought tickets to two or more of your events — your most loyal audience, ready for VIP outreach.

  _Use weekly to build re-engagement lists before announcing new events to warm audiences first._

  ```bash
  dice-fm-pp-cli fans repeat --min-events 2 --since 2026-01-01 --csv
  ```
- **`fans optin`** — Export opted-in fan contacts filtered by city or country — CSV ready for Mailchimp, no dashboard exports needed.

  _Use every Monday to build targeted email lists from the previous week's ticket buyers without touching the Dice dashboard._

  ```bash
  dice-fm-pp-cli fans optin --event RXZlbnQ6MTIzNDU= --country GB --city London --csv
  ```
- **`fans top`** — Rank ticket buyers by total spend for an event or across all events — your VIP list for comps, upgrades, and sponsor decks.

  _Use before each show to identify high-value fans for VIP treatment, and before sponsor meetings to demonstrate audience quality._

  ```bash
  dice-fm-pp-cli fans top --event RXZlbnQ6MTIzNDU= --n 20 --json
  ```

### Inventory & catalog intelligence
- **`capacity`** — Roll up sold-vs-capacity headroom across every event from the local store; `capacity pools` breaks one event down by ticket pool (pool-sum vs event total).

  ```bash
  dice-fm-pp-cli capacity --limit 20 --select event_name,sold,capacity,remaining,pct_sold
  ```
- **`tier-performance`** — Rank price tiers by redemptions and each tier's share of total sales from the local store.

  ```bash
  dice-fm-pp-cli tier-performance --limit 20 --json
  ```
- **`normalize`** — Canonicalize free-text ticket-type and venue names into structured axes (parallel, re-runnable, local-only); `normalize recommend` emits a starter config, `normalize stats` shows coverage, and `normalize promote-rules` learns reusable regex rules from manual classifications.

  ```bash
  dice-fm-pp-cli normalize --tiers --fuzzy
  ```

## Command Reference

**events** — Events on your DICE account (scheduling, state, venues, ticket types)

- `dice-fm-pp-cli events get` — Get a single event by ID
- `dice-fm-pp-cli events list` — List your events

**extras** — Extras and add-ons sold with tickets

- `dice-fm-pp-cli extras` — List extras (filter by event)

**genres** — Event genre types and their child genres

- `dice-fm-pp-cli genres` — List genre types

**orders** — Ticket purchase orders with financial and geographic data

- `dice-fm-pp-cli orders` — List orders (filter by event)

**returns** — Ticket returns and refunds

- `dice-fm-pp-cli returns` — List returns (filter by event)

**tickets** — Sold tickets with holder details, pricing, and claim status

- `dice-fm-pp-cli tickets` — List sold tickets (filter by event)

**transfers** — Ticket transfers between fans

- `dice-fm-pp-cli transfers` — List ticket transfers (filter by event)

**normalize** — Canonicalize manually-entered ticket-type and venue names into structured axes (parallel and re-runnable; raw synced data is never modified)

- `dice-fm-pp-cli normalize` — Resolve raw names → canonical entities + axes (`--tiers`, `--venues`, `--all`, `--entity`, `--fuzzy`, `--fuzzy-threshold <float>`, `--export-unmatched <file>`, `--export-format prompt|names`, `--import <file.csv|.json>`)
- `dice-fm-pp-cli normalize stats` — Show the normalized axis distribution (`--entity`)
- `dice-fm-pp-cli normalize recommend` — Profile the store and emit a starter normalization config (`--print` previews without writing)
- `dice-fm-pp-cli normalize promote-rules` — Graduate method=manual classifications into validated regex rules (`--entity <type>`, `--write`, `--min-support <int>` default `2`)

  Query the normalized view via `revenue summary --by-axis <access_class|sales_stage|entry_window_type|group_size|comp_flag>`. Raw is the default; `--by-axis` falls back to raw (with a warning) if `normalize` has not been run. Normalization is local-only — resolved name mappings never leave your machine.

  Future: `--classifier-cmd <path>` will let you bring your own LLM subprocess for classification; the external command owns its auth and credentials.

**capacity** — Cross-event sold-vs-capacity headroom from the local store

- `dice-fm-pp-cli capacity` — Sold-vs-capacity headroom rollup across events (`--event`, `--limit`)
- `dice-fm-pp-cli capacity pools` — Per-ticket-pool allocation breakdown, pool-sum vs event total (`--event`, `--limit`)

**tier-performance** — Price-tier sales analysis from the local store

- `dice-fm-pp-cli tier-performance` — Per price-tier redemptions and each tier's share of total sales (`--limit`)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dice-fm-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Build tonight's door list

```bash
dice-fm-pp-cli door list --event RXZlbnQ6MTIzNDU= --json
```

Returns valid ticket holders with transfer resolution — who holds valid tickets, with new holder names for any transferred tickets.

### Export opted-in London fans for Mailchimp

```bash
dice-fm-pp-cli fans optin --event RXZlbnQ6MTIzNDU= --country GB --city London --csv
```

Filters opted-in buyers from London, outputs CSV with firstName, lastName, email for direct import.

### Weekly CFO revenue report

```bash
dice-fm-pp-cli revenue summary --from 2026-01-01 --json --select event_name,gross,dice_fees,net,orders_count
```

Aggregates all orders since January 1, showing gross, Dice fees, and net per event with totals.

### Find repeat buyers for new event announcement

```bash
dice-fm-pp-cli fans repeat --min-events 2 --since 2026-01-01 --agent
```

Lists fans who attended 2+ events this year with total spend — warm audience for early access campaigns.

### Check ticket velocity in first 72 hours

```bash
dice-fm-pp-cli velocity show --event RXZlbnQ6MTIzNDU= --bucket hour --json --select hour_offset,cumulative_sold
```

Shows hourly cumulative ticket sales relative to on-sale time so you can decide if an event needs promotional push.

### Cross-event capacity headroom

```bash
dice-fm-pp-cli capacity --limit 20 --select event_name,sold,capacity,remaining,pct_sold
```

Ranks events by how close they are to selling out (sold, capacity, remaining, pct_sold). Add `capacity pools --event <id>` to break a single event into its ticket pools.

### Which price tiers carried the sales mix

```bash
dice-fm-pp-cli tier-performance --limit 20 --json
```

Per price-tier redemptions and each tier's share of total sales — which price points actually moved.

### Normalize names, then report by axis

```bash
dice-fm-pp-cli normalize --tiers --venues --fuzzy
dice-fm-pp-cli normalize --export-unmatched unmatched.json
# classify externally, then import the result as method=manual
dice-fm-pp-cli normalize --import classified.json
dice-fm-pp-cli normalize promote-rules --entity ticket_type --write
dice-fm-pp-cli normalize --tiers --venues --fuzzy
dice-fm-pp-cli revenue summary --from 2026-01-01 --by-axis access_class --json
```

Canonicalizes free-text ticket-type and venue names into structured axes (parallel and local-only; raw data untouched), then groups a revenue report on a clean axis. Run `normalize recommend --print` first to preview a starter config; after importing manual classifications, `normalize promote-rules --entity <type> --write` promotes repeat tokens into deterministic regex rules.

### Via the MCP server

After installing `dice-fm-pp-mcp` (see **MCP Server Installation** below), call tools by name — CLI command paths map to tool names with spaces/hyphens as underscores, and flags become arguments:

- `orders_list` with `{ "event": "<id>" }` — a show's orders
- `capacity` with `{ "limit": 20 }` — capacity headroom across events
- `tier_performance` with `{ "limit": 20 }` — price-tier sales mix
- `normalize_stats` with `{ "entity": "ticket_type" }` — normalized coverage by axis

These (plus the eight typed `*_list` / `events_get` resource tools) are read-only. `normalize` writes the local store, and `normalize_promote_rules` is a write tool when writing promoted rules, so call them from the CLI or invoke them deliberately. Custom SQL is out of scope here.

**Personal data is pseudonymized by default.** Tools that can return fan/holder PII — `tickets_list`, `orders_list`, `returns_list`, `transfers_list`, `extras_list`, `search`, `sql`, and the mirrored `fans_top` / `fans_profile` / `fans_optin` / `fans_repeat` / `fans_segment` / `door_list` — replace buyer/holder `email`, `phone`, and name with a stable `fan_ref` token (e.g. `fan:1a2b3c4d5e6f708192`) and omit `dob`. The token is deterministic per fan, so you can dedup/correlate the same person across calls without raw identifiers entering the model context. Pass `include_pii: true` on any of these to get raw values **plus** the token. The plain CLI is unaffected — it always emits raw output to the operator's terminal. `sql` redaction is best-effort (known PII columns + nested JSON `data` cells); a column alias or computed expression can slip past it, so prefer the typed tools or `search` when you need a guarantee.

## Auth Setup

Requires a Bearer token from MIO (DICE.FM AMP). Set DICE_FM_TOKEN in your environment. All commands are read-only — no writes to the DICE platform.

Run `dice-fm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dice-fm-pp-cli events list --agent --select id,name,status
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
dice-fm-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dice-fm-pp-cli feedback --stdin < notes.txt
dice-fm-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.dice-fm-pp-cli/feedback.jsonl`. They are never POSTed unless `DICE_FM_FEEDBACK_ENDPOINT` is set (must be `https://`) AND either `--send` is passed or `DICE_FM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename; created dirs `0700`, file `0600`) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

Webhook delivery is hardened because command output can carry personal data:

- **https only** — cleartext `http://` is refused.
- **SSRF guard** — a host that resolves to a private (RFC-1918), loopback, link-local, or cloud-metadata (`169.254.169.254`) address is refused unless you pass `--allow-private-webhook` (opt-in for a trusted internal endpoint).
- **Audit** — a `delivered N bytes to <host>` line is written to stderr on success.

`--deliver` is blocked on the MCP surface, so an agent cannot use it to exfiltrate output; it is a human/CLI feature.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
dice-fm-pp-cli profile save briefing --json
dice-fm-pp-cli --profile briefing events list
dice-fm-pp-cli profile list --json
dice-fm-pp-cli profile show briefing
dice-fm-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dice-fm-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/cmd/dice-fm-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dice-fm-pp-mcp -- dice-fm-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dice-fm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dice-fm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dice-fm-pp-cli <command> --help`.
