# Doordash CLI
> Private-beta staging note: this generated Go tree is unofficial/experimental reference material. The active working runtime is the sibling `../active-wrapper/` Node/CycleTLS package. Do not commit credentials or session material. Live cart/order mutations require explicit bricenice17 approval and the CLI safety gates.


Discovered API spec for doordash

Learn more at [Doordash](https://www.doordash.com).
Created by [@bricenice17](https://github.com/bricenice17) (bricenice17).

## Install

This directory is the generated Printing Press Go skeleton/reference tree. It is not the runtime users should install for DoorDash browser-facing calls.

The public-install path for the working DoorDash PP is the repository root Node package, which exposes the same Node/CycleTLS runtime used by Hermes:

```bash
# After bricenice17 approves the visibility flip:
npm install -g github:bricenice17/doordash-pp-cli-clean
doordash-pp-cli --help
doordash-pp-cli doctor --json
```

Until the repo is public, verify from the private checkout root:

```bash
npm ci
npm run build
node dist/cli.js --help
node dist/cli.js doctor --json
```

Do not use `npx -y @mvanhorn/printing-press install doordash`, public release downloads, or generated Go binaries as the working DoorDash install story unless the Go runtime is later ported to the Node/CycleTLS behavior and reverified.


### Without Node

There is no approved no-Node runtime path. The Go skeleton can be built locally for credential-free PP/spec tests, but it is not the working DoorDash runtime.

### Pre-built binary

No standalone pre-built binary is approved. The public-ready install path is the root Node package from the GitHub repo after visibility approval.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Hermes should use the Node/CycleTLS runtime exposed by the root package or the installed wrappers. For private Hermes testing, verify:

```bash
/home/hermes/go/bin/doordash-pp-mcp
/home/hermes/go/bin/doordash-pp-cli doctor --json
```

If this generated skeleton is built for credential-free checks, do not present it as the runtime replacement.

## Install for OpenClaw

For OpenClaw/Paperclip work, point agents at the GitHub/root Node install path once public, or the private checkout before release. Tell them the active runtime is the root package / `active-wrapper/`, not this generated Go tree.

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
doordash-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Safe Commands

```bash
# Health/session metadata only — does not print cookie values
doordash-pp-cli doctor --json
doordash-pp-cli --help

# Read-only DoorDash flows from the generated Go CLI
doordash-pp-cli search "pizza" --json
doordash-pp-cli menu --store-id <store-id> --json
doordash-pp-cli item-options --store-id <store-id> --item-id <item-id> --json
doordash-pp-cli convenience-search --store-id <store-id> "sparkling water" --json
doordash-pp-cli recent-orders --limit 5 --json
doordash-pp-cli addresses --json
doordash-pp-cli payment-methods --json

# Checkout preview; never calls createOrderFromCart
doordash-pp-cli cart preview --variables '{"orderCartId":"cart_123","includeCompanyPaymentInfo":false,"includeRewardBalanceAvailable":false}' --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Read-only DoorDash workflows
- **`search`** — Search DoorDash stores through the consumer GraphQL autocomplete feed without mutating cart or order state.

  _Useful for meal/vendor research while preserving account safety._

  ```bash
  doordash-pp-cli search "thai" --agent
  ```
- **`menu`** — Fetch DoorDash store menus in a normalized shape without changing the cart.

  _Lets agents compare menu choices before any cart mutation is considered._

  ```bash
  doordash-pp-cli menu --store-id STORE_ID --agent
  ```
- **`item-options`** — Inspect item option groups and nested add-ons before adding anything to a cart.

  _Agents can reason about required modifiers before proposing a cart change._

  ```bash
  doordash-pp-cli item-options --store-id STORE_ID --item-id ITEM_ID --agent
  ```
- **`recent-orders`** — Read recent DoorDash order summaries without placing a new order.

  _Supports repeat-order and preference analysis without checkout risk._

  ```bash
  doordash-pp-cli recent-orders --limit 3 --agent
  ```

### Guarded mutation boundary
- **`cart`** — Expose cart inspection and mutations as a separately named, guarded command family rather than mixing them into search/menu reads.

  _Clear command boundaries reduce accidental purchase-flow side effects._

  ```bash
  doordash-pp-cli cart --help
  ```

## Usage

Run `doordash-pp-cli --help` for the full command reference and flag list.

## Commands

### Curated safe commands

- **`doordash-pp-cli search <query>`** — read-only store search.
- **`doordash-pp-cli menu --store-id <id>`** — read-only store menu.
- **`doordash-pp-cli item-options --store-id <id> --item-id <id>`** — read-only option groups.
- **`doordash-pp-cli convenience-search --store-id <id> <query>`** — read-only convenience/grocery item search.
- **`doordash-pp-cli recent-orders --limit 5`** — read-only recent order summary.
- **`doordash-pp-cli addresses`** — read-only saved addresses.
- **`doordash-pp-cli payment-methods`** — read-only payment metadata; never prints full card data.
- **`doordash-pp-cli cart preview --variables '{"orderCartId":"cart_123","includeCompanyPaymentInfo":false,"includeRewardBalanceAvailable":false}'`** — checkout/fee preview; never calls `createOrderFromCart`.
- **`doordash-pp-cli cart place --variables '{}' --enable-live-order-placement --owner-approved --confirm "PLACE DOORDASH ORDER"`** — live order placement; disabled unless every explicit safety gate is provided.

### graphql

The generated Go skeleton preserves the curated low-level GraphQL operation spec for advanced/debug use. The active Hermes wrapper intentionally exposes the safer curated command surface shown above; check `doordash-pp-cli --help` before assuming raw `graphql` subcommands are available in PATH.

- **`doordash-pp-cli graphql create-add-cart-item`** - POST /graphql/addCartItem
- **`doordash-pp-cli graphql create-autocomplete-facet-feed`** - POST /graphql/autocompleteFacetFeed
- **`doordash-pp-cli graphql create-checkout`** - POST /graphql/checkout
- **`doordash-pp-cli graphql create-consumer-order-cart`** - POST /graphql/consumerOrderCart
- **`doordash-pp-cli graphql create-convenience-search-query`** - POST /graphql/convenienceSearchQuery
- **`doordash-pp-cli graphql create-create-order-from-cart`** - POST /graphql/createOrderFromCart
- **`doordash-pp-cli graphql create-delete-cart`** - POST /graphql/deleteCart
- **`doordash-pp-cli graphql create-detailed-cart-items`** - POST /graphql/detailedCartItems
- **`doordash-pp-cli graphql create-get-has-new-notifications`** - POST /graphql/getHasNewNotifications
- **`doordash-pp-cli graphql create-get-open-carts-count`** - POST /graphql/getOpenCartsCount
- **`doordash-pp-cli graphql create-item-page`** - POST /graphql/itemPage
- **`doordash-pp-cli graphql create-list-carts`** - POST /graphql/listCarts
- **`doordash-pp-cli graphql create-poll-order-payment-status`** - POST /graphql/pollOrderPaymentStatus
- **`doordash-pp-cli graphql create-promo-sticky-footer`** - POST /graphql/promoStickyFooter
- **`doordash-pp-cli graphql create-remove-cart-item-v2`** - POST /graphql/removeCartItemV2
- **`doordash-pp-cli graphql create-storepage-feed`** - POST /graphql/storepageFeed
- **`doordash-pp-cli graphql create-total-fee-tally`** - POST /graphql/totalFeeTally
- **`doordash-pp-cli graphql create-update-cart-item-v2`** - POST /graphql/updateCartItemV2
- **`doordash-pp-cli graphql create-validate-consumer-address-with-address-link-id`** - POST /graphql/validateConsumerAddressWithAddressLinkId


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
doordash-pp-cli graphql create-add-cart-item --operation-name example-resource

# JSON for scripting and agents
doordash-pp-cli graphql create-add-cart-item --operation-name example-resource --json

# Filter to specific fields
doordash-pp-cli graphql create-add-cart-item --operation-name example-resource --json --select id,name,status

# Dry run — show the request without sending
doordash-pp-cli graphql create-add-cart-item --operation-name example-resource --dry-run

# Active wrapper: prefer JSON for scripting; inspect --help before using generated-only flags
doordash-pp-cli search pizza --json
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - commands take explicit flags and do not print cookie values.
- **Pipeable** - use `--json` for agent-readable output.
- **Previewable** - cart add/remove support `--dry-run` for no-mutation checks.
- **Confirmable** - cart mutations require `--yes`; order placement also requires `ALLOW_DOORDASH_ORDERING=1` plus `--confirm "PLACE ORDER"`.
- **Runtime-truth first** - the active wrapper may not expose every generated skeleton flag (`--agent`, `--select`, `which`, `agent-context`); inspect `--help` before use.

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-doordash -g
```

Then invoke `/pp-doordash <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


If MCP use is needed before bricenice17 approves public release and the release gate passes, install/register only from the private local checkout or an explicitly approved private build.

Then register it:

```bash
claude mcp add doordash doordash-pp-mcp
```

</details>

## Use with Claude Desktop

Do not install this DoorDash MCP/CLI from `mvanhorn/printing-press-library` releases as the working runtime. After public approval, install the root GitHub Node package and register `doordash-pp-mcp`.

For private local testing, build from this checkout and register the Node/CycleTLS MCP wrapper.

<details>
<summary>Manual JSON config (private local testing only)</summary>

If private MCP use is needed before bricenice17 approves public release and the release gate passes, install/register only from this private local checkout or an explicitly approved private build.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "doordash": {
      "command": "doordash-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
doordash-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/doordash-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

Generated Go skeleton note: this tree uses the generated standard Go HTTP transport and is kept for spec/build/reference checks. The active private-beta runtime uses the Node/CycleTLS wrapper in `../active-wrapper/`; use wrapper help/doctor as runtime truth for browser-facing DoorDash calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.doordash.com/graphql/autocompleteFacetFeed
- Capture coverage: 8 API entries from 8 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: graphql (92% confidence)
- Candidate command ideas: create_addCartItem — Derived from observed POST /graphql/addCartItem traffic.; create_autocompleteFacetFeed — Derived from observed POST /graphql/autocompleteFacetFeed traffic.; create_checkout — Derived from observed POST /graphql/checkout traffic.; create_convenienceSearchQuery — Derived from observed POST /graphql/convenienceSearchQuery traffic.; create_createOrderFromCart — Derived from observed POST /graphql/createOrderFromCart traffic.; create_itemPage — Derived from observed POST /graphql/itemPage traffic.; create_listCarts — Derived from observed POST /graphql/listCarts traffic.; create_storepageFeed — Derived from observed POST /graphql/storepageFeed traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
