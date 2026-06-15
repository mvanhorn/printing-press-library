---
name: pp-shopping
description: "Every LemmeBuyIt retail endpoint, plus a local store that compares one UPC across 70+ retailers Trigger phrases: `compare prices across retailers`, `find the cheapest retailer for this UPC`, `find retail deals over 30% off`, `is this product profitable to flip on Amazon`, `show this week's biggest price drops`, `use shopping`, `run shopping`."
author: "NicholasSpisak"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - shopping-pp-cli
    install:
      - kind: go
        bins: [shopping-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/shopping/cmd/shopping-pp-cli
---

# Shopping — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `shopping-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install shopping --cli-only
   ```
2. Verify: `shopping-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopping/cmd/shopping-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Shopping wraps the LemmeBuyIt aggregated-retail API (115M+ products across 70+ retailers) and adds a local SQLite layer no other tool has. `index` syncs products and price history; `compare` finds the cheapest retailer for a shared identifier; `deals` runs a compound discount/price/rating/stock query across stores; `arbitrage` ranks Amazon FBA resale margin; `price-drops` surfaces weekly price drops and `leaderboard` ranks discount buckets. Agent-native --json/--select, offline search, typed exit codes.

## When to Use This CLI

Use Shopping for cross-retailer retail-product tasks an agent needs answered fast: find the cheapest retailer for a UPC, surface the best in-stock deals over a discount/price threshold, rank biggest weekly price drops, or evaluate Amazon FBA resale margin. It is strongest when you sync once and run many compound queries against the local store.

## Anti-triggers

Do not use this CLI for:
- Do not use it to place orders, add to cart, or check out — it is a read-only data API.
- Do not use it for non-retail data or for a single retailer's own order-management API.
- Do not use it as a real-time price oracle — store-backed queries reflect the last `index` sync, not the live shelf price.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`index`** — Pull products (and optional price history) from one or more retailers into a fast local SQLite store so every other query runs instantly and offline.

  _Reach for this first: the compare/deals/arbitrage/analytics commands all read what index populates._

  ```bash
  shopping-pp-cli index --retailer walmart --retailer target --on-sale --limit 500
  ```
- **`watch status`** — Pin items you care about and, after each index refresh, see which ones moved, by how much, and which hit your target price.

  _Use it for change-since-last-observation on a curated set instead of re-fetching and eyeballing prices._

  ```bash
  shopping-pp-cli watch status --json
  ```

### Cross-retailer intelligence
- **`compare`** — See which synced retailer sells the exact same item (by UPC/EAN/GTIN/ASIN) cheapest right now, ranked, with the savings spread.

  _Use it to answer 'who has this cheapest' in one call instead of N per-retailer lookups._

  ```bash
  shopping-pp-cli compare 012345678905 --lookup-type upc --in-stock --json
  ```
- **`deals`** — Surface deals that clear several bars at once — deep discount, under a price ceiling, well-reviewed, in stock — across all tracked retailers in one ranked list.

  _Pick this when an agent needs the best deals worth a human's time, not a raw product dump._

  ```bash
  shopping-pp-cli deals --min-discount 30 --max-price 100 --min-rating 4 --in-stock --sort discount --json
  ```

### Profitability
- **`arbitrage`** — Rank synced products by what you would net reselling them on Amazon after referral and FBA fees, filtered by ROI and buy price.

  _Use it to filter clearance products down to the ones that actually clear a resale ROI bar._

  ```bash
  shopping-pp-cli arbitrage --retailer walmart --min-roi 30 --max-buy-price 50 --in-stock --json
  ```

### Time-series analytics
- **`price-drops`** — Rank the products that fell the most over the last week (or N weeks) across everything synced, by both dollar and percent drop.

  _Reach for it to catch fresh markdowns first instead of re-pulling each product's history by hand._

  ```bash
  shopping-pp-cli price-drops --weeks 1 --min-drop-pct 15 --limit 25 --json
  ```
- **`leaderboard`** — Show which retailers and categories are consistently the deepest-discount buckets in your synced data, by average discount, on-sale count, or average price.

  _Use it to decide which retailer/category to scan first when hunting deals._

  ```bash
  shopping-pp-cli leaderboard --by avg-discount --limit 15 --json
  ```

## Command Reference

**amazon** — Manage amazon

- `shopping-pp-cli amazon <asin>` — Returns chart-ready weekly Amazon-side time-series data for a specific ASIN — direct access to amazon_price_history

**retailers** — Operations related to retailers and their products.

- `shopping-pp-cli retailers` — Retrieves a list of retailers that the authenticated API key has access to.

**status** — API health and status checks.

- `shopping-pp-cli status` — Provides the current operational status of the API service, including database connectivity.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
shopping-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Cheapest retailer for a UPC

```bash
shopping-pp-cli compare 012345678905 --lookup-type upc --in-stock --json --select results.retailer_id,results.current_price
```

Cross-retailer compare narrowed to the two fields an agent needs to act on.

### Best in-stock deals

```bash
shopping-pp-cli deals --min-discount 40 --max-price 75 --in-stock --sort discount --limit 20 --json
```

Compound discount/price/stock query ranked by discount over the synced store.

### FBA arbitrage shortlist

```bash
shopping-pp-cli arbitrage --retailer walmart --min-roi 30 --max-buy-price 50 --json --select results.product_name,results.roi,results.net_margin
```

Rank resale candidates by ROI, selecting only the decision fields.

### This week's biggest drops

```bash
shopping-pp-cli price-drops --weeks 1 --min-drop-pct 20 --limit 25 --json
```

Weekly time-series delta ranking across every synced product.

## Auth Setup

Authenticate with a single LemmeBuyIt API key sent as the `X-API-Key` header on every request (set `SHOPPING_API_KEY`, or run `shopping-pp-cli auth set-token <key>`). Free keys reach the `/shopping/` product surface; paid keys additionally unlock full product search, weekly price history, Amazon ASIN history, and the Amazon profitability data that powers `arbitrage`. Get a key at https://www.lemmebuyit.com/developer.

Run `shopping-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  shopping-pp-cli retailers --agent --select id,name,status
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
shopping-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
shopping-pp-cli feedback --stdin < notes.txt
shopping-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/shopping-pp-cli/feedback.jsonl`. They are never POSTed unless `SHOPPING_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SHOPPING_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
shopping-pp-cli profile save briefing --json
shopping-pp-cli --profile briefing retailers
shopping-pp-cli profile list --json
shopping-pp-cli profile show briefing
shopping-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `shopping-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/shopping/cmd/shopping-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add shopping-pp-mcp -- shopping-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which shopping-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   shopping-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `shopping-pp-cli <command> --help`.
