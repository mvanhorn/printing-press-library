# Domino's CLI

**Order pizza, browse menus, optimize deals, and track delivery from the terminal - with a local SQLite store that powers reorder, analytics, and price comparison no other Domino's tool offers.**

Every Domino's feature you would expect - store locator, menu browse, validate, price, place, and track - plus a local data layer that compounds. Save named templates with `template save`, find the cheapest store for your order with `compare-prices`, hunt for the best deal with `deals best`, and watch your delivery in real-time with `tracking --watch`.

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/commerce/dominos/cmd/dominos-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dominos-current).

## Authentication

Most commands work without authentication: store locator, menu browse, anonymous order placement, and tracking by phone all succeed unauthenticated. For loyalty rewards, member-exclusive deals, and order history you need a token.

Three options:

```bash
# 1. Interactive login - obtains a token via authproxy.dominos.com and caches it
dominos-pp-cli auth login --u you@example.com --p '<password>'

# 2. Save an existing token to the config file
dominos-pp-cli auth set-token <token>

# 3. Set an env var (overrides the config file)
export DOMINOS_TOKEN=<token>
```

Inspect or clear with `dominos-pp-cli auth status` and `dominos-pp-cli auth logout`.

## Quick Start

```bash
# Verify auth and connectivity
dominos-pp-cli doctor

# Find your closest stores
dominos-pp-cli stores find_stores --s "421 N 63rd St" --c "Seattle WA"

# Get the full structured menu for a store
dominos-pp-cli menu 7094 --json

# Compare item prices across nearby stores and pick the cheapest
dominos-pp-cli compare-prices --address "421 N 63rd St" --city "Seattle WA" --items 14SCREEN,W08PHOTW

# Track an active order by phone number
dominos-pp-cli tracking --phone 2065551234 --watch
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`compare-prices`** - Compare item pricing across every nearby store and find the cheapest one for your order.

  _Pizza prices vary by store. Reach for this when the user cares about saving money or has a flexible delivery radius._

  ```bash
  dominos-pp-cli compare-prices --address "421 N 63rd St" --city "Seattle WA" --items 14SCREEN,W08PHOTW --agent
  ```
- **`template save`** - Save a complete order (store, address, items, toppings, payment) as a named template and replay it with one command.

  _Most users order the same thing repeatedly. Reach for this when the user says 'usual order' or 'what I had last Friday'._

  ```bash
  dominos-pp-cli template save "friday-night" --from-cart ./cart.json
  dominos-pp-cli template order "friday-night"
  ```
- **`deals best`** - Cross-references the cart against every available deal (including loyalty-exclusive) to find the cheapest combination.

  _The headline price often hides a cheaper deal-applied path. Reach for this whenever the cart total is non-trivial._

  ```bash
  dominos-pp-cli deals best --cart cart.json --agent
  ```
- **`menu diff`** - Compare the current menu against the last-synced snapshot to surface new items, removed items, and price changes.

  _Power users want to know when menu items appear or change price. Reach for this for periodic health checks of a favorite store._

  ```bash
  dominos-pp-cli menu diff --store 7094
  ```
- **`analytics`** - Aggregate order history into spending trends, favorite items, order frequency, and average order value over a chosen window.

  _Useful for budgeting and 'how much do I actually spend on pizza?' questions. Reach for this when the user asks for trends._

  ```bash
  dominos-pp-cli analytics --period 90d --top 10 --agent
  ```
- **`reorder`** - Replay your last order against today's menu, automatically substituting unavailable items with the closest match.

  _Menus change. The 'usual' order may no longer be valid verbatim but can be reproduced in spirit. Reach for this when reorder fails strict._

  ```bash
  dominos-pp-cli reorder --last --substitute-unavailable --dry-run
  ```
- **`nutrition`** - Sum calories, protein, fat, and carbs across all items in a cart using the menu's embedded nutrition data.

  _Health-conscious users want to know cart-level totals before ordering. Reach for this when nutrition is mentioned._

  ```bash
  dominos-pp-cli nutrition --cart cart.json --store 7094
  ```
- **`order-bulk`** - Read a CSV of multi-person orders, find the optimal store for the group, and build a combined cart.

  _Group ordering is painful. Reach for this when the user has more than 3 individual orders to place._

  ```bash
  dominos-pp-cli order-bulk --csv ./team-friday.csv --address "123 Main St" --city "Seattle WA"
  ```
- **`stores health`** - Composite store health score combining wait times, hours, service capabilities, and historical delivery performance.

  _Two stores at similar distances can have very different ETAs. Reach for this when the user is choosing between options._

  ```bash
  dominos-pp-cli stores health 7094
  ```

### Agent-native plumbing

- **`tracking --watch`** - Polls the tracker endpoint at a chosen interval and streams status updates: prep, bake, quality check, out for delivery.

  _Agents and users alike want a non-blocking 'tell me when the pizza arrives' primitive. Reach for this immediately after placing an order._

  ```bash
  dominos-pp-cli tracking --phone 2065551234 --watch --interval 30s
  ```

## Usage

Run `dominos-pp-cli --help` for the full command reference and flag list.

## Commands

### Stores and menus

| Command | What it does |
|---------|--------------|
| `stores find_stores --s <street> --c <city>` | Find nearby stores |
| `stores get_store <storeID>` | Get hours, capabilities, wait times |
| `stores health <storeID>` | Composite 0-100 store health score |
| `menu <storeID>` | Get the full structured menu |
| `menu diff --store <storeID>` | Diff against the last synced snapshot |

### Ordering

| Command | What it does |
|---------|--------------|
| `orders validate_order --order @cart.json` | Validate before pricing |
| `orders price_order --order @cart.json` | Price including tax and fees |
| `orders place_order --order @cart.json` | Place the order |
| `tracking --phone <number>` | Track an order (use `--watch` to poll) |
| `reorder --last` | Replay the last template against today's menu |
| `order-bulk --csv <file>` | Group ordering from CSV |

### Compounding capabilities

| Command | What it does |
|---------|--------------|
| `compare-prices --items <codes>` | Pick the cheapest nearby store |
| `deals best --cart <file>` | Find best deal/loyalty combo for the cart |
| `template save <name> --from-cart <file>` | Save a named order |
| `template list` / `template show <name>` / `template order <name>` | Replay saved templates |
| `analytics --period 30d` | Spending and order-cadence trends |
| `nutrition --cart <file>` | Cart-level calorie/macro totals |
| `sync` | Hydrate the local SQLite store |
| `tail --resources orders` | Stream live API changes |

### GraphQL BFF

The Domino's BFF surfaces loyalty and cart operations not in the legacy REST API:

| Command | What it does |
|---------|--------------|
| `graphql customer` | Authenticated profile with saved addresses |
| `graphql categories` | Menu categories for a store |
| `graphql products` | Products with customization options |
| `graphql create_cart` / `graphql get_cart` | Manage BFF carts |
| `graphql quick_add_product` | Add a product by code |
| `graphql deals_list` / `graphql loyalty_deals` | Public deals + member-exclusive deals |
| `graphql loyalty_points` / `graphql loyalty_rewards` | Loyalty status and rewards |
| `graphql summary_charges` | Cart totals including tax and delivery fee |

### Auth and config

| Command | What it does |
|---------|--------------|
| `auth login --u <email> --p <password>` | Interactive login |
| `auth set-token <token>` | Save an existing token |
| `auth status` / `auth logout` | Inspect / clear credentials |
| `profile save <name>` / `profile list` / `profile use <name>` | Saved flag profiles |
| `doctor` | Health check |
| `agent-context` | JSON description of the CLI for agents |
| `which <capability>` | Find which command implements a capability |


## Output Formats

```bash
# Human-readable table (default in terminal)
dominos-pp-cli template list

# JSON for scripting and agents
dominos-pp-cli template list --json

# Filter to specific fields
dominos-pp-cli stores find_stores --s "421 N 63rd St" --c "Seattle WA" --json --select StoreID,Phone,IsOpen

# CSV for spreadsheets
dominos-pp-cli analytics --csv

# Compact mode strips verbose fields for minimal token usage
dominos-pp-cli menu 7094 --json --compact

# Dry run shows the request without sending
dominos-pp-cli orders place_order --order @cart.json --dry-run

# Agent mode = --json + --compact + --no-input + --no-color + --yes
dominos-pp-cli compare-prices --address "421 N 63rd St" --city "Seattle WA" --items 14SCREEN --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add dominos dominos-pp-mcp -e DOMINOS_TOKEN=<your-key>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dominos": {
      "command": "dominos-pp-mcp",
      "env": {
        "DOMINOS_TOKEN": "<your-key>"
      }
    }
  }
}
```

## Health Check

```bash
dominos-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dominos-pp-cli/config.toml`
Local SQLite store: `~/.local/share/dominos-pp-cli/data.db`

Environment variables:
- `DOMINOS_TOKEN` - OAuth bearer token for loyalty rewards, member deals, and order history (optional - most commands work without auth)
- `DOMINOS_BASE_URL` - Override the API base URL (default: `https://order.dominos.com`)
- `DOMINOS_CONFIG` - Override the config file path

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dominos-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DOMINOS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Store finder returns no results** - Domino's geocoder is strict; pass street and city/state separately to `find_stores`: `--s "421 N 63rd St" --c "Seattle WA"`. The `compare-prices` command uses the same split with `--address` and `--city`.
- **Menu fetch returns 403** - Domino's blocks non-US IPs. Run `dominos-pp-cli doctor` to verify connectivity. Use a US-located machine or VPN.
- **auth login fails with 401** - Most often a typo in email/password or a 2FA challenge. Try `dominos-pp-cli auth set-token <token>` if you have a token from another source, or `auth status` to inspect the current state.
- **`tracking` shows order not found** - Use the phone number tied to the order, not your account phone. The tracker is keyed on the order phone field.

---

## Cookbook

### Find your closest store

```bash
dominos-pp-cli stores find_stores --s "421 N 63rd St" --c "Seattle WA" --json | jq '.Stores[0].StoreID'
# -> "7094"
```

### Browse a full store menu

```bash
# Fetch the structured menu with categories, products, variants, and toppings
dominos-pp-cli menu 7094 --json | jq '.Menu.Products | keys'
```

### Validate, price, then place an order from a cart JSON

```bash
# Pipe a cart JSON through validate -> price -> place
cat cart.json | dominos-pp-cli orders validate_order --stdin --json
cat cart.json | dominos-pp-cli orders price_order --stdin --json | jq '.Order.AmountsBreakdown.Customer'

# Always dry-run first
dominos-pp-cli orders place_order --order @cart.json --dry-run
```

### Find the cheapest store for your order

```bash
# Syncs menus from up to N nearby stores and joins on item codes locally
dominos-pp-cli compare-prices --address "421 N 63rd St" --city "Seattle WA" --items 14SCREEN,W08PHOTW --max-stores 5
```

### Watch a live order

```bash
# Polls the tracker endpoint and streams status: prep, bake, quality check, out for delivery
dominos-pp-cli tracking --phone 2065551234 --watch --interval 30s
```

### Save and replay favorite orders

```bash
# Save a template directly from a cart JSON file
dominos-pp-cli template save "friday-night" --from-cart ./cart.json --description "Usual large pepperoni"

# List, inspect, and replay
dominos-pp-cli template list
dominos-pp-cli template show "friday-night"
dominos-pp-cli template order "friday-night"
```

### Reorder with smart substitution

```bash
# Replays the last template; substitutes items that are no longer on the menu
dominos-pp-cli reorder --last --substitute-unavailable --dry-run
```

### Find the best deal for your cart

```bash
# Cross-references cart against all available deals plus loyalty-exclusive deals
dominos-pp-cli deals best --cart cart.json --include-loyalty --json

# Inspect a store's deals without matching against a cart
dominos-pp-cli deals best --store 7094 --top 10
```

### Track menu changes over time

```bash
# Diffs current menu against the last synced snapshot, then updates the snapshot
dominos-pp-cli menu diff --store 7094

# Compare without updating the baseline
dominos-pp-cli menu diff --store 7094 --no-update --json
```

### Spending analytics from synced order history

```bash
# Sync first
dominos-pp-cli sync --resources orders

# Aggregate: spending, frequency, AOV, favorite items
dominos-pp-cli analytics --period 30d --top 10 --json
```

### Group ordering with a CSV

```bash
# Each row is a person + items; the CLI builds a combined cart and picks the best store
dominos-pp-cli order-bulk --csv ./team-friday.csv --address "123 Main St" --city "Seattle WA" --output combined-cart.json
```

### Score a store before ordering

```bash
dominos-pp-cli stores health 7094
```

### Pre-order nutrition totals

```bash
dominos-pp-cli nutrition --cart cart.json --store 7094
```

### Drive everything through saved profiles

```bash
# Capture the global flags you reuse every time
dominos-pp-cli profile save agent --json --compact --yes --description "Default agent flags"

# Apply the profile on any command
dominos-pp-cli compare-prices --profile agent --address "421 N 63rd St" --city "Seattle WA" --items 14SCREEN
```

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**node-dominos-pizza-api**](https://github.com/RIAEvangelist/node-dominos-pizza-api) — JavaScript (470 stars)
- [**pizzapi-py**](https://github.com/ggrammar/pizzapi) — Python (280 stars)
- [**apizza**](https://github.com/harrybrwn/apizza) — Go (130 stars)
- [**dominos-py**](https://github.com/tomasbasham/dominos) — Python (90 stars)
- [**dawg**](https://github.com/harrybrwn/dawg) — Go (50 stars)
- [**mcpizza**](https://github.com/GrahamMcBain/mcpizza) — Python (25 stars)
- [**pizzamcp**](https://github.com/GrahamMcBain/pizzamcp) — JavaScript (18 stars)
- [**dominos-canada**](https://github.com/RIAEvangelist/node-dominos-pizza-api) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
