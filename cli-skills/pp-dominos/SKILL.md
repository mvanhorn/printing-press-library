---
name: pp-dominos
description: "Order pizza, browse menus, optimize deals, and track delivery from the terminal — with a local SQLite store that powers reorder, analytics, and price comparison no other Domino's tool offers. Trigger phrases: `order a pizza`, `find a domino's near me`, `track my pizza`, `what's my pizza usual`, `how much do i spend on pizza`, `compare pizza prices`, `use dominos`, `run dominos`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dominos-pp-cli
    install:
      - kind: go
        bins: [dominos-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/dominos/cmd/dominos-pp-cli
---

# Dominos — Printing Press CLI

Every Domino's feature you would expect — store locator, menu browse, build cart, validate, price, place, and track — plus a local data layer that compounds. Save named templates with `template save`, find the cheapest store for your order with `compare-prices`, hunt for the best deal with `deals best`, and watch your delivery in real-time with `tracking --watch`.

## When to Use This CLI

Use this CLI when an agent or power user needs to interact with Domino's outside a browser — building, pricing, and placing orders, tracking deliveries, comparing prices across stores, optimizing deal selection, or analyzing past spending. Excellent for automation: every command supports --json, --dry-run, --agent, and structured exit codes. Local SQLite store enables features the public API cannot serve directly (reorder substitution, menu diff, deal optimization, spending trends).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`compare-prices`** — Compare item pricing across every nearby store and find the cheapest one for your order.

  _Pizza prices vary by store. Reach for this when the user cares about saving money or has a flexible delivery radius._

  ```bash
  dominos-pp-cli compare-prices --address "421 N 63rd St, Seattle WA" --items S_PIZPH,S_LAVA --agent
  ```
- **`template save`** — Save a complete order (store, address, items, toppings, payment) as a named template and replay it with one command.

  _Most users order the same thing repeatedly. Reach for this when the user says 'usual order' or 'what I had last Friday'._

  ```bash
  dominos-pp-cli template save "friday-night" --from-cart ./cart.json && dominos-pp-cli template order "friday-night"
  ```
- **`deals best`** — Cross-references the cart against every available deal (including loyalty-exclusive) to find the cheapest combination.

  _The headline price often hides a cheaper deal-applied path. Reach for this whenever the cart total is non-trivial._

  ```bash
  dominos-pp-cli deals best --cart ./cart.json --agent
  ```
- **`menu diff`** — Compare the current menu against the last-synced snapshot to surface new items, removed items, and price changes.

  _Power users want to know when menu items appear or change price. Reach for this for periodic health checks of a favorite store._

  ```bash
  dominos-pp-cli menu diff --store 7094 --no-update
  ```
- **`analytics`** — Aggregate order history into spending trends, favorite items, order frequency, and average order value over a chosen window.

  _Useful for budgeting and 'how much do I actually spend on pizza?' questions. Reach for this when the user asks for trends._

  ```bash
  dominos-pp-cli analytics --period 90d --top 10 --agent
  ```
- **`reorder`** — Replay your last order against today's menu, automatically substituting unavailable items with the closest match using FTS similarity.

  _Menus change. The 'usual' order may no longer be valid verbatim but can be reproduced in spirit. Reach for this when reorder fails strict._

  ```bash
  dominos-pp-cli reorder --last --substitute-unavailable --dry-run
  ```
- **`nutrition`** — Sum calories, protein, fat, and carbs across all items in a cart using the menu's embedded nutrition data.

  _Health-conscious users want to know cart-level totals before ordering. Reach for this when nutrition is mentioned._

  ```bash
  dominos-pp-cli nutrition --cart ./cart.json
  ```
- **`order-bulk`** — Read a CSV of multi-person orders, find the optimal store for the group, and emit a combined cart JSON ready for orders place_order.

  _Group ordering is painful. Reach for this when the user has more than 3 individual orders to place._

  ```bash
  dominos-pp-cli order-bulk --csv ./team-friday.csv --address "421 N 63rd St" --city "Seattle WA"
  ```
- **`stores health`** — Composite store health score combining wait times, hours, service capabilities, and historical delivery performance.

  _Two stores at similar distances can have very different ETAs. Reach for this when the user is choosing between options._

  ```bash
  dominos-pp-cli stores health 7094
  ```

### Agent-native plumbing

- **`tracking --watch`** — Polls the tracker endpoint at a chosen interval and streams status updates: prep → bake → quality check → out for delivery.

  _Agents and users alike want a non-blocking 'tell me when the pizza arrives' primitive. Reach for this immediately after order place._

  ```bash
  dominos-pp-cli tracking --phone 2065551234 --watch --interval 30s
  ```

## Command Reference

**auth** — Authentication and account management

- `dominos-pp-cli auth login` — Log in to a Domino's account

**graphql** — GraphQL BFF operations (discovered via sniff)

- `dominos-pp-cli graphql categories` — Get menu categories for a store
- `dominos-pp-cli graphql create_cart` — Create a new shopping cart
- `dominos-pp-cli graphql customer` — Get authenticated customer profile with saved addresses and preferences
- `dominos-pp-cli graphql deals_list` — Get available deals and coupons for a store
- `dominos-pp-cli graphql get_cart` — Get cart by ID with items and pricing
- `dominos-pp-cli graphql loyalty_deals` — Get member-exclusive deals
- `dominos-pp-cli graphql loyalty_points` — Get loyalty points balance and status
- `dominos-pp-cli graphql loyalty_rewards` — Get available loyalty rewards by tier
- `dominos-pp-cli graphql products` — Get products in a category with customization options
- `dominos-pp-cli graphql quick_add_product` — Quick-add a product to cart by code
- `dominos-pp-cli graphql summary_charges` — Get cart totals including tax and delivery fee

**menu** — Browse store menus and search for items

- `dominos-pp-cli menu get_menu` — Get the full menu for a store with categories, products, variants, and toppings

**orders** — Create, validate, price, and place orders

- `dominos-pp-cli orders place_order` — Place an order for delivery or carryout
- `dominos-pp-cli orders price_order` — Get the price for an order including taxes and fees
- `dominos-pp-cli orders validate_order` — Validate an order before placing it

**stores** — Find and get information about Domino's stores

- `dominos-pp-cli stores find_stores` — Find nearby Domino's stores by address
- `dominos-pp-cli stores get_store` — Get detailed store information including hours, capabilities, and wait times

**tracking** — Track active orders

- `dominos-pp-cli tracking track_order` — Track an order by phone number


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dominos-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Friday night reorder

```bash
dominos-pp-cli template order friday-night
```

Emit the saved template's cart JSON to stdout. Pipe it to `orders place_order --order @-` to place, then `tracking --watch` to follow the order.

### Find cheapest store for tonight's order

```bash
dominos-pp-cli compare-prices --address "$HOME_ADDR" --items S_PIZPH,S_LAVA --agent
```

Surveys every nearby store, computes the same cart at each, and reports the cheapest.

### Hunt for the best deal

```bash
dominos-pp-cli deals best --cart default --agent
```

Tries every available deal against the current cart and reports the lowest total with the deal code.

### Watch a delivery in real-time

```bash
dominos-pp-cli tracking --phone 2065551234 --watch --interval 30s
```

Streams status transitions every 30 seconds until the order is delivered.

### How much did I spend on pizza this quarter

```bash
dominos-pp-cli analytics --period 90d --agent
```

Aggregates synced order history into spending totals, frequency, and favorite items.

## Auth Setup

Most commands work without authentication: store locator, menu browse, cart building, anonymous order placement, and tracking by phone all succeed unauthenticated. For loyalty rewards, member-exclusive deals, and saved-card payments, the CLI uses a Bearer token via the `DOMINOS_TOKEN` environment variable. Set it with `export DOMINOS_TOKEN=<your-token>` or save it to the config file with `dominos-pp-cli auth set-token <token>`.

The `auth login` subcommand wraps the legacy `/power/login` endpoint (username + password). Domino's actual production auth flow uses an OAuth password grant against `authproxy.dominos.com`, which this CLI does not implement directly — set `DOMINOS_TOKEN` from a token captured via the website if you need authenticated calls.

Auth subcommands: `auth login` (legacy /power/login), `auth set-token` (save to config), `auth status` (show source), `auth logout` (clear stored token).

Run `dominos-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dominos-pp-cli graphql loyalty_points --agent --select Points,Pending
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
dominos-pp-cli feedback "the --interval flag clamps below 10s without a clear error"
dominos-pp-cli feedback --stdin < notes.txt
dominos-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.dominos-pp-cli/feedback.jsonl`. They are never POSTed unless `DOMINOS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DOMINOS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dominos-pp-cli profile save briefing --json
dominos-pp-cli --profile briefing auth login
dominos-pp-cli profile list --json
dominos-pp-cli profile show briefing
dominos-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dominos-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/dominos/cmd/dominos-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/commerce/dominos/cmd/dominos-pp-cli@main
   ```
3. Verify: `dominos-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/dominos/cmd/dominos-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/commerce/dominos/cmd/dominos-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dominos-pp-mcp -- dominos-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dominos-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dominos-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dominos-pp-cli <command> --help`.
