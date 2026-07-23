---
name: pp-amazon-orders
description: "Walk your Amazon order history offline — every order, item, shipment, and dollar in a local SQLite store no other... Trigger phrases: `where is my Amazon order`, `what did I spend on Amazon this month`, `find that thing I ordered on Amazon`, `when did I order from Amazon`, `track my Amazon shipments`, `show my Amazon purchase history`, `use amazon-orders`, `run amazon-orders`."
author: "Brian Wishan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - amazon-orders-pp-cli
    install:
      - kind: go
        bins: [amazon-orders-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/cmd/amazon-orders-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/amazon-orders/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Amazon Orders — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `amazon-orders-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install amazon-orders --cli-only
   ```
2. Verify: `amazon-orders-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/cmd/amazon-orders-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for amazon-orders-pp-cli when an agent needs to reason across the user's Amazon order history — multi-order tracking radars, recurring-purchase detection, spending rollups, FTS5 searches like 'when did I order that USB-C cable'. The local SQLite store means cross-cutting questions return instantly without re-hitting Amazon and without burning context on full HTML pages.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`where-is-my-stuff`** — All in-flight Amazon shipments with their current status and ETA in one view.

  _When an agent needs to answer 'is my package coming today' across many orders, this is the one-shot view._

  ```bash
  amazon-orders-pp-cli where-is-my-stuff --json --select orderId,status,etaDate,carrier
  ```
- **`delivery-slips`** — Orders whose actual delivery date slipped more than N days from the original estimate.

  _Surfaces unreliable carriers and sellers without manually scrolling through every order._

  ```bash
  amazon-orders-pp-cli delivery-slips --days 3 --since 2025-01-01 --json
  ```
- **`spend`** — Spending broken down by month, year, category, seller, or payment method.

  _Gives agents a one-shot answer for budgeting, expense reporting, and trend analysis._

  ```bash
  amazon-orders-pp-cli spend --by month --year 2025 --json
  ```
- **`top-items`** — Most-ordered items by frequency or by total spend, ASIN-grouped across all history.

  _Helps an agent reason about what the user actually consumes vs one-off purchases._

  ```bash
  amazon-orders-pp-cli top-items --by total-spend --limit 20 --json
  ```
- **`subscribe-and-save`** — Recurring purchases inferred from order history (same ASIN ordered on a regular cadence).

  _Surfaces candidates for actual S&S enrollment and detects de-facto subscriptions the user may not realize they have._

  ```bash
  amazon-orders-pp-cli subscribe-and-save --min-occurrences 3 --json
  ```
- **`arriving-soon`** — Shipments arriving in the next N days, sorted by ETA.

  _Lets an agent plan around incoming deliveries (e.g. 'is my router arriving before the meeting on Friday?')._

  ```bash
  amazon-orders-pp-cli arriving-soon --days 7 --json
  ```
- **`late`** — Active shipments past their original estimated delivery date.

  _Surfaces carrier delays the moment they happen, no manual review._

  ```bash
  amazon-orders-pp-cli late --json
  ```

### Agent-native plumbing
- **`find`** — FTS5 search across orders, items, sellers, and tracking notes.

  _Direct answer to 'when did I order that thing' without scrolling through years of order history._

  ```bash
  amazon-orders-pp-cli find 'usb-c cable' --json --select orderId,placedDate,total
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 6 API entries from 6 total network entries
- Protocols: html_cookie (85% confidence)

## Command Reference

**gift_cards** — Gift card balance and activity history.

- `amazon-orders-pp-cli gift_cards` — Current gift card balance plus the activity log: amounts, kinds (added/applied/refund), dates, linked order IDs.

**orders** — Your buyer-side order history listings and per-order detail pages.

- `amazon-orders-pp-cli orders get` — Full detail for a single order: items, ASINs, prices, shipments, payment method, ship-to address, totals.
- `amazon-orders-pp-cli orders invoice` — Printable invoice for an order (HTML), useful for VAT/expense reconciliation.
- `amazon-orders-pp-cli orders list` — Fetch one page of your order history. Use timeFilter (year-2026, last30days, months-3) and startIndex to paginate.

**shipments** — Per-package tracking details.

- `amazon-orders-pp-cli shipments` — Tracking detail for a single shipment: carrier, tracking number, status, ETA, delivery confirmation.

**transactions** — Charges and refunds across all orders, recurring services, and Prime.

- `amazon-orders-pp-cli transactions` — First page of your transactions list, grouped by date. Each row has payment method, last-4, signed amount, and (when...


**Hand-written commands**

- `amazon-orders-pp-cli sync` — Sync order history into the local SQLite store. Use `--since 90d` for incremental, `--full` to re-walk everything.
- `amazon-orders-pp-cli orders list` — List orders with `--time-filter` (year-YYYY, last30days, months-3) and `--start-index` paging.
- `amazon-orders-pp-cli orders get <order-id>` — Show full detail for one order (items, shipments, payment, totals).
- `amazon-orders-pp-cli orders invoice <order-id>` — Print invoice for one order.
- `amazon-orders-pp-cli shipments --order-id <id>` — Tracking detail for one order's shipments.
- `amazon-orders-pp-cli track <order-id>` — Live tracking for one order: carrier, tracking number, status, ETA.
- `amazon-orders-pp-cli where-is-my-stuff` — All in-flight shipments with current status and ETA in one view.
- `amazon-orders-pp-cli arriving-soon --days 7` — Shipments arriving in the next N days, sorted by ETA.
- `amazon-orders-pp-cli late` — Shipments past their estimated delivery date.
- `amazon-orders-pp-cli delivery-slips --days 3 --since 2025-01-01` — Orders whose actual delivery slipped >N days from the original estimate. (Requires sync history.)
- `amazon-orders-pp-cli spend --by month --year 2026` — Spending rollup by month, year, ship-to, or status.
- `amazon-orders-pp-cli top-items --by count --limit 20` — Most-ordered items / ASINs across history.
- `amazon-orders-pp-cli returns` — Items returned, joined orders <-> transactions. (Requires sync history.)
- `amazon-orders-pp-cli subscribe-and-save --min-occurrences 3` — De-facto subscription detector. (Requires sync history.)
- `amazon-orders-pp-cli carriers --rank` — Per-carrier on-time percentage. (Requires sync history.)
- `amazon-orders-pp-cli find '<query>'` — Free-text search across orders, items, ASINs, sellers.
- `amazon-orders-pp-cli transactions` — Charges and refunds (transactions page parser).
- `amazon-orders-pp-cli gift-cards` — Gift card balance + activity log.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `AMAZON_ORDERS_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `amazon-orders-pp-cli orders`
- `amazon-orders-pp-cli orders get`
- `amazon-orders-pp-cli orders invoice`
- `amazon-orders-pp-cli orders list`
- `amazon-orders-pp-cli transactions`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
amazon-orders-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Where is everything I'm waiting on?

```bash
amazon-orders-pp-cli where-is-my-stuff --json --select orderId,status,etaDate,carrier
```

Lists every open shipment with current status and ETA — the unified radar across all in-flight Amazon orders.

### How much did I spend this year?

```bash
amazon-orders-pp-cli spend --by month --year 2026 --json
```

Rolls up monthly Amazon spend from the local store; no live calls, sub-second.

### Find the order with that thing I bought

```bash
amazon-orders-pp-cli find 'usb-c hub' --json --select orderId,placedDate,total,items.title
```

FTS5 search across orders + items; uses dotted `--select` to extract just the fields the agent needs.

### What ships am I being charged for that aren't here yet?

```bash
amazon-orders-pp-cli late --json
```

Shows active shipments past their original ETA so an agent can flag carrier delays.

### What do I actually buy?

```bash
amazon-orders-pp-cli top-items --by total-spend --limit 20 --json
```

Groups every purchased ASIN across history by total spend — surfaces the recurring purchases.

## Auth Setup

Amazon publishes no buyer API. The CLI imports cookies from your logged-in Chrome / Firefox / Safari / Brave session via `auth login --chrome`. Those cookies persist locally, refresh automatically, and authenticate every subsequent fetch — no API key, no OAuth, no resident browser at runtime.

For non-US marketplaces, pass the marketplace domain during login. Example for India:

```bash
amazon-orders-pp-cli auth login --chrome --domain amazon.in
```

You can also set `AMAZON_ORDERS_BASE_URL=https://www.amazon.in` before running `auth login --chrome`.

Run `amazon-orders-pp-cli doctor` to verify setup.

### Headless agent setup with 1Password (LLM-free roundtrip)

For a headless host (CI, dev container, remote agent) where you cannot run `auth login --chrome`, capture the session once on a logged-in machine, stash it in 1Password (or any other secrets manager), and inject it on the headless host. **The cookie value never enters an LLM's context window** because the bytes flow `op → stdin → CLI` without crossing a shell variable the LLM can read.

**Stash it once (on the logged-in machine):**

```bash
TMP=$(mktemp -t amazon-orders-session.XXXXXX.json)
chmod 600 "$TMP"
amazon-orders-pp-cli auth export --output "$TMP" --note "captured $(date -u +%FT%TZ) from $(hostname)"
op document create "$TMP" \
  --title 'amazon-orders-session' \
  --vault Agent \
  --tags 'cli-session,amazon-orders,printing-press'
shred -u "$TMP" 2>/dev/null || rm -f "$TMP"
```

**Inject it on every other machine:**

```bash
# Long form (separate fetch + import):
op document get 'amazon-orders-session' --vault Agent \
  | amazon-orders-pp-cli auth import --stdin

# Short form via op:// URI (works in direnv, agent configs, K8s init containers):
op read "op://Agent/amazon-orders-session/file" \
  | amazon-orders-pp-cli auth import --stdin
```

Either form lands the cookie material in `~/.config/amazon-orders-pp-cli/config.toml` and you're authenticated. Validate with `amazon-orders-pp-cli auth status`.

**Refresh when Amazon rotates `session-id`** (typically every few weeks; signaled by 401s from `orders list`):

```bash
amazon-orders-pp-cli auth login --chrome           # on the logged-in machine
amazon-orders-pp-cli auth export --output /tmp/s.json
op document edit 'amazon-orders-session' --vault Agent /tmp/s.json
shred -u /tmp/s.json 2>/dev/null || rm -f /tmp/s.json
```

**Substitutes:** any secret manager that pipes to stdout works the same way. `vault kv get`, `aws secretsmanager get-secret-value --query SecretString --output text`, `pass show amazon-orders-session`, `bw get item amazon-orders-session | jq -r .notes` all chain into `amazon-orders-pp-cli auth import --stdin`.

**`AMAZON_COOKIES` env var path:** the CLI also auto-imports from this env var. For one-shot containers:

```bash
export AMAZON_COOKIES="$(op read 'op://Agent/amazon-orders-session/file')"
amazon-orders-pp-cli auth import   # reads $AMAZON_COOKIES, then exits
amazon-orders-pp-cli orders list --json
```

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  amazon-orders-pp-cli orders list --agent --select id,name,status
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
amazon-orders-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
amazon-orders-pp-cli feedback --stdin < notes.txt
amazon-orders-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.amazon-orders-pp-cli/feedback.jsonl`. They are never POSTed unless `AMAZON_ORDERS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AMAZON_ORDERS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
amazon-orders-pp-cli profile save briefing --json
amazon-orders-pp-cli --profile briefing orders list
amazon-orders-pp-cli profile list --json
amazon-orders-pp-cli profile show briefing
amazon-orders-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `amazon-orders-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/cmd/amazon-orders-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add amazon-orders-pp-mcp -- amazon-orders-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which amazon-orders-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   amazon-orders-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `amazon-orders-pp-cli <command> --help`.
