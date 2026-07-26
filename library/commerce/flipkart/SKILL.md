---
name: pp-flipkart
description: "Every Flipkart scraper's features, plus a local price-history and deal-tracking layer no single-shot script can offer. Trigger phrases: `find me the best price on Flipkart`, `track this Flipkart product's price`, `compare these Flipkart products`, `check Flipkart deals in electronics`, `use flipkart-cli`, `run flipkart-pp-cli`."
author: "Rajiv Lokare"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - flipkart-pp-cli
    install:
      - kind: go
        bins: [flipkart-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/flipkart/cmd/flipkart-pp-cli
---

# Flipkart — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `flipkart-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install flipkart --cli-only
   ```
2. Verify: `flipkart-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/flipkart/cmd/flipkart-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search, product detail, and the official Affiliate API in one consistent schema — with local SQLite turning every fetch into price history, watchlist digests, and cross-product deal arbitrage.

## When to Use This CLI

Reach for this CLI when comparison-shopping on Flipkart, tracking prices toward a target before a sale, or running an affiliate catalog refresh. Not for personal order history or account management — Flipkart exposes no API for that.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch digest`** — See everything that changed across your whole watchlist since you last checked — price drops, new offers, stock changes — in one command.

  _Reach for this instead of re-checking N product pages by hand — it answers 'what changed' in one call._

  ```bash
  flipkart-pp-cli watch digest --json
  ```
- **`compare`** — Line up 2 or more products side by side — price, rating, discount, key specs — in one table instead of copy-pasting across tabs.

  _Use when deciding between a shortlist of similar products rather than fetching each individually._

  ```bash
  flipkart-pp-cli compare "https://www.flipkart.com/oppo-a3x-8-gb-ram-128-gb-storage-lake-green/p/itmc050065f36601?pid=MOBH4ZCFEQZFQBFB" "https://www.flipkart.com/samsung-galaxy-m06-5g-mint-green-128-gb/p/itmc0501a8c1f5e5?pid=MOBH8QSVDVBRZFNJ" --json
  ```
- **`catalog diff`** — Re-run a saved search and see exactly what's new, removed, or price-changed since the last time you ran it.

  _Use to track a category of interest over time without hand-comparing result pages._

  ```bash
  flipkart-pp-cli catalog diff "wireless earbuds" --json
  ```
- **`deals category`** — Scan an entire category for products above a discount threshold, and keep the results queryable offline afterward.

  _Use during sale windows to shortlist deals once, then re-query them offline without re-hitting the API._

  ```bash
  flipkart-pp-cli deals category electronics --min-discount 40 --json
  ```
- **`offers best-card`** — Across a set of products you're considering, find the single bank card that maximizes your total stacked savings.

  _Use before checkout when weighing which of your cards to use across a multi-item cart._

  ```bash
  flipkart-pp-cli offers best-card "https://www.flipkart.com/oppo-a3x-8-gb-ram-128-gb-storage-lake-green/p/itmc050065f36601?pid=MOBH4ZCFEQZFQBFB" "https://www.flipkart.com/samsung-galaxy-m06-5g-mint-green-128-gb/p/itmc0501a8c1f5e5?pid=MOBH8QSVDVBRZFNJ" --json
  ```

### Agent-native plumbing
- **`feed digest`** — Pull the official Delta Feed API without manually tracking the last fromVersion — the CLI remembers it — and see deltas ranked by discount-percentage change.

  _Use for a weekly affiliate-catalog refresh instead of hand-tracking version numbers across runs._

  ```bash
  flipkart-pp-cli feed digest electronics --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**catalog** — Search Flipkart's live product catalog by keyword.

- `flipkart-pp-cli catalog <q>` — Search Flipkart products by keyword.

**feed** — Official Flipkart Affiliate API — category and delta product feeds (requires an approved affiliate account).

- `flipkart-pp-cli feed category` — Fetch the product feed for one category via the official Affiliate API.
- `flipkart-pp-cli feed delta` — Fetch only products changed since a given feed version via the official Affiliate API.
- `flipkart-pp-cli feed product` — Look up a single product by Flipkart product ID via the Affiliate API.
- `flipkart-pp-cli feed search` — Search the Affiliate API catalog by keyword.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
flipkart-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `flipkart-pp-cli product get <product-url>` — Fetch full product detail (price, rating, brand, images, reviews) from a Flipkart product URL — persists a...
- `flipkart-pp-cli product offers <product-url>` — Extract MRP, discount percentage, and bank/card offers from a Flipkart product page
- `flipkart-pp-cli watch add <product-url> --threshold <price>` — Track a product's price, alerting when it drops below a threshold
- `flipkart-pp-cli watch check` — Refresh all watched products and report any that crossed their threshold
- `flipkart-pp-cli watch digest` — Show everything that changed across your whole watchlist since you last checked
- `flipkart-pp-cli compare <url1> <url2> [url3...]` — Line up 2+ products side by side (price, rating, discount, key specs)
- `flipkart-pp-cli feed digest <category>` — Pull the Affiliate Delta Feed without manually tracking fromVersion, ranked by discount-percentage change
- `flipkart-pp-cli catalog diff <query>` — Re-run a saved search and see what's new, removed, or price-changed since last time
- `flipkart-pp-cli deals category <category> --min-discount <n>` — Scan a category for products above a discount threshold; results stay queryable offline
- `flipkart-pp-cli offers best-card <url1> <url2> [url3...]` — Across a set of products, find the single bank card that maximizes total stacked savings

## Recipes


### Track a product toward a target price

```bash
flipkart-pp-cli watch add <url> --threshold 25000 && flipkart-pp-cli sync && flipkart-pp-cli watch digest
```

Adds a watch, refreshes it, then shows what changed — the core price-tracking loop.

### Compare a shortlist before buying

```bash
flipkart-pp-cli compare <url1> <url2> <url3> --json --select products.title,products.price,products.rating
```

Narrows a side-by-side compare to just the fields that matter, keeping agent context small.

### Find the best card for a multi-item cart

```bash
flipkart-pp-cli offers best-card <url1> <url2>
```

Aggregates bank/card offers across items to surface the single card with the highest total stacked savings.

### Weekly affiliate catalog refresh

```bash
flipkart-pp-cli feed digest electronics --json
```

Pulls only what changed since the last run, ranked by discount-percentage change, without manual version tracking.

## Auth Setup

No authentication required.

Run `flipkart-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  flipkart-pp-cli catalog mock-value --agent --select id,name,status
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
flipkart-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
flipkart-pp-cli feedback --stdin < notes.txt
flipkart-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.flipkart-pp-cli/feedback.jsonl`. They are never POSTed unless `FLIPKART_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FLIPKART_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
flipkart-pp-cli profile save briefing --json
flipkart-pp-cli --profile briefing catalog mock-value
flipkart-pp-cli profile list --json
flipkart-pp-cli profile show briefing
flipkart-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `flipkart-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/flipkart/cmd/flipkart-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add flipkart-pp-mcp -- flipkart-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which flipkart-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   flipkart-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `flipkart-pp-cli <command> --help`.
