---
name: pp-dominos
description: "Order pizza, browse menus, optimize deals, and track delivery from the terminal — with a local SQLite store that powers reorder, price comparison, and deal stacking no other Domino's tool offers. Trigger phrases: `order a pizza`, `find a domino's near me`, `track my pizza`, `what's my pizza usual`, `best deal on my pizza order`, `compare pizza prices`, `use dominos`, `run dominos`."
version: "3.9.1+pr639+dominos-rebase"
author: "Trevin Chow"
license: "Apache-2.0"
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
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/cmd/dominos-pp-cli
---

# Domino's — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dominos-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install dominos --cli-only
   ```
2. Verify: `dominos-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/cmd/dominos-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or power user needs to interact with Domino's outside a browser — building, pricing, and placing orders, tracking deliveries, comparing prices across stores, optimizing deal selection, or replaying saved order templates. Excellent for automation: every command supports --json, --dry-run, --agent, --select, and structured exit codes. Local SQLite store enables features the public API cannot serve directly (deal optimization, multi-store wait-time comparison, named order templates). DO NOT use this CLI for: other pizza chains (Pizza Hut, Papa John's, etc.), generic food delivery (DoorDash, Uber Eats), restaurant search/aggregation, or non-US Domino's storefronts (only US endpoints are supported).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`template save`** — Save your usual order — store, address, items, toppings, payment ref — and replay it with one command.

  _Reach for this when an agent or user wants a one-command repeat of a known-good order without rebuilding the cart from scratch._

  ```bash
  dominos-pp-cli template save friday-night --from-cart && dominos-pp-cli template order friday-night --eta-watch --json
  ```
- **`reorder`** — Replay your last order against today's menu; FTS-substitute items the menu rotated out so the order still goes through.

  _Reach for this when a saved-order-style replay must survive Domino's menu rotation; substitution avoids the 'this item is no longer available' failure._

  ```bash
  dominos-pp-cli reorder --last --substitute-unavailable --dry-run --json
  ```
- **`deals best`** — Cross-reference your cart against every available deal (incl. loyalty-exclusive) and report the lowest-priced combination.

  _Reach for this before placing an order whenever the user cares about price; the flag enumerates 2-3-deal combinations the website never surfaces._

  ```bash
  dominos-pp-cli deals best --agent
  ```
- **`analytics`** — Aggregate your synced order history into spending totals, item frequency, favorite stores, and average order value.

  _Reach for this when 'how much have I spent on pizza this quarter' or 'what are my top 3 items' is the question. Powered entirely by the local SQLite order history._

  ```bash
  dominos-pp-cli analytics --period 90d --group-by item --agent
  ```

### Cross-source insights
- **`compare-prices`** — Same cart priced at every nearby store; rank by total including delivery fee.

  _Reach for this when latency-or-price tradeoffs across nearby stores matter (delivery fee + wait time can offset a cheaper menu)._

  ```bash
  dominos-pp-cli compare-prices --address "421 N 63rd St" --city "Seattle WA" --items S_PIZPH,S_LAVA --agent
  ```
- **`stores wait`** — Pull CartEtaMinutes for every store in radius and rank by ETA — the unique GraphQL BFF op every other wrapper ignores.

  _Reach for this when busy-hour delivery decisions need accurate wait estimates rather than a phone call to the store._

  ```bash
  dominos-pp-cli stores wait --address "421 N 63rd St" --city "Seattle WA" --agent
  ```
- **`deals eligible`** — List which advertised deals actually apply to your current cart and explain why each non-matching one fails.

  _Reach for this when the user is hunting for a coupon and needs to understand the predicate gap, not just whether 'a deal' applies._

  ```bash
  dominos-pp-cli deals eligible --agent
  ```

### Agent-native plumbing
- **`track`** — Stream Domino's tracker stages — placed → prep → bake → quality check → out → delivered — until the order arrives.

  _Reach for this when an agent or user wants to know precisely when a placed order changes stage without holding open a browser tab._

  ```bash
  dominos-pp-cli track --phone 2065551234 --watch --interval 30s --agent
  ```
- **`order-quick`** — Replay a template, validate, price, place (with --confirm), and tail the tracker — all in one command emitting a final JSON envelope.

  _Reach for this when an agent or automation wants to trigger an order and exit cleanly with structured `{order_id, eta_min, total, tracker_phone}` output._

  ```bash
  dominos-pp-cli order-quick --template friday-night --confirm --eta-watch --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries
- Auth signals: bearer_token

## Command Reference

**customer** — Customer profile, order history, and loyalty (requires `auth login`)

- `dominos-pp-cli customer loyalty` — Loyalty points balance, tier status, and pending points for the customer.
- `dominos-pp-cli customer orders` — List the customer's recent orders. Returns full Order objects (Address, Products, Amounts, Coupons, Status, etc.)...

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

- `dominos-pp-cli menu <storeID>` — Get the full menu for a store with categories, products, variants, and toppings

**orders** — Create, validate, price, and place orders

- `dominos-pp-cli orders place` — Place an order for delivery or carryout
- `dominos-pp-cli orders price` — Get the price for an order including taxes and fees
- `dominos-pp-cli orders validate` — Validate an order before placing it

**stores** — Find and get information about Domino's stores

- `dominos-pp-cli stores find` — Find nearby Domino's stores by address
- `dominos-pp-cli stores get` — Get detailed store information including hours, capabilities, and wait times

**tracking** — Track active orders

- `dominos-pp-cli tracking` — Track an order by phone number


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dominos-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Full real-world flow: closest store → menu → cart → deals

```bash
# 1. Find closest store (no auth needed)
dominos-pp-cli stores find --address "709 19th Ave" --city "Seattle WA 98122" --json

# 2. List all coupons available at that store (no auth needed; 58 coupons)
dominos-pp-cli deals list --store-id 7144 --json

# 3. Build a cart (local, no API call)
dominos-pp-cli cart new --store 7144 --service Delivery --address "709 19th Ave, Seattle WA 98122"
dominos-pp-cli cart add 12THIN --qty 1     # Medium 12" Thin Pizza
dominos-pp-cli cart add F_PBITES --qty 1   # Parmesan Bread Bites

# 4. See which coupons auto-apply (real auto-couponing-service call)
dominos-pp-cli deals best --json
dominos-pp-cli deals eligible --json
```

Every step talks to a real Domino's endpoint and returns real data. The deals call correctly identifies which store coupons fulfill your specific cart.

### Friday night reorder, hands-off

```bash
dominos-pp-cli order-quick --template friday-night --confirm --eta-watch --json
```

Replay a saved template, validate, price, place (with --confirm), and tail the tracker. Emits a final JSON envelope with order_id, eta_min, and total. Requires a template named friday-night — see `template save`. **Without `--confirm`, returns a dry-run preview (no order placed).**

### Authenticated commands (after `auth login`)

```bash
# Order history
dominos-pp-cli customer orders --limit 5 --json

# Loyalty points balance
dominos-pp-cli customer loyalty --json
```

Customer ID is the long base64-style identifier from your sign-in. Once `auth login` completes, the bearer token persists in `~/.config/dominos-pp-cli/config.toml` until Dominos expires it (~1 hour).

### Find the cheapest store for tonight's order (agent + select for narrow output)

```bash
dominos-pp-cli compare-prices --address "421 N 63rd St" --city "Seattle WA" --items S_PIZPH,S_LAVA --agent --select results[].store_id,results[].total_cents
```

Surveys nearby stores and returns only the comparison fields the agent needs.

### Hunt for the best stacked deal

```bash
dominos-pp-cli deals best --agent
```

Tries every available deal — including 2-3 deal stacked combinations — against the active cart and reports the lowest-priced combination plus the deal codes.

### Watch a delivery in real-time

```bash
dominos-pp-cli track --phone 2065551234 --watch --interval 30s --agent
```

Streams status transitions every 30 seconds (placed → prep → bake → qc → out → delivered) until the order arrives. Exits 0 on delivered.

### Why is this deal not applying to my cart?

```bash
dominos-pp-cli deals eligible --agent
```

Lists deals against the active cart and explains eligibility against each deal's predicates.

## Auth Setup

Most commands work without authentication: store locator, menu browse, cart building, anonymous order placement, and tracking-by-phone all succeed unauthenticated. For loyalty rewards, member-exclusive deals, and order history, run `dominos-pp-cli auth login` — it spawns a Chrome window pointed at dominos.com sign-in, waits for you to complete a normal login (handles captcha and 2FA), then reads the bearer token from sessionStorage and saves it to `~/.config/dominos-pp-cli/config.toml`. The token persists until Domino's expires it (~1 hour). Run `auth status` to confirm; `auth logout` to clear.

Run `dominos-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dominos-pp-cli stores get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
dominos-pp-cli feedback "menu sync skipped nutrition fields for half the items"
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
dominos-pp-cli --profile briefing stores get mock-value
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
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/cmd/dominos-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dominos-pp-mcp -- dominos-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dominos-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dominos-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dominos-pp-cli <command> --help`.
