# WooCommerce CLI

**The only WooCommerce CLI with a local database - so it answers the compound questions the Reports API cannot, and reads any store on the internet without credentials.**

WooCommerce's REST API is complete but stateless: it tells you what is true right now, one question at a time. This CLI mirrors your store into local SQLite, which turns questions like stock velocity, refund rate by product, and cohort retention from spreadsheet chores into single commands. It also speaks the public Store API, so `catalog watch` and `catalog diff` work against competitor stores you have no keys for.

## Install

The recommended path installs both the `woocommerce-pp-cli` binary and the `pp-woocommerce` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install woocommerce
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install woocommerce --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install woocommerce --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install woocommerce --agent claude-code
npx -y @mvanhorn/printing-press-library install woocommerce --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/cmd/woocommerce-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/woocommerce-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install woocommerce --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-woocommerce --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-woocommerce --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install woocommerce --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/woocommerce-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WOOCOMMERCE_CONSUMER_KEY`, `WOOCOMMERCE_CONSUMER_SECRET`, and `WOOCOMMERCE_BASE_URL` (`https://your-store.com/wp-json`) when Claude Desktop prompts you. WooCommerce Basic auth needs BOTH halves of the key pair; one alone returns 401.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/cmd/woocommerce-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "woocommerce": {
      "command": "woocommerce-pp-mcp",
      "env": {
        "WOOCOMMERCE_BASE_URL": "https://your-store.com/wp-json",
        "WOOCOMMERCE_CONSUMER_KEY": "<your-key>",
        "WOOCOMMERCE_CONSUMER_SECRET": "<your-secret>"
      }
    }
  }
}
```

</details>

## Authentication

WooCommerce has no central API host - every store serves its own at `https://<your-store>/wp-json/`. Point the CLI at yours with `WOOCOMMERCE_BASE_URL=https://your-store.com/wp-json`, then generate a key pair under WooCommerce > Settings > Advanced > REST API and export `WOOCOMMERCE_CONSUMER_KEY` and `WOOCOMMERCE_CONSUMER_SECRET`. Read scope is enough for every read command. The public catalog commands need no credentials at all and work against any store. Two host-level gotchas worth knowing: some servers strip the Authorization header (you will see a misleading "Consumer key is missing" error over HTTPS), and pretty permalinks must be enabled or `/wp-json/` returns 404 for everything.

## Quick Start

```bash
# confirm the CLI can see a store URL and credentials before anything hits the network
woocommerce-pp-cli doctor --dry-run

# read a public catalog with no credentials at all - this works immediately
woocommerce-pp-cli catalog list --per-page 5

# mirror your own store locally; every analysis command reads from here
woocommerce-pp-cli sync --resources orders,products,customers --since 90d

# the reorder list - units per day divided into current stock
woocommerce-pp-cli stock velocity --window 30d

# what is stuck in the queue right now, and which gateway is failing
woocommerce-pp-cli orders triage

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`orders triage`** — Splits the order queue into actionable buckets and flags payment gateways whose failure rate is climbing.

  _Reach for this instead of listing orders by status when you need to know what is stuck and whether a gateway is degrading._

  ```bash
  woocommerce-pp-cli orders triage --stuck-after 24h --agent
  ```
- **`stock velocity`** — Units sold per day per variation divided into current stock, producing days-of-cover and a reorder list sorted by stockout date.

  _Use this to answer 'what do I reorder' - it is the one number the WooCommerce admin never shows._

  ```bash
  woocommerce-pp-cli stock velocity --window 30d --agent
  ```
- **`revenue explain`** — Breaks a revenue change between two periods into order-count, basket-size, per-product, refund, and coupon contributions.

  _Pick this over a sales report when the question is why a number moved, not what the number is._

  ```bash
  woocommerce-pp-cli revenue explain --window 7d --against prior --agent
  ```
- **`refund-rate`** — Refunded units over sold units for every product and variation, so return-prone items surface by rate rather than raw count.

  _Use this to find margin leaks that a top-sellers report actively hides._

  ```bash
  woocommerce-pp-cli refund-rate --window 90d --limit 20 --agent
  ```
- **`customers ltv`** — Per-customer lifetime value and order count, plus repeat-purchase retention bucketed by first-order month.

  _Reach for this when the question is about customer quality over time rather than a single order._

  ```bash
  woocommerce-pp-cli customers ltv --cohort month --agent
  ```

### Catalog intelligence
- **`catalog audit`** — Deterministic scan for publishable defects: missing images, empty descriptions, absent SKUs, zero prices, and unpriced or stockless variations.

  _Run this before a campaign to catch broken listings while they are still cheap to fix._

  ```bash
  woocommerce-pp-cli catalog audit --agent --select rule,count
  ```
- **`catalog watch`** — Records a timestamped price, sale, and stock snapshot of any public WooCommerce store, with no credentials of any kind.

  _Use this on stores you do not own; every other WooCommerce tool assumes you hold admin keys._

  ```bash
  woocommerce-pp-cli catalog watch --store https://woocommerce.com --max-pages 2 --agent
  ```
- **`catalog diff`** — Compares two recorded snapshots to report added and delisted products, price moves with magnitude, sale windows, and stock flips.

  _This is the payoff of catalog watch: what actually changed, rather than what is true right now._

  ```bash
  woocommerce-pp-cli catalog diff --store https://woocommerce.com --since 30d --agent
  ```

## Recipes

### Find what to reorder before you stock out

```bash
woocommerce-pp-cli stock velocity --window 30d --agent --select sku,units_per_day,stock_quantity,days_of_cover
```

Aggregates 30 days of order line items against current stock and returns only the four fields that drive a purchase order, keeping the response small enough to reason over directly.

### Track a competitor's prices without any credentials

```bash
woocommerce-pp-cli catalog watch --store https://woocommerce.com --max-pages 2 && woocommerce-pp-cli catalog diff --store https://woocommerce.com --since 30d
```

The first command records a snapshot through the public Store API; the second compares it against earlier snapshots to surface price moves, new products, and delistings.

### Explain a bad week

```bash
woocommerce-pp-cli revenue explain --window 7d --against prior --agent
```

Decomposes the revenue delta into order count, average basket, per-product contribution, refunds, and coupon discount, so the cause is a row rather than a hunch.

### Audit a catalog before a campaign

```bash
woocommerce-pp-cli catalog audit --agent --select rule,count,product_ids
```

Returns each defect rule with a count and the offending product IDs, which is directly actionable without opening the admin UI.

### Pull deeply nested order data without drowning in it

```bash
woocommerce-pp-cli orders list --status processing --agent --select id,number,total,line_items.name,line_items.quantity,billing.email
```

Order payloads are large and deeply nested; dotted --select paths narrow them to the handful of fields an agent actually needs.

## Usage

Run `woocommerce-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WOOCOMMERCE_CONFIG_DIR`, `WOOCOMMERCE_DATA_DIR`, `WOOCOMMERCE_STATE_DIR`, or `WOOCOMMERCE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WOOCOMMERCE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WOOCOMMERCE_HOME=/srv/woocommerce
woocommerce-pp-cli doctor
```

Under `WOOCOMMERCE_HOME=/srv/woocommerce`, the four dirs resolve to `/srv/woocommerce/config`, `/srv/woocommerce/data`, `/srv/woocommerce/state`, and `/srv/woocommerce/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "woocommerce": {
      "command": "woocommerce-pp-mcp",
      "env": {
        "WOOCOMMERCE_HOME": "/srv/woocommerce"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WOOCOMMERCE_DATA_DIR` overrides an explicit `--home` for that kind. Use `WOOCOMMERCE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WOOCOMMERCE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `woocommerce-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### all-variations

Store-wide product variation listing (not exposed by the official docs)

- **`woocommerce-pp-cli all-variations`** - List every product variation across the store (undocumented store-wide endpoint).

### attribute-terms

Manage WooCommerce product attribute terms

- **`woocommerce-pp-cli attribute-terms batch`** - Run batch for attribute terms.
- **`woocommerce-pp-cli attribute-terms create`** - Create attribute terms.
- **`woocommerce-pp-cli attribute-terms delete`** - Delete attribute terms.
- **`woocommerce-pp-cli attribute-terms get`** - Get attribute terms.
- **`woocommerce-pp-cli attribute-terms list`** - List attribute terms.
- **`woocommerce-pp-cli attribute-terms update`** - Update attribute terms.

### brands

Manage WooCommerce product brands

- **`woocommerce-pp-cli brands batch`** - Run batch for brands.
- **`woocommerce-pp-cli brands create`** - Create brands.
- **`woocommerce-pp-cli brands delete`** - Delete brands.
- **`woocommerce-pp-cli brands get`** - Get brands.
- **`woocommerce-pp-cli brands list`** - List brands.
- **`woocommerce-pp-cli brands update`** - Update brands.

### catalog

Browse the public WooCommerce Store API catalog

- **`woocommerce-pp-cli catalog collection-data`** - Get collection data for catalog.
- **`woocommerce-pp-cli catalog get`** - Get catalog.
- **`woocommerce-pp-cli catalog list`** - List catalog.

### catalog-reviews

Browse public WooCommerce product reviews

- **`woocommerce-pp-cli catalog-reviews`** - List catalog reviews.

### catalog-taxonomy

Browse public WooCommerce catalog taxonomy

- **`woocommerce-pp-cli catalog-taxonomy attribute`** - Get attribute for catalog taxonomy.
- **`woocommerce-pp-cli catalog-taxonomy attribute-terms`** - Get attribute terms for catalog taxonomy.
- **`woocommerce-pp-cli catalog-taxonomy attributes`** - Get attributes for catalog taxonomy.
- **`woocommerce-pp-cli catalog-taxonomy brands`** - Get brands for catalog taxonomy.
- **`woocommerce-pp-cli catalog-taxonomy categories`** - Get categories for catalog taxonomy.
- **`woocommerce-pp-cli catalog-taxonomy category`** - Get category for catalog taxonomy.
- **`woocommerce-pp-cli catalog-taxonomy tags`** - Get tags for catalog taxonomy.

### categories

Manage WooCommerce product categories

- **`woocommerce-pp-cli categories batch`** - Run batch for categories.
- **`woocommerce-pp-cli categories create`** - Create categories.
- **`woocommerce-pp-cli categories delete`** - Delete categories.
- **`woocommerce-pp-cli categories get`** - Get categories.
- **`woocommerce-pp-cli categories list`** - List categories.
- **`woocommerce-pp-cli categories update`** - Update categories.

### coupons

Manage WooCommerce coupons

- **`woocommerce-pp-cli coupons batch`** - Run batch for coupons.
- **`woocommerce-pp-cli coupons create`** - Create coupons.
- **`woocommerce-pp-cli coupons delete`** - Delete coupons.
- **`woocommerce-pp-cli coupons get`** - Get coupons.
- **`woocommerce-pp-cli coupons list`** - List coupons.
- **`woocommerce-pp-cli coupons update`** - Update coupons.

### customers

Manage WooCommerce customers

- **`woocommerce-pp-cli customers batch`** - Run batch for customers.
- **`woocommerce-pp-cli customers create`** - Create customers.
- **`woocommerce-pp-cli customers delete`** - Delete customers.
- **`woocommerce-pp-cli customers downloads`** - Get downloads for customers.
- **`woocommerce-pp-cli customers get`** - Get customers.
- **`woocommerce-pp-cli customers list`** - List customers.
- **`woocommerce-pp-cli customers update`** - Update customers.

### order-notes

Manage notes attached to WooCommerce orders

- **`woocommerce-pp-cli order-notes create`** - Create order notes.
- **`woocommerce-pp-cli order-notes delete`** - Delete order notes.
- **`woocommerce-pp-cli order-notes get`** - Get order notes.
- **`woocommerce-pp-cli order-notes list`** - List order notes.

### order-refunds

Manage refunds attached to WooCommerce orders

- **`woocommerce-pp-cli order-refunds create`** - Create order refunds.
- **`woocommerce-pp-cli order-refunds delete`** - Delete order refunds.
- **`woocommerce-pp-cli order-refunds get`** - Get order refunds.
- **`woocommerce-pp-cli order-refunds list`** - List order refunds.

### orders

Manage WooCommerce orders and order actions

- **`woocommerce-pp-cli orders batch`** - Run batch for orders.
- **`woocommerce-pp-cli orders create`** - Create orders.
- **`woocommerce-pp-cli orders delete`** - Delete orders.
- **`woocommerce-pp-cli orders email-templates`** - Get email templates for orders.
- **`woocommerce-pp-cli orders get`** - Get orders.
- **`woocommerce-pp-cli orders get-receipt`** - Get get receipt for orders.
- **`woocommerce-pp-cli orders invoice`** - Get invoice for orders.
- **`woocommerce-pp-cli orders list`** - List orders.
- **`woocommerce-pp-cli orders receipt`** - Run receipt for orders.
- **`woocommerce-pp-cli orders send-email`** - Run send email for orders.
- **`woocommerce-pp-cli orders send-order-details`** - Run send order details for orders.
- **`woocommerce-pp-cli orders statuses`** - Get statuses for orders.
- **`woocommerce-pp-cli orders update`** - Update orders.

### payment-gateways

Inspect and update WooCommerce payment gateways

- **`woocommerce-pp-cli payment-gateways get`** - Get payment gateways.
- **`woocommerce-pp-cli payment-gateways list`** - List payment gateways.
- **`woocommerce-pp-cli payment-gateways update`** - Update payment gateways.

### product-attributes

Manage WooCommerce product attributes

- **`woocommerce-pp-cli product-attributes batch`** - Run batch for product attributes.
- **`woocommerce-pp-cli product-attributes create`** - Create product attributes.
- **`woocommerce-pp-cli product-attributes delete`** - Delete product attributes.
- **`woocommerce-pp-cli product-attributes get`** - Get product attributes.
- **`woocommerce-pp-cli product-attributes list`** - List product attributes.
- **`woocommerce-pp-cli product-attributes update`** - Update product attributes.

### products

Manage WooCommerce products

- **`woocommerce-pp-cli products batch`** - Run batch for products.
- **`woocommerce-pp-cli products create`** - Create products.
- **`woocommerce-pp-cli products custom-field-names`** - Get custom field names for products.
- **`woocommerce-pp-cli products delete`** - Delete products.
- **`woocommerce-pp-cli products duplicate`** - Run duplicate for products.
- **`woocommerce-pp-cli products get`** - Get products.
- **`woocommerce-pp-cli products list`** - List products.
- **`woocommerce-pp-cli products related`** - Get related for products.
- **`woocommerce-pp-cli products suggested-products`** - Get suggested products for products.
- **`woocommerce-pp-cli products update`** - Update products.

### refunds

List WooCommerce refunds

- **`woocommerce-pp-cli refunds`** - List refunds.

### reports

Inspect WooCommerce reports

- **`woocommerce-pp-cli reports coupon-totals`** - Get coupon totals for reports.
- **`woocommerce-pp-cli reports customer-totals`** - Get customer totals for reports.
- **`woocommerce-pp-cli reports list`** - List reports.
- **`woocommerce-pp-cli reports order-totals`** - Get order totals for reports.
- **`woocommerce-pp-cli reports product-totals`** - Get product totals for reports.
- **`woocommerce-pp-cli reports review-totals`** - Get review totals for reports.
- **`woocommerce-pp-cli reports sales`** - Get sales for reports.
- **`woocommerce-pp-cli reports top-sellers`** - Get top sellers for reports.

### reviews

Manage WooCommerce product reviews

- **`woocommerce-pp-cli reviews batch`** - Run batch for reviews.
- **`woocommerce-pp-cli reviews create`** - Create reviews.
- **`woocommerce-pp-cli reviews delete`** - Delete reviews.
- **`woocommerce-pp-cli reviews get`** - Get reviews.
- **`woocommerce-pp-cli reviews list`** - List reviews.
- **`woocommerce-pp-cli reviews update`** - Update reviews.

### settings

Inspect and update WooCommerce settings

- **`woocommerce-pp-cli settings batch`** - Run batch for settings.
- **`woocommerce-pp-cli settings get`** - Get settings.
- **`woocommerce-pp-cli settings get-group`** - Get get group for settings.
- **`woocommerce-pp-cli settings group-batch`** - Run group batch for settings.
- **`woocommerce-pp-cli settings list`** - List settings.
- **`woocommerce-pp-cli settings update`** - Update settings.

### shipping-classes

Manage WooCommerce product shipping classes

- **`woocommerce-pp-cli shipping-classes batch`** - Run batch for shipping classes.
- **`woocommerce-pp-cli shipping-classes create`** - Create shipping classes.
- **`woocommerce-pp-cli shipping-classes delete`** - Delete shipping classes.
- **`woocommerce-pp-cli shipping-classes get`** - Get shipping classes.
- **`woocommerce-pp-cli shipping-classes list`** - List shipping classes.
- **`woocommerce-pp-cli shipping-classes slug-suggestion`** - Get slug suggestion for shipping classes.
- **`woocommerce-pp-cli shipping-classes update`** - Update shipping classes.

### shipping-methods

Inspect WooCommerce shipping methods

- **`woocommerce-pp-cli shipping-methods get`** - Get shipping methods.
- **`woocommerce-pp-cli shipping-methods list`** - List shipping methods.

### shipping-zone-locations

Manage WooCommerce shipping zone locations

- **`woocommerce-pp-cli shipping-zone-locations list`** - List shipping zone locations.
- **`woocommerce-pp-cli shipping-zone-locations update`** - Update shipping zone locations.

### shipping-zone-methods

Manage WooCommerce shipping zone methods

- **`woocommerce-pp-cli shipping-zone-methods create`** - Create shipping zone methods.
- **`woocommerce-pp-cli shipping-zone-methods delete`** - Delete shipping zone methods.
- **`woocommerce-pp-cli shipping-zone-methods get`** - Get shipping zone methods.
- **`woocommerce-pp-cli shipping-zone-methods list`** - List shipping zone methods.
- **`woocommerce-pp-cli shipping-zone-methods update`** - Update shipping zone methods.

### shipping-zones

Manage WooCommerce shipping zones

- **`woocommerce-pp-cli shipping-zones create`** - Create shipping zones.
- **`woocommerce-pp-cli shipping-zones delete`** - Delete shipping zones.
- **`woocommerce-pp-cli shipping-zones get`** - Get shipping zones.
- **`woocommerce-pp-cli shipping-zones list`** - List shipping zones.
- **`woocommerce-pp-cli shipping-zones update`** - Update shipping zones.

### store-data

Inspect WooCommerce reference data

- **`woocommerce-pp-cli store-data continent`** - Get continent for store data.
- **`woocommerce-pp-cli store-data continents`** - Get continents for store data.
- **`woocommerce-pp-cli store-data countries`** - Get countries for store data.
- **`woocommerce-pp-cli store-data country`** - Get country for store data.
- **`woocommerce-pp-cli store-data currencies`** - Get currencies for store data.
- **`woocommerce-pp-cli store-data currency`** - Get currency for store data.
- **`woocommerce-pp-cli store-data current-currency`** - Get current currency for store data.
- **`woocommerce-pp-cli store-data list`** - List store data.

### system-status

Inspect WooCommerce system status and tools

- **`woocommerce-pp-cli system-status get`** - Get system status.
- **`woocommerce-pp-cli system-status get-tool`** - Get get tool for system status.
- **`woocommerce-pp-cli system-status list-tools`** - Get list tools for system status.
- **`woocommerce-pp-cli system-status update-tool`** - Update update tool for system status.

### tags

Manage WooCommerce product tags

- **`woocommerce-pp-cli tags batch`** - Run batch for tags.
- **`woocommerce-pp-cli tags create`** - Create tags.
- **`woocommerce-pp-cli tags delete`** - Delete tags.
- **`woocommerce-pp-cli tags get`** - Get tags.
- **`woocommerce-pp-cli tags list`** - List tags.
- **`woocommerce-pp-cli tags update`** - Update tags.

### tax-classes

Manage WooCommerce tax classes

- **`woocommerce-pp-cli tax-classes create`** - Create tax classes.
- **`woocommerce-pp-cli tax-classes delete`** - Delete tax classes.
- **`woocommerce-pp-cli tax-classes get`** - Get tax classes.
- **`woocommerce-pp-cli tax-classes list`** - List tax classes.

### taxes

Manage WooCommerce tax rates

- **`woocommerce-pp-cli taxes batch`** - Run batch for taxes.
- **`woocommerce-pp-cli taxes create`** - Create taxes.
- **`woocommerce-pp-cli taxes delete`** - Delete taxes.
- **`woocommerce-pp-cli taxes get`** - Get taxes.
- **`woocommerce-pp-cli taxes list`** - List taxes.
- **`woocommerce-pp-cli taxes update`** - Update taxes.

### variations

Manage WooCommerce product variations

- **`woocommerce-pp-cli variations batch`** - Run batch for variations.
- **`woocommerce-pp-cli variations create`** - Create variations.
- **`woocommerce-pp-cli variations delete`** - Delete variations.
- **`woocommerce-pp-cli variations generate`** - Run generate for variations.
- **`woocommerce-pp-cli variations get`** - Get variations.
- **`woocommerce-pp-cli variations list`** - List variations.
- **`woocommerce-pp-cli variations update`** - Update variations.

### webhooks

Manage WooCommerce webhooks

- **`woocommerce-pp-cli webhooks batch`** - Run batch for webhooks.
- **`woocommerce-pp-cli webhooks create`** - Create webhooks.
- **`woocommerce-pp-cli webhooks delete`** - Delete webhooks.
- **`woocommerce-pp-cli webhooks get`** - Get webhooks.
- **`woocommerce-pp-cli webhooks list`** - List webhooks.
- **`woocommerce-pp-cli webhooks update`** - Update webhooks.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`woocommerce-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`woocommerce-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`woocommerce-pp-cli learnings list`** - Inspect taught rows
- **`woocommerce-pp-cli learnings forget <query>`** - Undo a teach
- **`woocommerce-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`woocommerce-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`woocommerce-pp-cli teach-pattern`** - Install a query/resource template up front
- **`woocommerce-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WOOCOMMERCE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `woocommerce-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
woocommerce-pp-cli all-variations

# JSON for scripting and agents
woocommerce-pp-cli all-variations --json

# Filter to specific fields
woocommerce-pp-cli all-variations --json --select id,name,status

# Dry run — show the request without sending
woocommerce-pp-cli all-variations --dry-run

# Agent mode — JSON + compact + no prompts in one flag
woocommerce-pp-cli all-variations --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `WOOCOMMERCE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `woocommerce-pp-cli all-variations`
- `woocommerce-pp-cli attribute-terms`
- `woocommerce-pp-cli attribute-terms get`
- `woocommerce-pp-cli attribute-terms list`
- `woocommerce-pp-cli brands`
- `woocommerce-pp-cli brands get`
- `woocommerce-pp-cli brands list`
- `woocommerce-pp-cli catalog`
- `woocommerce-pp-cli catalog get`
- `woocommerce-pp-cli catalog list`
- `woocommerce-pp-cli catalog-reviews`
- `woocommerce-pp-cli catalog-taxonomy`
- `woocommerce-pp-cli categories`
- `woocommerce-pp-cli categories get`
- `woocommerce-pp-cli categories list`
- `woocommerce-pp-cli coupons`
- `woocommerce-pp-cli coupons get`
- `woocommerce-pp-cli coupons list`
- `woocommerce-pp-cli customers`
- `woocommerce-pp-cli customers get`
- `woocommerce-pp-cli customers list`
- `woocommerce-pp-cli orders`
- `woocommerce-pp-cli orders get`
- `woocommerce-pp-cli orders list`
- `woocommerce-pp-cli payment-gateways`
- `woocommerce-pp-cli payment-gateways get`
- `woocommerce-pp-cli payment-gateways list`
- `woocommerce-pp-cli product-attributes`
- `woocommerce-pp-cli product-attributes get`
- `woocommerce-pp-cli product-attributes list`
- `woocommerce-pp-cli products`
- `woocommerce-pp-cli products get`
- `woocommerce-pp-cli products list`
- `woocommerce-pp-cli refunds`
- `woocommerce-pp-cli reports`
- `woocommerce-pp-cli reports list`
- `woocommerce-pp-cli reviews`
- `woocommerce-pp-cli reviews get`
- `woocommerce-pp-cli reviews list`
- `woocommerce-pp-cli settings`
- `woocommerce-pp-cli settings get`
- `woocommerce-pp-cli settings list`
- `woocommerce-pp-cli shipping-classes`
- `woocommerce-pp-cli shipping-classes get`
- `woocommerce-pp-cli shipping-classes list`
- `woocommerce-pp-cli shipping-methods`
- `woocommerce-pp-cli shipping-methods get`
- `woocommerce-pp-cli shipping-methods list`
- `woocommerce-pp-cli shipping-zones`
- `woocommerce-pp-cli shipping-zones get`
- `woocommerce-pp-cli shipping-zones list`
- `woocommerce-pp-cli store-data`
- `woocommerce-pp-cli store-data list`
- `woocommerce-pp-cli system-status`
- `woocommerce-pp-cli system-status get`
- `woocommerce-pp-cli tags`
- `woocommerce-pp-cli tags get`
- `woocommerce-pp-cli tags list`
- `woocommerce-pp-cli tax-classes`
- `woocommerce-pp-cli tax-classes get`
- `woocommerce-pp-cli tax-classes list`
- `woocommerce-pp-cli taxes`
- `woocommerce-pp-cli taxes get`
- `woocommerce-pp-cli taxes list`
- `woocommerce-pp-cli webhooks`
- `woocommerce-pp-cli webhooks get`
- `woocommerce-pp-cli webhooks list`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `WOOCOMMERCE_ATTRIBUTE_ID` resolves `{attribute_id}`

Base URL (default — override with `WOOCOMMERCE_BASE_URL` to target your own store): `https://woocommerce.com/wp-json`

## Health Check

```bash
woocommerce-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `woocommerce-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/woocommerce-pp-cli/config.toml`; `--home`, `WOOCOMMERCE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WOOCOMMERCE_ATTRIBUTE_ID` | endpoint | Yes |  |
| `WOOCOMMERCE_CONSUMER_KEY` | per_call | Yes | Set to your API credential. |
| `WOOCOMMERCE_CONSUMER_SECRET` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `woocommerce-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `woocommerce-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WOOCOMMERCE_CONSUMER_KEY $WOOCOMMERCE_CONSUMER_SECRET`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 with "Consumer key is missing" even though credentials are set and the URL is HTTPS** — The host is stripping the Authorization header. WooCommerce's documented workaround is passing credentials as query parameters - run `woocommerce-pp-cli doctor` to confirm the diagnosis.
- **Every request returns 404, including `/wp-json/`** — Pretty permalinks are disabled. Enable them in Settings > Permalinks; the REST API has no routes without them.
- **PUT or DELETE requests return 501 Method Not Implemented** — ModSecurity is blocking the verb. Ask the host to allow REST verbs, or use the batch endpoints which POST instead.
- **Prices from catalog commands look 100x too large** — Store API prices are integer minor units. The CLI normalizes using each response's currency_minor_unit - if you are reading raw JSON yourself, divide accordingly.
- **Analysis commands return empty results** — They read the local mirror, not the API. Run `woocommerce-pp-cli sync --resources orders,products` first.
- **A list command returns only 10 rows** — That is WooCommerce's default per_page. Pass `--per-page 100` (the API maximum) or let `sync` paginate for you.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**woocommerce-mcp-server**](https://github.com/techspawn/woocommerce-mcp-server) — JavaScript (96 stars)
- [**woocommerce-rest-api-ts-lib**](https://github.com/Yuri-Lima/woocommerce-rest-api-ts-lib) — TypeScript (43 stars)
- [**woocommerce-claude**](https://github.com/woocommerce/woocommerce-claude) — PHP (24 stars)
- [**mcp-for-woocommerce**](https://github.com/iOSDevSK/mcp-for-woocommerce) — PHP (15 stars)
- [**order-fetcher**](https://github.com/JaredReisinger/order-fetcher) — TypeScript (2 stars)
- [**mcp-server-woocommerce**](https://github.com/AmitGurbani/mcp-server-woocommerce) — TypeScript (1 stars)
- [**shopextract**](https://github.com/umerkhan95/shopextract) — Python (1 stars)
- [**wpscrape**](https://github.com/zaidkx37/wpscrape) — Python (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
