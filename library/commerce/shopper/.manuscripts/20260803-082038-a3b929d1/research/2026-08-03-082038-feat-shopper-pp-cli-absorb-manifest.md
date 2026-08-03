# Shopper CLI Absorb Manifest — Reprint 2026-08-03

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Product search by query | Prior CLI (catalog search) | `shopper-pp-cli catalog search <query>` | Works offline with synced FTS, regex, --select |
| 2 | Browse by department/category | Prior CLI (catalog departments) | `shopper-pp-cli catalog departments` | Offline + --json composable |
| 3 | Add product to cart | Prior CLI (cart add) | `shopper-pp-cli cart add --id <n> --quantity <n>` | --dry-run preview, --json |
| 4 | Remove product from cart | Prior CLI (cart remove) | `shopper-pp-cli cart remove --id <n> --quantity <n>` | --dry-run preview, --json |
| 5 | View cart totals | Prior CLI (cart summary) | `shopper-pp-cli cart summary` | Cashback tier, min/max order, --json |
| 6 | List delivery addresses | Prior CLI (address) | `shopper-pp-cli address` | Shows per-address available stores, --json |
| 7 | View upcoming delivery date | Prior CLI (delivery summary) | `shopper-pp-cli delivery summary` | Shows edit deadline context, --json |
| 8 | Delivery reschedule calendar | Prior CLI (delivery calendar) | `shopper-pp-cli delivery calendar` | Shows allowed range + disabled days |
| 9 | Purchase history | Patch: orders-history | `shopper-pp-cli orders history --store <s>` | All 6 stores, --json |
| 10 | Month-over-month spend | Patch: orders-history | `shopper-pp-cli orders spend` | All 6 stores side-by-side, --json |
| 11 | List available storefronts | Web UI only (store picker) | `shopper-pp-cli stores` | Live from API, shows IDs/clusters/payment params |
| 12 | Search suggestions | Prior CLI (catalog suggest) | `shopper-pp-cli catalog suggest <query>` | Live autocomplete, --json |
| 13 | Feature toggle status | Prior CLI (features toggle) | `shopper-pp-cli features toggle` | Hidden diagnostic, --json |
| 14 | Delivery reschedule (open browser) | Web UI | `shopper-pp-cli delivery reschedule --store <s>` | Deep-link opener with store-aware URL |
| 15 | Skip delivery (open browser) | Web UI | `shopper-pp-cli delivery skip --store <s>` | Deep-link opener, subscription stores |
| 16 | Suspend subscription (open browser) | Web UI | `shopper-pp-cli delivery suspend --store <s>` | Deep-link opener with safety note |
| 17 | Retrieve boleto (open browser) | Web UI | `shopper-pp-cli delivery boleto --store <s>` | Deep-link opener for bank slip |
| 18 | Subscription pause (open browser) | Web UI | `shopper-pp-cli subscription pause --store <s>` | Deep-link opener |
| 19 | Subscription resume (open browser) | Web UI | `shopper-pp-cli subscription resume --store <s>` | Deep-link opener |
| 20 | Payment cards status | Web UI | `shopper-pp-cli payment cards` | Card slot count + deep-link to manage |
| 21 | Checkout preview | Web UI checkout page | `shopper-pp-cli checkout preview` | Aggregates cart + delivery + payment params, siteapi read-only |
| 22 | Open checkout in browser | Web UI | `shopper-pp-cli checkout open` | Store-aware deep link opener |
| 23 | New product arrivals | Web UI | `shopper-pp-cli catalog news` | --json, per-store |
| 24 | Promotional banners | Web UI | `shopper-pp-cli catalog banners` | --json, per-store |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Charge & Edit Calendar | charge-calendar | hand-code | Requires combining /delivery/summary (deliveryDate) and /delivery/v2/calendar (reschedule config) and computing charge=−7d, lock=−5d. No single endpoint returns this view. | Use this command to see the full timeline for the next delivery cycle. Shows charge date, edit-lock deadline, and delivery date in one view. |
| 2 | Basket Diff | basket diff | hand-code | Requires snapshotting basket state to local SQLite and comparing against a previous snapshot. No API tracks this. | Compares current recurring basket against a previous cycle snapshot to show what changed before the edit window closes. |
| 3 | Price Watch | price-watch | hand-code | Tracks price history per SKU across synced catalog data. No API surface for historical price comparison. | Alerts when a product you buy rises or drops meaningfully versus your purchase baseline. |
| 4 | Restock Predictor | restock predict | hand-code | Requires buy-cadence analysis across order history in local SQLite. No API endpoint answers "when will you run out". | Predicts when you'll run out of staples from historical buying cadence and suggests what to add to the upcoming basket. |
| 5 | Catalog Drift Detector | catalog drift | hand-code | Requires comparing catalog snapshots across sync runs to detect discontinuation/substitution/price-per-unit changes. | Flags products you buy that were discontinued, silently swapped, or kept price while shrinking pack size. |
| 6 | Cashback Threshold Optimizer | cashback optimize | hand-code | Requires computing the cheapest items to cross the next cashback tier from local catalog + cart data. | Computes cheapest items to add (or whether to wait) to cross the next cashback tier, favoring items you'll need anyway. |
| 7 | Cross-Store Checkout Preview | checkout preview | hand-code | Aggregates cart/summary + delivery/summary + features/stores (payment params) into a unified pre-checkout view with charge-date computation. | Shows basket total, next delivery date, charge date, minimum order status, and accepted payment types before opening checkout in browser. |
| 8 | Store Capability Explorer | stores | spec-emits | Dynamic discovery from live GET /features/stores — no hardcoded list. Shows cluster IDs, payment method availability, recurrence type, ultra-fast flag. | Shows all 6 available storefronts with their API IDs and capability flags. |

## Approved Stubs
None — all transcendence features are buildable with available API surface and local SQLite.

## Prior Patches to Preserve
1. **store-scoping**: Global --store flag sets x-store-id/x-cluster-id headers. Cache key is store-aware. unica=cluster 3 (not 1). Now: add now(6/11) and now-bebidas(8/11).
2. **orders-history**: GET /orders/orders queries all storefronts (now 6). Empty stores excluded from table, present in --json.
3. **shopper-catalog-search-required-flags**: brands/metadata/types are optional filters (not required).
4. **shopper-charge-calendar-realshape**: /delivery/summary for deliveryDate; /delivery/v2/calendar for date-picker; charge=−7d, lock=−5d.
5. **catalog-go-1265-vuln-floor**: go.mod declares go 1.26.5+.
