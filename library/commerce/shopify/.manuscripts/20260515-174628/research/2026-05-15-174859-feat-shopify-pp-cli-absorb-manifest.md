# Shopify CLI Absorb Manifest (Reprint)

## Absorbed (match or beat what already exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List orders (paginated) | Shopify Admin GraphQL | endpoint mirror, --json, --select, cursor | offline cache via sync, FTS5, --csv |
| 2 | Get order by id | Shopify Admin GraphQL | endpoint mirror | served from local store after sync |
| 3 | List products (paginated) | Shopify Admin GraphQL | endpoint mirror | as above |
| 4 | Get product by id | Shopify Admin GraphQL | endpoint mirror | as above |
| 5 | List customers (paginated) | Shopify Admin GraphQL | endpoint mirror | as above |
| 6 | Get customer by id | Shopify Admin GraphQL | endpoint mirror | as above |
| 7 | List inventory items (paginated) | Shopify Admin GraphQL | endpoint mirror | as above |
| 8 | Get inventory item by id | Shopify Admin GraphQL | endpoint mirror | as above |
| 9 | List fulfillment orders (paginated) | Shopify Admin GraphQL | endpoint mirror | as above |
| 10 | Get fulfillment order by id | Shopify Admin GraphQL | endpoint mirror | as above |
| 11 | Bulk operations (run/poll/inspect) | Shopify Admin GraphQL bulkOperationRunQuery | extra_command tree | structured exit codes, --json |
| 12 | Local SQLite store + per-table FTS + ad-hoc SQL | Printing Press generator | framework | always-on, agent-friendly |
| 13 | MCP server (stdio + http) mirroring the Cobra tree | Printing Press generator | framework | remote-capable for hosted agents (Codex stdio, hosted http) |

## Transcendence (only possible with our approach)
| # | Feature | Command | Description | Persona | Group | Score |
|---|---------|---------|-------------|---------|-------|-------|
| 1 | Order status one-shot | `order-status <id-or-name-or-email>` | Resolves order by id / name (#1234) / customer email; returns order + line items + fulfillment + tracking, joined from local store. | Agent operator | agent-native composite | 9 |
| 2 | Low-stock with velocity | `low-stock` | Inventory_items below threshold joined to product/variant title + 30-day sales velocity from local orders; `--days-cover` filter. | Merchant ops + agent | cross-entity local join | 9 |
| 3 | Unfulfilled aging | `unfulfilled-aging` | Fulfillment_orders OPEN/IN_PROGRESS older than H hours, joined to order name + customer email + age. | Merchant ops + agent | cross-entity local join | 8 |
| 4 | Customer history one-shot | `customer-history <id-or-email>` | Customer summary: order count, LTV, last order date, top product by units. | Agent operator | agent-native composite | 8 |
| 5 | Sync freshness | `sync-status` | Per-entity last-synced watermark, row counts, age — agent decides if data is fresh enough. | Agent + merchant | store introspection | 8 |
| 6 | Top products | `top-products` | Ranked top-K products over window by units or revenue, from local orders + line items. | Merchant ops + agent | ranking agg | 8 |
| 7 | Repeat buyers | `repeat-buyers` | Customers with >=N orders in window with order count + LTV joined from local store. | Merchant ops | cohort join | 7 |
| 8 | Revenue by day | `revenue-by-day` | Daily aggregate over local orders: date, order_count, gross, refunds, net, currency. | Agent + merchant | time-series agg | 7 |

All eight: read-only (`mcp:read-only=true`), no external services, no LLM, no write scopes. None re-wrap a single GraphQL endpoint; each exploits the local SQLite store or composes a cross-entity join the absorbed endpoint mirrors cannot deliver in one MCP call.

## User vision applied
Reprint reason: "codex". Top axis = agent-native depth via local-store joins. MCP server is upgraded with `transport: [stdio, http]` so Codex CLI (stdio) and hosted agents (http) both consume the same surface.

## Required behaviors carried forward (patches landed 2026-05-16)

These four corrections were applied after the initial reprint and verified end-to-end against the live Shopify store. They must persist across future reprints; generators should produce code matching these specs, not regenerate the broken originals.

| # | Required behavior | Where | Why |
|---|---|---|---|
| 1 | `shopifyql funnel` returns 3 stages (sessions / checkouts_started / orders) computed as a hybrid: ShopifyQL `SHOW sessions FROM sessions ...` (live) + local SQLite COUNT on `abandoned_checkouts` + `orders` in window. `checkouts_started = abandoned + orders`. Output keys: `sessions, checkouts_started, abandoned_checkouts, orders, session_to_checkout_pct, checkout_to_order_pct, session_to_order_pct, days`. | `internal/cli/shopifyql.go: newShopifyqlFunnelCmd`, `runShopifyqlFunnel`, `extractScalarSessions` | Columns `sessions_with_cart_addition`, `sessions_with_checkout_started`, `sessions_with_checkout_completion` do not exist in any merchant-accessible ShopifyQL version (2026-01, 2026-04 verified). The factory hallucinated them. Local-join is the only honest funnel. |
| 2 | `--since` duration converted to UTC RFC3339 before being placed into `query: "updated_at:>=..."` GraphQL search variable. | `internal/cli/sync.go: sinceTS = ts.UTC().Format(time.RFC3339)` (~line 122) | Local-timezone offsets (e.g. `+08:00`) cause Shopify's search-syntax parser to silently return zero rows. Must be `Z`-suffixed UTC. |
| 3 | Default `--max-pages` is `0` (unlimited), not `10`. | `internal/cli/sync.go: cmd.Flags().IntVar(&maxPages, "max-pages", 0, ...)` | Page cap of 10 x pageSize 50 = 500 records is far too small for any real store. Silently truncates analytics windows. Unlimited by default; users opt in to caps. |
| 4 | `doctor` flags fulfillment-orders access-denial as a token-scope hint (`read_merchant_managed_fulfillment_orders` etc.) rather than letting `sync` fail mysteriously. Optional: skip `fulfillment-orders` from default sync resource list when the token lacks the scope. | `internal/cli/auth.go` / `internal/cli/doctor.go` | Caller cannot diagnose scope error without reading the per-resource sync log. |
| 5 | Every `<resource> list` command whose GraphQL field accepts `$query` MUST wire the `--query` flag through to `variables["query"]`. Affects `orders`, `customers`, `products`, `inventory-items`, `abandoned-checkouts`. Pattern: immediately after the `flagAfter` assignment block, add `if flagQuery != "" { variables["query"] = flagQuery }`. `fulfillment-orders` does NOT accept `$query` and should either drop the flag or surface an error when set; do not wire it. | `internal/cli/<resource>_list.go` (5 files) | The factory's list-command template declares `flagQuery` and registers the cobra flag but never passes it to the variables map. Filter silently ignored, command returns unfiltered results. This is a generator template bug, not a per-resource quirk; fix at the template level so future resources auto-inherit. |

## Generation guidance for future reprints

If the factory's Phase 3 generates ShopifyQL helper commands from the spec or templates, it MUST NOT emit columns from the `sessions` dataset other than `sessions` and `conversion_rate`. Schema introspection of Shopify's ShopifyQL is not the same as GraphQL introspection: there is no machine-discoverable column list, so the generator must hardcode the known-good set: `{sessions, conversion_rate}` from the `sessions` table, plus the documented `sales` table columns (`total_sales`, `average_order_value`, `orders`, `gross_sales`, `net_sales`, `discounts`, `returns`, `taxes`, `shipping`). All other column names in any auto-generated ShopifyQL query are hallucinations.

