---
name: pp-woocommerce
description: "The only WooCommerce CLI with a local database - so it answers the compound questions the Reports API cannot, and reads any store on the internet without credentials. Trigger phrases: `check my WooCommerce orders`, `what should I reorder`, `which products get refunded most`, `track a competitor's WooCommerce prices`, `why did revenue drop this week`, `audit my WooCommerce catalog`, `use woocommerce`, `run woocommerce`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - woocommerce-pp-cli
    install:
      - kind: go
        bins: [woocommerce-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/cmd/woocommerce-pp-cli
---

# WooCommerce — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `woocommerce-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install woocommerce --cli-only
   ```
2. Verify: `woocommerce-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/cmd/woocommerce-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

WooCommerce's REST API is complete but stateless: it tells you what is true right now, one question at a time. This CLI mirrors your store into local SQLite, which turns questions like stock velocity, refund rate by product, and cohort retention from spreadsheet chores into single commands. It also speaks the public Store API, so `catalog watch` and `catalog diff` work against competitor stores you have no keys for.

## When to Use This CLI

Use this CLI for WooCommerce store operations and analysis over the REST API: reading and mutating orders, products, variations, customers, coupons, taxes, shipping, and webhooks, and for the compound analytics that need history - stock velocity, refund rates, customer LTV, revenue decomposition. It is also the right tool for reading any public WooCommerce store's catalog without credentials, including competitor research and price tracking. Reach for it whenever a question spans more than one API call or more than one point in time.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for WordPress content (posts, pages, users, media) - it speaks only the WooCommerce namespaces; use the WordPress REST API or WP-CLI.
- Do not use this CLI for server-side WooCommerce maintenance such as HPOS migration, database updates, attribute lookup table regeneration, or Blueprint import/export - those exist only in WP-CLI and require shell access to the WordPress host, with no REST equivalent.
- Do not use this CLI to install or manage WooCommerce extensions or connect the store to WooCommerce.com - that is WP-CLI's `wc com` family.
- Do not use this CLI to place real customer orders through the storefront checkout - the cart and checkout Store API routes are deliberately excluded.
- Do not expect it to work against a WooCommerce store older than 3.5, which predates the wc/v3 namespace.

## Unique Capabilities

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

## Command Reference

**all-variations** — Store-wide product variation listing (not exposed by the official docs)

- `woocommerce-pp-cli all-variations` — List every product variation across the store (undocumented store-wide endpoint).

**attribute-terms** — Manage WooCommerce product attribute terms

- `woocommerce-pp-cli attribute-terms batch` — Run batch for attribute terms.
- `woocommerce-pp-cli attribute-terms create` — Create attribute terms.
- `woocommerce-pp-cli attribute-terms delete` — Delete attribute terms.
- `woocommerce-pp-cli attribute-terms get` — Get attribute terms.
- `woocommerce-pp-cli attribute-terms list` — List attribute terms.
- `woocommerce-pp-cli attribute-terms update` — Update attribute terms.

**brands** — Manage WooCommerce product brands

- `woocommerce-pp-cli brands batch` — Run batch for brands.
- `woocommerce-pp-cli brands create` — Create brands.
- `woocommerce-pp-cli brands delete` — Delete brands.
- `woocommerce-pp-cli brands get` — Get brands.
- `woocommerce-pp-cli brands list` — List brands.
- `woocommerce-pp-cli brands update` — Update brands.

**catalog** — Browse the public WooCommerce Store API catalog

- `woocommerce-pp-cli catalog collection-data` — Get collection data for catalog.
- `woocommerce-pp-cli catalog get` — Get catalog.
- `woocommerce-pp-cli catalog list` — List catalog.

**catalog-reviews** — Browse public WooCommerce product reviews

- `woocommerce-pp-cli catalog-reviews` — List catalog reviews.

**catalog-taxonomy** — Browse public WooCommerce catalog taxonomy

- `woocommerce-pp-cli catalog-taxonomy attribute` — Get attribute for catalog taxonomy.
- `woocommerce-pp-cli catalog-taxonomy attribute-terms` — Get attribute terms for catalog taxonomy.
- `woocommerce-pp-cli catalog-taxonomy attributes` — Get attributes for catalog taxonomy.
- `woocommerce-pp-cli catalog-taxonomy brands` — Get brands for catalog taxonomy.
- `woocommerce-pp-cli catalog-taxonomy categories` — Get categories for catalog taxonomy.
- `woocommerce-pp-cli catalog-taxonomy category` — Get category for catalog taxonomy.
- `woocommerce-pp-cli catalog-taxonomy tags` — Get tags for catalog taxonomy.

**categories** — Manage WooCommerce product categories

- `woocommerce-pp-cli categories batch` — Run batch for categories.
- `woocommerce-pp-cli categories create` — Create categories.
- `woocommerce-pp-cli categories delete` — Delete categories.
- `woocommerce-pp-cli categories get` — Get categories.
- `woocommerce-pp-cli categories list` — List categories.
- `woocommerce-pp-cli categories update` — Update categories.

**coupons** — Manage WooCommerce coupons

- `woocommerce-pp-cli coupons batch` — Run batch for coupons.
- `woocommerce-pp-cli coupons create` — Create coupons.
- `woocommerce-pp-cli coupons delete` — Delete coupons.
- `woocommerce-pp-cli coupons get` — Get coupons.
- `woocommerce-pp-cli coupons list` — List coupons.
- `woocommerce-pp-cli coupons update` — Update coupons.

**customers** — Manage WooCommerce customers

- `woocommerce-pp-cli customers batch` — Run batch for customers.
- `woocommerce-pp-cli customers create` — Create customers.
- `woocommerce-pp-cli customers delete` — Delete customers.
- `woocommerce-pp-cli customers downloads` — Get downloads for customers.
- `woocommerce-pp-cli customers get` — Get customers.
- `woocommerce-pp-cli customers list` — List customers.
- `woocommerce-pp-cli customers update` — Update customers.

**order-notes** — Manage notes attached to WooCommerce orders

- `woocommerce-pp-cli order-notes create` — Create order notes.
- `woocommerce-pp-cli order-notes delete` — Delete order notes.
- `woocommerce-pp-cli order-notes get` — Get order notes.
- `woocommerce-pp-cli order-notes list` — List order notes.

**order-refunds** — Manage refunds attached to WooCommerce orders

- `woocommerce-pp-cli order-refunds create` — Create order refunds.
- `woocommerce-pp-cli order-refunds delete` — Delete order refunds.
- `woocommerce-pp-cli order-refunds get` — Get order refunds.
- `woocommerce-pp-cli order-refunds list` — List order refunds.

**orders** — Manage WooCommerce orders and order actions

- `woocommerce-pp-cli orders batch` — Run batch for orders.
- `woocommerce-pp-cli orders create` — Create orders.
- `woocommerce-pp-cli orders delete` — Delete orders.
- `woocommerce-pp-cli orders email-templates` — Get email templates for orders.
- `woocommerce-pp-cli orders get` — Get orders.
- `woocommerce-pp-cli orders get-receipt` — Get get receipt for orders.
- `woocommerce-pp-cli orders invoice` — Get invoice for orders.
- `woocommerce-pp-cli orders list` — List orders.
- `woocommerce-pp-cli orders receipt` — Run receipt for orders.
- `woocommerce-pp-cli orders send-email` — Run send email for orders.
- `woocommerce-pp-cli orders send-order-details` — Run send order details for orders.
- `woocommerce-pp-cli orders statuses` — Get statuses for orders.
- `woocommerce-pp-cli orders update` — Update orders.

**payment-gateways** — Inspect and update WooCommerce payment gateways

- `woocommerce-pp-cli payment-gateways get` — Get payment gateways.
- `woocommerce-pp-cli payment-gateways list` — List payment gateways.
- `woocommerce-pp-cli payment-gateways update` — Update payment gateways.

**product-attributes** — Manage WooCommerce product attributes

- `woocommerce-pp-cli product-attributes batch` — Run batch for product attributes.
- `woocommerce-pp-cli product-attributes create` — Create product attributes.
- `woocommerce-pp-cli product-attributes delete` — Delete product attributes.
- `woocommerce-pp-cli product-attributes get` — Get product attributes.
- `woocommerce-pp-cli product-attributes list` — List product attributes.
- `woocommerce-pp-cli product-attributes update` — Update product attributes.

**products** — Manage WooCommerce products

- `woocommerce-pp-cli products batch` — Run batch for products.
- `woocommerce-pp-cli products create` — Create products.
- `woocommerce-pp-cli products custom-field-names` — Get custom field names for products.
- `woocommerce-pp-cli products delete` — Delete products.
- `woocommerce-pp-cli products duplicate` — Run duplicate for products.
- `woocommerce-pp-cli products get` — Get products.
- `woocommerce-pp-cli products list` — List products.
- `woocommerce-pp-cli products related` — Get related for products.
- `woocommerce-pp-cli products suggested-products` — Get suggested products for products.
- `woocommerce-pp-cli products update` — Update products.

**refunds** — List WooCommerce refunds

- `woocommerce-pp-cli refunds` — List refunds.

**reports** — Inspect WooCommerce reports

- `woocommerce-pp-cli reports coupon-totals` — Get coupon totals for reports.
- `woocommerce-pp-cli reports customer-totals` — Get customer totals for reports.
- `woocommerce-pp-cli reports list` — List reports.
- `woocommerce-pp-cli reports order-totals` — Get order totals for reports.
- `woocommerce-pp-cli reports product-totals` — Get product totals for reports.
- `woocommerce-pp-cli reports review-totals` — Get review totals for reports.
- `woocommerce-pp-cli reports sales` — Get sales for reports.
- `woocommerce-pp-cli reports top-sellers` — Get top sellers for reports.

**reviews** — Manage WooCommerce product reviews

- `woocommerce-pp-cli reviews batch` — Run batch for reviews.
- `woocommerce-pp-cli reviews create` — Create reviews.
- `woocommerce-pp-cli reviews delete` — Delete reviews.
- `woocommerce-pp-cli reviews get` — Get reviews.
- `woocommerce-pp-cli reviews list` — List reviews.
- `woocommerce-pp-cli reviews update` — Update reviews.

**settings** — Inspect and update WooCommerce settings

- `woocommerce-pp-cli settings batch` — Run batch for settings.
- `woocommerce-pp-cli settings get` — Get settings.
- `woocommerce-pp-cli settings get-group` — Get get group for settings.
- `woocommerce-pp-cli settings group-batch` — Run group batch for settings.
- `woocommerce-pp-cli settings list` — List settings.
- `woocommerce-pp-cli settings update` — Update settings.

**shipping-classes** — Manage WooCommerce product shipping classes

- `woocommerce-pp-cli shipping-classes batch` — Run batch for shipping classes.
- `woocommerce-pp-cli shipping-classes create` — Create shipping classes.
- `woocommerce-pp-cli shipping-classes delete` — Delete shipping classes.
- `woocommerce-pp-cli shipping-classes get` — Get shipping classes.
- `woocommerce-pp-cli shipping-classes list` — List shipping classes.
- `woocommerce-pp-cli shipping-classes slug-suggestion` — Get slug suggestion for shipping classes.
- `woocommerce-pp-cli shipping-classes update` — Update shipping classes.

**shipping-methods** — Inspect WooCommerce shipping methods

- `woocommerce-pp-cli shipping-methods get` — Get shipping methods.
- `woocommerce-pp-cli shipping-methods list` — List shipping methods.

**shipping-zone-locations** — Manage WooCommerce shipping zone locations

- `woocommerce-pp-cli shipping-zone-locations list` — List shipping zone locations.
- `woocommerce-pp-cli shipping-zone-locations update` — Update shipping zone locations.

**shipping-zone-methods** — Manage WooCommerce shipping zone methods

- `woocommerce-pp-cli shipping-zone-methods create` — Create shipping zone methods.
- `woocommerce-pp-cli shipping-zone-methods delete` — Delete shipping zone methods.
- `woocommerce-pp-cli shipping-zone-methods get` — Get shipping zone methods.
- `woocommerce-pp-cli shipping-zone-methods list` — List shipping zone methods.
- `woocommerce-pp-cli shipping-zone-methods update` — Update shipping zone methods.

**shipping-zones** — Manage WooCommerce shipping zones

- `woocommerce-pp-cli shipping-zones create` — Create shipping zones.
- `woocommerce-pp-cli shipping-zones delete` — Delete shipping zones.
- `woocommerce-pp-cli shipping-zones get` — Get shipping zones.
- `woocommerce-pp-cli shipping-zones list` — List shipping zones.
- `woocommerce-pp-cli shipping-zones update` — Update shipping zones.

**store-data** — Inspect WooCommerce reference data

- `woocommerce-pp-cli store-data continent` — Get continent for store data.
- `woocommerce-pp-cli store-data continents` — Get continents for store data.
- `woocommerce-pp-cli store-data countries` — Get countries for store data.
- `woocommerce-pp-cli store-data country` — Get country for store data.
- `woocommerce-pp-cli store-data currencies` — Get currencies for store data.
- `woocommerce-pp-cli store-data currency` — Get currency for store data.
- `woocommerce-pp-cli store-data current-currency` — Get current currency for store data.
- `woocommerce-pp-cli store-data list` — List store data.

**system-status** — Inspect WooCommerce system status and tools

- `woocommerce-pp-cli system-status get` — Get system status.
- `woocommerce-pp-cli system-status get-tool` — Get get tool for system status.
- `woocommerce-pp-cli system-status list-tools` — Get list tools for system status.
- `woocommerce-pp-cli system-status update-tool` — Update update tool for system status.

**tags** — Manage WooCommerce product tags

- `woocommerce-pp-cli tags batch` — Run batch for tags.
- `woocommerce-pp-cli tags create` — Create tags.
- `woocommerce-pp-cli tags delete` — Delete tags.
- `woocommerce-pp-cli tags get` — Get tags.
- `woocommerce-pp-cli tags list` — List tags.
- `woocommerce-pp-cli tags update` — Update tags.

**tax-classes** — Manage WooCommerce tax classes

- `woocommerce-pp-cli tax-classes create` — Create tax classes.
- `woocommerce-pp-cli tax-classes delete` — Delete tax classes.
- `woocommerce-pp-cli tax-classes get` — Get tax classes.
- `woocommerce-pp-cli tax-classes list` — List tax classes.

**taxes** — Manage WooCommerce tax rates

- `woocommerce-pp-cli taxes batch` — Run batch for taxes.
- `woocommerce-pp-cli taxes create` — Create taxes.
- `woocommerce-pp-cli taxes delete` — Delete taxes.
- `woocommerce-pp-cli taxes get` — Get taxes.
- `woocommerce-pp-cli taxes list` — List taxes.
- `woocommerce-pp-cli taxes update` — Update taxes.

**variations** — Manage WooCommerce product variations

- `woocommerce-pp-cli variations batch` — Run batch for variations.
- `woocommerce-pp-cli variations create` — Create variations.
- `woocommerce-pp-cli variations delete` — Delete variations.
- `woocommerce-pp-cli variations generate` — Run generate for variations.
- `woocommerce-pp-cli variations get` — Get variations.
- `woocommerce-pp-cli variations list` — List variations.
- `woocommerce-pp-cli variations update` — Update variations.

**webhooks** — Manage WooCommerce webhooks

- `woocommerce-pp-cli webhooks batch` — Run batch for webhooks.
- `woocommerce-pp-cli webhooks create` — Create webhooks.
- `woocommerce-pp-cli webhooks delete` — Delete webhooks.
- `woocommerce-pp-cli webhooks get` — Get webhooks.
- `woocommerce-pp-cli webhooks list` — List webhooks.
- `woocommerce-pp-cli webhooks update` — Update webhooks.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `WOOCOMMERCE_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

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

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
woocommerce-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

WooCommerce has no central API host - every store serves its own at `https://<your-store>/wp-json/`. Point the CLI at yours with `WOOCOMMERCE_BASE_URL=https://your-store.com/wp-json`, then generate a key pair under WooCommerce > Settings > Advanced > REST API and export `WOOCOMMERCE_CONSUMER_KEY` and `WOOCOMMERCE_CONSUMER_SECRET`. Read scope is enough for every read command. The public catalog commands need no credentials at all and work against any store. Two host-level gotchas worth knowing: some servers strip the Authorization header (you will see a misleading "Consumer key is missing" error over HTTPS), and pretty permalinks must be enabled or `/wp-json/` returns 404 for everything.

Run `woocommerce-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  woocommerce-pp-cli all-variations --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `WOOCOMMERCE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WOOCOMMERCE_CONFIG_DIR`, `WOOCOMMERCE_DATA_DIR`, `WOOCOMMERCE_STATE_DIR`, `WOOCOMMERCE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WOOCOMMERCE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `woocommerce-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WOOCOMMERCE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WOOCOMMERCE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
woocommerce-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "woocommerce-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `woocommerce-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `woocommerce-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `woocommerce-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
woocommerce-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
woocommerce-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
woocommerce-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
woocommerce-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`woocommerce-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `WOOCOMMERCE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
woocommerce-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
woocommerce-pp-cli feedback --stdin < notes.txt
woocommerce-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WOOCOMMERCE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WOOCOMMERCE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
woocommerce-pp-cli profile save briefing --json
woocommerce-pp-cli --profile briefing all-variations
woocommerce-pp-cli profile list --json
woocommerce-pp-cli profile show briefing
woocommerce-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `woocommerce-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/cmd/woocommerce-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add woocommerce-pp-mcp -- woocommerce-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which woocommerce-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   woocommerce-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `woocommerce-pp-cli <command> --help`.

## Works with pp-wpengine (same machine)

Stores hosted on WP Engine have a hosting layer owned by the sibling CLI `wpengine-pp-cli` (in PATH). Split the work by layer:

- Which install/account hosts this store: `wpengine-pp-cli whois <store-domain> --agent`.
- After catalog/price/stock changes: purge the host cache so the storefront reflects them — `wpengine-pp-cli installs purge-cache create <install_id> --type page` (or `cdn`/`all`).
- Before bulk product mutations: `wpengine-pp-cli guard <install>` creates a checkpoint backup and waits for completion (CI-safe exit codes).
- Storefront SSL/domain problems: `wpengine-pp-cli audit certs` / `audit domains` — that is hosting, not commerce.

Full layer map + inter-CLI choreographies: `~/docs/runbooks/pp-web-stack.md`.
