# Shopify CLI Absorb Manifest (Reprint, v4.28.0)

## Absorbed (match or beat what already exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List orders (paginated) | Shopify Admin GraphQL | (generated endpoint) orders list | offline cache via sync, FTS5, --csv |
| 2 | Get order by id | Shopify Admin GraphQL | (generated endpoint) orders get | served from local store after sync |
| 3 | List products (paginated) | Shopify Admin GraphQL | (generated endpoint) products list | as above |
| 4 | Get product by id | Shopify Admin GraphQL | (generated endpoint) products get | as above |
| 5 | List customers (paginated) | Shopify Admin GraphQL | (generated endpoint) customers list | as above |
| 6 | Get customer by id | Shopify Admin GraphQL | (generated endpoint) customers get | as above |
| 7 | List inventory items (paginated) | Shopify Admin GraphQL | (generated endpoint) inventory-items list | as above |
| 8 | Get inventory item by id | Shopify Admin GraphQL | (generated endpoint) inventory-items get | as above |
| 9 | List fulfillment orders (paginated) | Shopify Admin GraphQL | (generated endpoint) fulfillment-orders list | as above |
| 10 | Get fulfillment order by id | Shopify Admin GraphQL | (generated endpoint) fulfillment-orders get | as above |
| 11 | List abandoned checkouts | Shopify Admin GraphQL | (generated endpoint) abandoned-checkouts list | as above |
| 12 | Bulk operations (run/poll/inspect) | Shopify Admin GraphQL bulkOperationRunQuery | shopify-pp-cli bulk-operations current / run-query | structured exit codes, --json |
| 13 | Local SQLite store + per-table FTS + ad-hoc SQL | Printing Press generator | framework | always-on, agent-friendly |
| 14 | MCP server (stdio + http) mirroring the Cobra tree | Printing Press generator | framework | remote-capable (Codex stdio, hosted http) |
| 15 | Revenue / channel / lifecycle analytics | Prior shipped patch | shopify-pp-cli report revenue-daily / channel-mix / show-impact / attach-rate / customer-lifecycle | local-store compound analytics |
| 16 | ShopifyQL passthrough + funnel | Prior shipped patch | shopify-pp-cli shopifyql query / funnel / sessions / conversion | real-column funnel (hybrid ShopifyQL + local join), verified against live store |
| 17 | Store daily brief / health audit | Prior shipped patch | shopify-pp-cli store daily-brief / audit | executive summary from local mirror |
| 18 | Growth workflows | Prior shipped patch | shopify-pp-cli growth campaign-brief / winback-candidates / vip-segments | local-store segmentation |
| 19 | Ops risk workflows | Prior shipped patch | shopify-pp-cli ops fulfillment-risk / shipping-anomalies | local-store risk detection |
| 20 | Merchandising workflows | Prior shipped patch | shopify-pp-cli merchandising bundle-opportunities / dead-stock-actions / launch-brief | co-purchase / inventory-driven suggestions |
| 21 | Order/customer tag mutations | Prior shipped patch | shopify-pp-cli orders tag / customers tag | (flagged — see Reprint Verdicts: confirm write scopes are actually granted) |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|-------------------|
| 1 | Order support brief | `orders brief <id>` | hand-code | Joins order + fulfillment-order status + inventory availability from local store in one agent-facing call; no single GraphQL call returns this shape. | Use this command for a single support-ready order summary including fulfillment and inventory-availability context. Do NOT use this command for the raw order object; use 'orders get' instead. |
| 2 | Sync content diff | `store diff --since <sync-run>` | hand-code | Reads local SQLite filtered by updated_at against the last sync watermark — a pure local-data delta view no live API call can answer. | Use this command to see which rows changed content since the last sync. Do NOT use this command for sync freshness or row counts; use 'workflow status' instead. |
| 3 | Bulk operation blocking wait | `bulk-operations wait` | hand-code | Polls the existing currentBulkOperation query until a terminal state instead of forcing the caller to hand-loop. | Use this command to block until the current bulk operation finishes and return its terminal result. Do NOT use this command for a non-blocking snapshot of job state; use 'bulk-operations current' instead. |
| 4 | GraphQL cost/throttle check | `doctor throttle` | hand-code | Surfaces Shopify's own extensions.cost.throttleStatus block so an agent can pre-flight-check budget before a heavy batch/agent session. | Use this command to check remaining GraphQL query-cost budget before a heavy batch or agent session. Do NOT use this command for general auth/connectivity health; use 'doctor' instead. |

## Stub items
None. All 4 new transcendence rows ship fully (no stubs).

## Reprint Verdicts (see full detail in 2026-07-11-170430-novel-features-brainstorm.md)

- Absorbed rows 1-14: Keep, still core/foundational.
- Already-shipped analytics/growth/ops/merchandising themes (rows 15-20): Keep.
- Row 21 (order/customer tag mutations): Keep, but **flag for verification** — confirm `write_orders`/`write_customers` scopes are actually granted on the token before relying on these in a read-only deployment.
- **Known issue carried forward, not fixed in this reprint (out of hand-code scope, flagged for user/retro):** the shipped OAuth `client_credentials` auth-audit command appears to model the wrong grant type — Shopify custom-app Admin API auth is a static `X-Shopify-Access-Token` header, not an OAuth client_credentials flow. Recommend follow-up review/fix; not blocking this reprint's ship decision since it's a pre-existing shipped command being reconciled, not part of the 4 new hand-code rows.

## Hand-code commitment
4 of 4 new transcendence features require hand-written Go after generate (~50-150 LoC each plus root.go wiring): `orders brief`, `store diff`, `bulk-operations wait`, `doctor throttle`. 0 auto-emitted from spec (all 4 are hand-code).
