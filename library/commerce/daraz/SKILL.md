---
name: pp-daraz
description: "Every Daraz product search, plus a local price database, real-deal ranking, and drop alerts no Daraz scraper has. Trigger phrases: `search daraz for`, `daraz price of`, `find deals on daraz`, `daraz reviews for`, `is this a good deal on daraz`, `use daraz`, `run daraz`."
author: "Hamza Qazi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - daraz-pp-cli
    install:
      - kind: go
        bins: [daraz-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/daraz/cmd/daraz-pp-cli
---

# Daraz.pk — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `daraz-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install daraz --cli-only
   ```
2. Verify: `daraz-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/daraz/cmd/daraz-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

daraz-pp-cli searches Daraz.pk listings, reads reviews, and extracts product detail like any scraper — then keeps it in a local SQLite store so it can do what one-shot scrapers cannot: track price history (price-history), rank genuine deals over fake discounts (deals, value), diff a saved search over time (since), and vet sellers (seller stats). Read-only, no login required.

## When to Use This CLI

Use daraz-pp-cli for shopper-side Daraz.pk research: finding products, comparing prices and sellers, reading reviews, and tracking whether a price is genuinely low over time. It is ideal for agents that need structured, filterable product data and offline price intelligence from a single keyword.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place orders, manage a cart, or checkout — it is read-only.
- Do not use it for order tracking, wishlists, or account actions that require a logged-in Daraz session.
- Do not use it for seller/merchant operations — that is the separate Daraz Open Platform.
- Do not use it for non-Pakistan marketplaces; it targets daraz.pk.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local price intelligence
- **`price-history`** — See a product's recorded price trend and its lowest-ever price from your local store.

  _Reach for this to judge whether a current price is actually low for that item, not just discounted on paper._

  ```bash
  daraz-pp-cli price-history 599201597 --agent
  ```
- **`watch`** — Record current prices for every item matching a search into the local store so price history builds over time.

  _Run periodically on a query you care about; then price-history shows the trend for any item it captured._

  ```bash
  daraz-pp-cli watch "gaming laptop"
  ```
- **`since`** — Diff a saved search against its last local snapshot to show new listings and price moves since you last checked.

  _Use to monitor a market over time instead of re-reading a full result list each run._

  ```bash
  daraz-pp-cli since "gaming mouse" --agent
  ```

### Smart ranking
- **`deals`** — Rank a search by a composite of discount, rating, and units sold to surface genuinely good deals.

  _Use when the goal is the best-value item for a query, not raw listing order._

  ```bash
  daraz-pp-cli deals "laptop" --agent --select name,price,discountPct,rating
  ```
- **`value`** — Flag inflated original-price discounts by comparing each item's claimed original price to the local median for that query.

  _Use to avoid misleading discounts; it is the inverse of deals._

  ```bash
  daraz-pp-cli value "power bank" --agent
  ```
- **`compare`** — Find the same item across sellers and show the cheapest and best-rated side by side.

  _Use to pick the best seller/listing for an item before buying._

  ```bash
  daraz-pp-cli compare "airpods pro" --agent
  ```

### Seller trust
- **`seller stats`** — Aggregate a seller's catalog from the local store: average rating, price range, listing count, discount pattern.

  _Use to vet a seller's track record before trusting a listing._

  ```bash
  daraz-pp-cli seller stats 1066739 --agent
  ```

## Command Reference

**products** — Search Daraz product listings

- `daraz-pp-cli products` — Search products by keyword with sort and price filtering

**reviews** — Read product reviews and rating distribution

- `daraz-pp-cli reviews` — List reviews for a product by its numeric item ID


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
daraz-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Agent-friendly search with field selection

```bash
daraz-pp-cli deals "wireless earbuds" --agent --select name,price,discountPct,rating,url
```

deals returns a flat, ranked array of listings, so --select trims each row to just the fields an agent needs and keeps payloads small.

### Find the best-value items, not the biggest fake discount

```bash
daraz-pp-cli deals "power bank" --agent --select name,price,discountPct,rating
```

Ranks by a discount-times-rating-times-sales composite computed locally.

### Start tracking a query's prices

```bash
daraz-pp-cli watch "gaming laptop"
```

Records current prices for the search so price-history builds a real trend over time.

### See what changed in a market since last check

```bash
daraz-pp-cli since "gaming mouse" --agent
```

Diffs the current search against the last local snapshot for new listings and price moves.

## Auth Setup

No authentication required.

Run `daraz-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  daraz-pp-cli products --query example-value --agent --select id,name,status
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
daraz-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
daraz-pp-cli feedback --stdin < notes.txt
daraz-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/daraz-pp-cli/feedback.jsonl`. They are never POSTed unless `DARAZ_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DARAZ_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
daraz-pp-cli profile save briefing --json
daraz-pp-cli --profile briefing products --query example-value
daraz-pp-cli profile list --json
daraz-pp-cli profile show briefing
daraz-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `daraz-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/daraz/cmd/daraz-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add daraz-pp-mcp -- daraz-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which daraz-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   daraz-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `daraz-pp-cli <command> --help`.
