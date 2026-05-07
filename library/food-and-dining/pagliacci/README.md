# Pagliacci Pizza CLI

**Order Seattle's favorite pizza from the terminal — every endpoint, plus discount stacking, slice rotation across stores, half-and-half pies, and a small-party planner nobody else has.**

First and only CLI for the Pagliacci API. Browse menus and slice availability across all Seattle stores, build half-and-half pizzas, plan a small-party order, manage rewards and stored coupons, and replay past orders — all with offline search, agent-native output, and Chrome-cookie login (no manual token paste).

## Install

The recommended path installs both the `pagliacci-pp-cli` binary and the `pp-pagliacci` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install pagliacci
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install pagliacci --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci/cmd/pagliacci-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pagliacci-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

Pagliacci has no public API and uses a custom composed `PagliacciAuth {customerId}|{authToken}` header constructed from cookies. Run `pagliacci-pp-cli auth login --chrome` while logged into pagliacci.com in Chrome — the CLI reads the auth cookies and constructs the header for you. Public commands (menu, stores, slices, time-windows) work without login; authenticated commands (order history, rewards, saved addresses) require the Chrome session.

## Quick Start

```bash
# Log in by reading cookies from your active Chrome session
pagliacci-pp-cli auth login --chrome


# Sync stores, menu, slices, orders, rewards into the local SQLite store
pagliacci-pp-cli sync --full


# See what slices are available right now across every Seattle store
pagliacci-pp-cli slices today --agent


# Plan a small-party order: store, time slot, cart contents, best discount
pagliacci-pp-cli orders plan --people 6 --address-label home --json


# Re-create the household's last order as a priced cart, without sending
pagliacci-pp-cli orders reorder --last --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`slices today`** — See which Pagliacci slices are available right now at every Seattle store, sorted by proximity to your saved address.

  _When the family asks 'what slices can we order tonight?', this returns a single comparable list — no per-store iteration needed._

  ```bash
  pagliacci-pp-cli slices today --agent
  ```
- **`rewards stack`** — Compute the best application of stored coupons, reward redemption, and account credit for a given order total. Defaults to single-best-coupon + credit; multi-coupon stacking is flagged --experimental.

  _Family-size orders ($40+) hit reward thresholds where stacking actually saves real money — agents pick the optimal discount, not just the first valid coupon._

  ```bash
  pagliacci-pp-cli rewards stack --order-total 55.00 --agent
  ```
- **`orders summary`** — Aggregate orders over a time range with top items, store breakdown, and order frequency.

  _See the household's pizza pattern — what we order most, which store, how often — for budgeting or just fun._

  ```bash
  pagliacci-pp-cli orders summary --since 90d --agent
  ```

### Time-aware composed lookups
- **`store tonight`** — List stores that are still open and can deliver to your saved address right now, sorted by ETA.

  _Last-minute family dinner: only surface stores that will actually take the order tonight._

  ```bash
  pagliacci-pp-cli store tonight --address-label home --agent
  ```
- **`address best-time`** — Resolve a saved address label to the next available delivery slot in one call.

  _Schedule delivery to land at family dinner time — no separate zone lookup or slot search._

  ```bash
  pagliacci-pp-cli address best-time --label home --agent
  ```

### Order workflows
- **`orders reorder`** — Re-create a past order as a fresh cart, with price revalidation since prices change. Add --send to also submit.

  _Households have a usual order — replay it without rebuilding the cart line by line._

  ```bash
  pagliacci-pp-cli orders reorder --last --dry-run
  ```
- **`menu half-half`** — Build a half-and-half pizza in one command, with each side's toppings validated against the menu and priced via ProductPrice.

  _Families with picky kids order half-and-half pies as the default. One command produces a sendable cart entry for the most common household pizza shape._

  ```bash
  pagliacci-pp-cli menu half-half --left pepperoni --right cheese --size large --json
  ```
- **`orders plan`** — Given the number of people and a saved address, suggest a complete order plan: best store, delivery slot, sized cart contents, and the optimal discount stack.

  _Hosting 4–8 people: one command gives the agent everything it needs to confirm an order — no chained tool calls, no manual compose._

  ```bash
  pagliacci-pp-cli orders plan --people 6 --address-label home --json
  ```

## Usage

Run `pagliacci-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Authentication and registration (no auth required for these endpoints)

- **`pagliacci-pp-cli account confirm_email`** - Confirm a new account by clicking the email-confirmation link's token
- **`pagliacci-pp-cli account create_token`** - Issue a session token (used internally by the SPA for token refresh)
- **`pagliacci-pp-cli account login`** - Authenticate with email/phone + password. Response sets customerId and authToken cookies.
- **`pagliacci-pp-cli account logout`** - Invalidate the current session
- **`pagliacci-pp-cli account password_forgot`** - Request a password reset email
- **`pagliacci-pp-cli account password_reset`** - Reset a password using a token from PasswordForgot email
- **`pagliacci-pp-cli account register`** - Create a new customer account

### address

Address validation and saved address book

- **`pagliacci-pp-cli address create`** - Create a new saved address
- **`pagliacci-pp-cli address delete`** - Delete a saved address
- **`pagliacci-pp-cli address get`** - Get a saved address by ID
- **`pagliacci-pp-cli address get_info`** - Get address info by saved ID
- **`pagliacci-pp-cli address list`** - List the authenticated user's saved addresses
- **`pagliacci-pp-cli address lookup`** - Validate an address and check delivery zone (returns store ID if deliverable)

### cart

Build and price an order before sending it

- **`pagliacci-pp-cli cart get_quote_building`** - Get the current cart/quote-building state by building ID
- **`pagliacci-pp-cli cart price_order`** - Compute the total price for an order (cart contents, taxes, fees, delivery) before sending
- **`pagliacci-pp-cli cart send_order`** - Submit an order. Requires payment information for guests; uses stored payment for authenticated users.
- **`pagliacci-pp-cli cart update_quote_building`** - Update cart contents (add/remove/modify items)

### credit

Account credit balance and entries

- **`pagliacci-pp-cli credit delete`** - Remove an account credit entry
- **`pagliacci-pp-cli credit get`** - Get a single credit entry
- **`pagliacci-pp-cli credit list`** - List the authenticated user's account credit entries

### customer

Customer profile and devices

- **`pagliacci-pp-cli customer access_devices_delete`** - Revoke a device's access to the account
- **`pagliacci-pp-cli customer access_devices_list`** - List devices that have access to this account
- **`pagliacci-pp-cli customer get`** - Get customer profile by ID
- **`pagliacci-pp-cli customer migrate_answer`** - Submit the answer to a migration question
- **`pagliacci-pp-cli customer migrate_question`** - Submit a security/migration question (legacy account migration flow)

### customer_feedback

Customer feedback submissions to Pagliacci

- **`pagliacci-pp-cli customer_feedback get`** - Get a feedback submission by ID
- **`pagliacci-pp-cli customer_feedback submit`** - Submit customer feedback (guest or authenticated)

### gifts

Stored gift cards, balance lookup, and transfer

- **`pagliacci-pp-cli gifts check`** - Check the balance of a gift card by ID and PIN (no auth required to check)
- **`pagliacci-pp-cli gifts delete`** - Remove a stored gift card from the account
- **`pagliacci-pp-cli gifts get`** - Get a single stored gift card by ID
- **`pagliacci-pp-cli gifts list`** - List the authenticated user's stored gift cards
- **`pagliacci-pp-cli gifts transfer`** - Transfer gift card balance to another account
- **`pagliacci-pp-cli gifts value`** - Get current value/balance of a saved gift card

### menu

Menus, slices, and product pricing

- **`pagliacci-pp-cli menu cache`** - Get the full menu (categories, products, prices, descriptions, images) for a store
- **`pagliacci-pp-cli menu product_price`** - Calculate the price for a customized product (size, toppings, modifiers)
- **`pagliacci-pp-cli menu slices`** - Get available slices across all stores for the current day (perishable, rotates daily)
- **`pagliacci-pp-cli menu top`** - Get featured top-of-menu items for a store

### orders

Order history and details

- **`pagliacci-pp-cli orders clone`** - Get order data shaped for re-ordering (transforms a past order into a new cart)
- **`pagliacci-pp-cli orders get`** - Get the full detail of a single past order (items, prices, store, time)
- **`pagliacci-pp-cli orders list`** - List the authenticated user's order history (paginated)
- **`pagliacci-pp-cli orders list_gift_cards`** - List orders that purchased gift cards
- **`pagliacci-pp-cli orders list_pending`** - List orders that are currently in flight (placed but not yet delivered/picked up)
- **`pagliacci-pp-cli orders suggestion`** - Get personalized order suggestions for a customer

### rewards

Loyalty card, rewards history, and stored coupons

- **`pagliacci-pp-cli rewards card`** - Get the authenticated user's reward card balance, points, and available rewards
- **`pagliacci-pp-cli rewards coupon_lookup`** - Look up a coupon by its serial number (validate before applying)
- **`pagliacci-pp-cli rewards history`** - Get reward earning/redemption history (most recent N entries)
- **`pagliacci-pp-cli rewards stored_coupons`** - List coupons saved to the authenticated user's account

### scheduling

Delivery and pickup time windows

- **`pagliacci-pp-cli scheduling slot_list`** - List available time-window slots for a store and service type
- **`pagliacci-pp-cli scheduling slot_list_for_date`** - List allowed slot times for a specific delivery/pickup date (YYYYMMDD)
- **`pagliacci-pp-cli scheduling window_days`** - List available delivery or pickup days for a store. serviceType is DEL (delivery) or PICK (pickup)

### store

Pagliacci store locations, hours, and quote info

- **`pagliacci-pp-cli store compute_quote`** - Compute a quote for a specific store with cart contents (returns Delivery, Drone, Pickup wait values)
- **`pagliacci-pp-cli store get`** - Get a single store by its numeric ID
- **`pagliacci-pp-cli store get_quote`** - Get quote-store metadata (delivery fee, drone status, pickup wait time) for a single store
- **`pagliacci-pp-cli store list`** - List all Pagliacci store locations with addresses, hours, GPS, amenities, and available slices
- **`pagliacci-pp-cli store list_quotes`** - List quote-store metadata (delivery fee, drone status, pickup wait) for all stores

### system

System information and announcements

- **`pagliacci-pp-cli system site_wide_message`** - Get site-wide announcement banner text (closures, holiday hours, etc.)
- **`pagliacci-pp-cli system version`** - Get the current API version


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pagliacci-pp-cli address list

# JSON for scripting and agents
pagliacci-pp-cli address list --json

# Filter to specific fields
pagliacci-pp-cli address list --json --select id,name,status

# Dry run — show the request without sending
pagliacci-pp-cli address list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pagliacci-pp-cli address list --agent
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

## Cookbook

### What's available right now across all stores

```bash
pagliacci-pp-cli slices today --agent
```

Returns one row per (store, slice) pair with `store_id`, `store_name`, `slice_id`, `slice_name`, `price`. Sample size is the union of all stores' current rotation.

### What's available at one store

```bash
pagliacci-pp-cli slices today --store 490 --json
```

`--store` accepts the numeric store ID from `store list` output.

### Where can I order from right now

```bash
pagliacci-pp-cli store tonight --address-label home --agent
```

Lists stores still open and able to deliver to your saved `home` address. Use `--service-type PICK` to filter to pickup-capable stores instead.

### When can I get my order

```bash
pagliacci-pp-cli address best-time --label home --json
```

Resolves the next available delivery slot for the saved `home` address. Pass `--limit 3` to get the next three slots.

### Plan a small-party order

```bash
pagliacci-pp-cli orders plan --people 6 --address-label home --json
```

Composes store choice, delivery slot, sized cart (2.5 slices/person), and best discount stack into one structured response. Use `--people <N>` for any household size.

### Build a half-and-half pizza

```bash
pagliacci-pp-cli menu half-half --left pepperoni --right cheese --size large --json
```

Emits a sendable cart entry. Add `--validate` to additionally call `/ProductPrice` and confirm the unit price for that store.

### Stack discounts before placing an order

```bash
pagliacci-pp-cli rewards stack --order-total 55.00 --agent
```

Picks the optimal coupon + stored credit + reward-points combination for a given pre-discount total. Add `--experimental` to attempt multi-coupon stacking (heuristic; checkout may reject).

### Replay the household's last order

```bash
pagliacci-pp-cli orders reorder --last --dry-run
```

Clones the most recent past order via `/OrderClone` and re-prices via `/OrderPrice` against current menu prices. Drop `--dry-run` and add `--send` to actually submit. Use `--clone-only` to skip the price-revalidation step.

### See the household's ordering rhythm

```bash
pagliacci-pp-cli orders summary --since 90d --agent
```

Aggregates synced orders from the last 90 days: total spent, top items, per-store breakdown, average days between orders. Requires `sync` first. Use `--since 7d`, `30d`, `1y` to change the window.

### Search synced data offline

```bash
pagliacci-pp-cli search "pepperoni" --json --limit 20
```

FTS5 search over synced menu, store, and order data. Add `--type stores` (or another resource name) to scope to one resource. Add `--data-source local` to skip the live-search fallback.

### Sync once, work offline thereafter

```bash
# Initial full pull
pagliacci-pp-cli sync --full

# Refresh only orders going forward
pagliacci-pp-cli sync --resources orders --since 7d
```

`sync --full` ignores the resume cursor; `--resources` scopes to one or more named resources; `--since` does an incremental pull.

### Pipe into jq

```bash
pagliacci-pp-cli store list --json | jq '.[] | select(.City == "Seattle") | .Name'
pagliacci-pp-cli rewards stack --order-total 60 --json | jq '.final_total'
```

`--json` is structured for machine consumption; the schema for each command is its `--help` output.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `PAGLIACCI_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `pagliacci-pp-cli address`
- `pagliacci-pp-cli address create`
- `pagliacci-pp-cli address delete`
- `pagliacci-pp-cli address get`
- `pagliacci-pp-cli address get_info`
- `pagliacci-pp-cli address list`
- `pagliacci-pp-cli address lookup`
- `pagliacci-pp-cli credit`
- `pagliacci-pp-cli credit delete`
- `pagliacci-pp-cli credit get`
- `pagliacci-pp-cli credit list`
- `pagliacci-pp-cli customer`
- `pagliacci-pp-cli customer access_devices_delete`
- `pagliacci-pp-cli customer access_devices_list`
- `pagliacci-pp-cli customer get`
- `pagliacci-pp-cli customer migrate_answer`
- `pagliacci-pp-cli customer migrate_question`
- `pagliacci-pp-cli gifts`
- `pagliacci-pp-cli gifts check`
- `pagliacci-pp-cli gifts delete`
- `pagliacci-pp-cli gifts get`
- `pagliacci-pp-cli gifts list`
- `pagliacci-pp-cli gifts transfer`
- `pagliacci-pp-cli gifts value`
- `pagliacci-pp-cli orders`
- `pagliacci-pp-cli orders clone`
- `pagliacci-pp-cli orders get`
- `pagliacci-pp-cli orders list`
- `pagliacci-pp-cli orders list_gift_cards`
- `pagliacci-pp-cli orders list_pending`
- `pagliacci-pp-cli orders suggestion`
- `pagliacci-pp-cli store`
- `pagliacci-pp-cli store compute_quote`
- `pagliacci-pp-cli store get`
- `pagliacci-pp-cli store get_quote`
- `pagliacci-pp-cli store list`
- `pagliacci-pp-cli store list_quotes`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-pagliacci -g
```

Then invoke `/pp-pagliacci <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/pagliacci/cmd/pagliacci-pp-mcp@latest
```

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
pagliacci-pp-cli auth login --chrome

claude mcp add pagliacci pagliacci-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pagliacci --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pagliacci --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pagliacci skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pagliacci. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
pagliacci-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pagliacci-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/pagliacci/cmd/pagliacci-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pagliacci": {
      "command": "pagliacci-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
pagliacci-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pagliacci-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth login --chrome reports 'no auth cookies found'** — Open pagliacci.com in Chrome and log in. The CLI reads customerId and authToken cookies from the cookie store; if they're missing the session has expired.
- **401 Unauthorized on authenticated commands** — Run `pagliacci-pp-cli auth status`. If cookies are stale, log in again at pagliacci.com and re-run `auth login --chrome`.
- **Empty MenuSlices result during the day** — Slices rotate daily and may be sold out before close. The endpoint reflects current availability at request time.
- **stores tonight returns no rows** — Stores have closed for the night. Use `stores list` for the next-day delivery scope or check `scheduling time-window-days <storeId> DEL` for upcoming windows.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://pagliacci.com/
- Capture coverage: 26 API entries from 26 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: rest_json (98% confidence)
- Auth signals: composed — headers: Authorization — cookies: customerId, authToken
- Generation hints: requires_browser_auth, composed_auth
- Candidate command ideas: store list — GET /Store returned full inventory of locations; menu top — GET /MenuTop/{storeId} drives the home menu UI; menu cache — GET /MenuCache/{storeId} returns the full menu; menu slices — GET /MenuSlices returns today's slices across all stores; address lookup — POST /AddressInfo validates an address and resolves a delivery store; address list — GET /AddressName returns saved addresses; orders list — GET /OrderList/{page}/{size} returns paginated history; orders get — GET /OrderListItem/{id} returns full detail

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
