---
name: pp-shopify
description: "Printing Press CLI for Shopify. Ecommerce orders, products, customers, inventory, fulfillment orders, and bulk operations via the Shopify Admin..."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["shopify-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/commerce/shopify/cmd/shopify-pp-cli@latest","bins":["shopify-pp-cli"],"label":"Install via go install"}]}}'
---

# Shopify — Printing Press CLI

Ecommerce orders, products, customers, inventory, fulfillment orders, and bulk operations via the Shopify Admin GraphQL API.

## When Not to Use This CLI

Do not use resource commands for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. The exception is `bulk-operations run-query`, which intentionally starts a Shopify bulk export job when the user explicitly asks for it.

## Command Reference

**customers** — Shopify customers with lifetime order count, lifetime spend, and contact fields.

- `shopify-pp-cli customers get` — Get one Shopify customer by GraphQL ID.
- `shopify-pp-cli customers list` — List customers from the Shopify Admin GraphQL API.

**fulfillment-orders** — Shopify fulfillment orders for lag, routing, and fulfillment-state analysis.

- `shopify-pp-cli fulfillment-orders get` — Get one Shopify fulfillment order by GraphQL ID.
- `shopify-pp-cli fulfillment-orders list` — List fulfillment orders from the Shopify Admin GraphQL API.

**inventory-items** — Shopify inventory items with tracked status and available quantities by location.

- `shopify-pp-cli inventory-items get` — Get one Shopify inventory item by GraphQL ID.
- `shopify-pp-cli inventory-items list` — List inventory items from the Shopify Admin GraphQL API.

**orders** — Shopify orders with money totals, financial state, and fulfillment state.

- `shopify-pp-cli orders get` — Get one Shopify order by GraphQL ID.
- `shopify-pp-cli orders list` — List orders from the Shopify Admin GraphQL API.

**products** — Shopify products with product status, catalog metadata, and a compact variant inventory projection.

- `shopify-pp-cli products get` — Get one Shopify product by GraphQL ID.
- `shopify-pp-cli products list` — List products from the Shopify Admin GraphQL API.


**Hand-written commands**

- `shopify-pp-cli bulk-operations current` — Show the current or most recent Shopify bulk operation.
- `shopify-pp-cli bulk-operations run-query --query-file <path>` — Start a Shopify bulk GraphQL export job from a query file.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `SHOPIFY_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `shopify-pp-cli customers`
- `shopify-pp-cli customers get`
- `shopify-pp-cli customers list`
- `shopify-pp-cli fulfillment-orders`
- `shopify-pp-cli fulfillment-orders get`
- `shopify-pp-cli fulfillment-orders list`
- `shopify-pp-cli inventory-items`
- `shopify-pp-cli inventory-items get`
- `shopify-pp-cli inventory-items list`
- `shopify-pp-cli orders`
- `shopify-pp-cli orders get`
- `shopify-pp-cli orders list`
- `shopify-pp-cli products`
- `shopify-pp-cli products get`
- `shopify-pp-cli products list`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
shopify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Set the store host, Admin API version, and access token via environment variables:

```bash
export SHOPIFY_SHOP="<your-store>.myshopify.com"
export SHOPIFY_API_VERSION="2026-04"
export SHOPIFY_ACCESS_TOKEN="<your-key>"
```

Or persist it in `~/.config/shopify-pp-cli/config.toml`.

Run `shopify-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  shopify-pp-cli customers list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-mostly** — resource commands are read-only; `bulk-operations run-query` mutates remote state by starting an export job and should only be used on explicit request

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
shopify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
shopify-pp-cli feedback --stdin < notes.txt
shopify-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.shopify-pp-cli/feedback.jsonl`. They are never POSTed unless `SHOPIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SHOPIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
shopify-pp-cli profile save briefing --json
shopify-pp-cli --profile briefing customers list
shopify-pp-cli profile list --json
shopify-pp-cli profile show briefing
shopify-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `shopify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/shopify/cmd/shopify-pp-cli@latest
   ```
3. Verify: `shopify-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/shopify/cmd/shopify-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add shopify-pp-mcp -- shopify-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which shopify-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   shopify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `shopify-pp-cli <command> --help`.
