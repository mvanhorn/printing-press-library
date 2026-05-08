# Domino's CLI

**Order pizza, browse menus, optimize deals, and track delivery from the terminal — with a local SQLite store that powers reorder, price comparison, and deal stacking no other Domino's tool offers.**

Every Domino's feature you would expect — store locator, menu browse, build cart, validate, price, place, and track — plus a local data layer that compounds. Save named templates with `template save`, find the cheapest store for your order with `compare-prices`, hunt for the best stacked deal with `deals best`, and watch your delivery in real-time with `track --watch`.

## Install

The recommended path installs both the `dominos-pp-cli` binary and the `pp-dominos` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install dominos
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install dominos --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/cmd/dominos-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dominos-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dominos --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dominos --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-dominos skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-dominos. The skill defines how its required CLI can be installed.
```

## Authentication

Most commands work without authentication: store locator, menu browse, cart building, anonymous order placement, and tracking-by-phone all succeed unauthenticated. For loyalty rewards, member-exclusive deals, and order history, run `dominos-pp-cli auth login` — it spawns a Chrome window pointed at dominos.com sign-in, waits for you to complete a normal login (handles captcha and 2FA), then reads the bearer token from sessionStorage and saves it to `~/.config/dominos-pp-cli/config.toml`. The token persists until Domino's expires it (~1 hour). Run `auth status` to confirm; `auth logout` to clear.

## Quick Start

```bash
# 1. Sign in once. Spawns Chrome, you sign in normally, token harvests automatically.
dominos-pp-cli auth login

# 2. Find your closest stores (no auth needed)
dominos-pp-cli stores find --street "709 19th Ave" --city "Seattle WA 98122"

# 3. Get a specific store's profile
dominos-pp-cli stores get 7144

# 4. Fetch the full menu (no auth needed; 68 products, 215 variants)
dominos-pp-cli menu 7144 --json | jq '.Variants | keys | .[0:10]'

# 5. List all coupons available at the store (no auth needed; 58 coupons)
dominos-pp-cli deals list --store-id 7144 --json | jq '.coupons[0:5]'

# 6. Build a cart locally
dominos-pp-cli cart new --store 7144 --service Delivery --address "709 19th Ave, Seattle WA 98122"
dominos-pp-cli cart add 12THIN --qty 1
dominos-pp-cli cart add F_PBITES --qty 1
dominos-pp-cli cart show

# 7. See which coupons auto-apply to YOUR cart
dominos-pp-cli deals best --json
dominos-pp-cli deals eligible --json

# 8. Save as template for future replay
dominos-pp-cli template save friday-night --from-cart

# 9. (Authenticated) See your order history
dominos-pp-cli customer orders --limit 5

# 10. (Authenticated) Loyalty points balance
dominos-pp-cli customer loyalty

# 11. Track an order (no auth — keyed by phone)
dominos-pp-cli track --phone 2065551234 --watch --interval 30s

# 12. Place a saved template (DRY-RUN by default; --confirm to actually place)
dominos-pp-cli order-quick --template friday-night --json
dominos-pp-cli order-quick --template friday-night --confirm --eta-watch --json
```

## Unique Features

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
  dominos-pp-cli compare-prices --street "421 N 63rd St" --city "Seattle WA" --items S_PIZPH,S_LAVA --agent
  ```
- **`stores wait`** — Pull CartEtaMinutes for every store in radius and rank by ETA — the unique GraphQL BFF op every other wrapper ignores.

  _Reach for this when busy-hour delivery decisions need accurate wait estimates rather than a phone call to the store._

  ```bash
  dominos-pp-cli stores wait --street "421 N 63rd St" --city "Seattle WA" --agent
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

## Usage

Run `dominos-pp-cli --help` for the full command reference and flag list.

## Commands

### customer

Customer profile, order history, and loyalty (requires `auth login`)

- **`dominos-pp-cli customer loyalty`** - Loyalty points balance, tier status, and pending points for the customer.
- **`dominos-pp-cli customer orders`** - List the customer's recent orders. Returns full Order objects (Address, Products, Amounts, Coupons, Status, etc.) for analytics, reorder, and order-history workflows.

### graphql

GraphQL BFF operations (discovered via sniff)

- **`dominos-pp-cli graphql categories`** - Get menu categories for a store
- **`dominos-pp-cli graphql create_cart`** - Create a new shopping cart
- **`dominos-pp-cli graphql customer`** - Get authenticated customer profile with saved addresses and preferences
- **`dominos-pp-cli graphql deals_list`** - Get available deals and coupons for a store
- **`dominos-pp-cli graphql get_cart`** - Get cart by ID with items and pricing
- **`dominos-pp-cli graphql loyalty_deals`** - Get member-exclusive deals
- **`dominos-pp-cli graphql loyalty_points`** - Get loyalty points balance and status
- **`dominos-pp-cli graphql loyalty_rewards`** - Get available loyalty rewards by tier
- **`dominos-pp-cli graphql products`** - Get products in a category with customization options
- **`dominos-pp-cli graphql quick_add_product`** - Quick-add a product to cart by code
- **`dominos-pp-cli graphql summary_charges`** - Get cart totals including tax and delivery fee

### menu

Browse store menus and search for items

- **`dominos-pp-cli menu get_menu`** - Get the full menu for a store with categories, products, variants, and toppings

### orders

Create, validate, price, and place orders

- **`dominos-pp-cli orders place`** - Place an order for delivery or carryout
- **`dominos-pp-cli orders price`** - Get the price for an order including taxes and fees
- **`dominos-pp-cli orders validate`** - Validate an order before placing it

### stores

Find and get information about Domino's stores

- **`dominos-pp-cli stores find`** - Find nearby Domino's stores by street and city
- **`dominos-pp-cli stores get`** - Get detailed store information including hours, capabilities, and wait times

### tracking

Track active orders

- **`dominos-pp-cli tracking track`** - Track an order by phone number

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dominos-pp-cli stores get mock-value

# JSON for scripting and agents
dominos-pp-cli stores get mock-value --json

# Filter to specific fields
dominos-pp-cli stores get mock-value --json --select id,name,status

# Dry run — show the request without sending
dominos-pp-cli stores get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dominos-pp-cli stores get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-dominos -g
```

Then invoke `/pp-dominos <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/cmd/dominos-pp-mcp@latest
```

Then register it:

```bash
claude mcp add dominos dominos-pp-mcp -e DOMINOS_USERNAME=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dominos-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DOMINOS_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/cmd/dominos-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dominos": {
      "command": "dominos-pp-mcp",
      "env": {
        "DOMINOS_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
dominos-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dominos-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DOMINOS_USERNAME` | auth_flow_input | No | Domino's account email or phone consumed by `auth login` to obtain a bearer token. Read-only commands work without. |
| `DOMINOS_PASSWORD` | auth_flow_input | No | Set during initial auth setup. |
| `DOMINOS_TOKEN` | harvested | No | Populated automatically by auth login. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dominos-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DOMINOS_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Store finder returns no results** — Domino's geocoder is strict; pass street and city/state separately: `--street "350 5th Ave" --city "New York NY 10118"`.
- **Authenticated commands return 401** — Bearer token expired or missing. Run `auth login` again (token TTL is ~1 hour). Check with `auth status`.
- **auth login times out without harvesting** — Increase `--timeout` (default 5m). Make sure you're actually completing sign-in in the spawned Chrome window.
- **Track command shows order not found** — Use the phone number tied to the order, not your account phone. The tracker is keyed on the order phone field.
- **deals best/eligible returns no fulfilled coupons** — Coupons typically need 2+ items, mix-and-match qualifying combinations. Add more cart items (`cart add <code>`).

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: browser_http (90% confidence)
- Auth signals: bearer_token

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**node-dominos-pizza-api**](https://github.com/RIAEvangelist/node-dominos-pizza-api) — JavaScript (470 stars)
- [**pizzapi-py**](https://github.com/ggrammar/pizzapi) — Python (280 stars)
- [**apizza**](https://github.com/harrybrwn/apizza) — Go (130 stars)
- [**dominos-py**](https://github.com/tomasbasham/dominos) — Python (90 stars)
- [**dawg**](https://github.com/harrybrwn/dawg) — Go (30 stars)
- [**mcpizza**](https://github.com/GrahamMcBain/mcpizza) — Python (25 stars)
- [**pizzamcp**](https://github.com/GrahamMcBain/pizzamcp) — JavaScript (12 stars)
- [**dominos-canada**](https://github.com/Adityasingh22/dominos-canada) — JavaScript (5 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
