# WooCommerce CLI — Absorb Manifest


Competing tools surveyed (verified, with star counts and recency):

| Tool | Kind | Surface | Stars | Last push |
|---|---|---|---|---|
| `AmitGurbani/mcp-server-woocommerce` | MCP | **101 tools** — the most complete CRUD surface found | 1 | 2026-05-05 |
| `woocommerce/woocommerce-claude` | Official Claude plugin | **18 analytics/ops skills** + 11 custom tools + 4 `wc-analytics-*` | 24 | 2026-05-22 |
| `techspawn/woocommerce-mcp-server` | MCP | CRUD: products/orders/customers/shipping/taxes/discounts | 96 | 2025-11-10 |
| `Yuri-Lima/woocommerce-rest-ts-lib` | SDK + MCP | 60+ tools, prompts: store-audit / order-report / inventory-check | 43 | 2026-07-08 |
| `iOSDevSK/mcp-for-woocommerce` | MCP | ~24 read tools incl. `wc_intelligent_search`, filtered product queries | 15 | 2026-04-22 |
| `tropk-ai/mcp-for-wordpress` | MCP | 500+ WP tools incl. WooCommerce slice | 27 | 2026-06-27 |
| WooCommerce core native MCP (WC 10.3+) | Official, dev preview | product query/create/update/delete; order query/update-status/add-notes | — | preview |
| `wppoland/woocommerce-mcp` | MCP | 5 read tools: list/get products, list orders, sales_report | 2 | 2026-07-19 |
| Official SDKs (`@woocommerce/woocommerce-rest-api`, `wc-api-python`, `automattic/woocommerce` PHP, Ruby) | SDK | generic get/post/put/delete + OAuth1.0a signing | — | maintained |
| WP-CLI `wp wc` command family | CLI | in-core CRUD for products/orders/customers/tools | — | in core |

**Competitive read:** the field is fragmented — dozens of single-author MCP servers, none dominant. Every one of them is a **stateless live-API proxy**. Not one persists data locally, and not one uses the public Store API. Feature completeness ≠ adoption (the 101-tool server has 1 star). Automattic's own plugin is the real quality bar, and it competes on *analytics workflows*, not CRUD count.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/filter orders | AmitGurbani, techspawn, core MCP | `(generated endpoint) orders list` | Full live filter set (24 args) + `--json`/`--select`/`--csv`, offline mirror, typed exit codes |
| 2 | Get / create / update / delete order | AmitGurbani, core MCP | `(generated endpoint) orders get\|create\|update\|delete` | `--dry-run` on every mutation, idempotent, agent-native |
| 3 | Batch order ops | AmitGurbani | `(generated endpoint) orders batch` | Batch with dry-run preview |
| 4 | Order status list | live route index (undocumented) | `(generated endpoint) orders statuses` | Not in the Slate docs; no competitor exposes it |
| 5 | Order notes CRUD | AmitGurbani, core MCP | `(generated endpoint) order-notes list\|get\|create\|delete` | — |
| 6 | Order refunds CRUD | AmitGurbani | `(generated endpoint) order-refunds list\|get\|create\|delete` | — |
| 7 | Order actions (resend email, send details, templates) | official docs only | `(generated endpoint) orders send-email\|send-order-details\|email-templates` | No MCP server exposes these |
| 8 | List/filter products | every tool | `(generated endpoint) products list` | Full arg set + local FTS search |
| 9 | Product CRUD + batch + duplicate | AmitGurbani, techspawn | `(generated endpoint) products get\|create\|update\|delete\|batch\|duplicate` | `--dry-run`, batch preview |
| 10 | Related / suggested products | live route index | `(generated endpoint) products related\|suggested-products` | Undocumented; no competitor has it |
| 11 | Product custom-field names | official docs | `(generated endpoint) products custom-field-names` | Missing from wtx-labs spec and all MCP servers |
| 12 | Variations CRUD + batch + generate | AmitGurbani | `(generated endpoint) variations list\|get\|create\|update\|delete\|batch` | — |
| 13 | Global variations list | live route index | `(generated endpoint) variations all` | Undocumented cross-product variation listing |
| 14 | Product attributes + terms CRUD | AmitGurbani, iOSDevSK | `(generated endpoint) product-attributes ...` / `attribute-terms ...` | — |
| 15 | Categories CRUD | all | `(generated endpoint) categories list\|get\|create\|update\|delete\|batch` | — |
| 16 | Tags CRUD | all | `(generated endpoint) tags ...` | — |
| 17 | Brands CRUD | AmitGurbani | `(generated endpoint) brands ...` | — |
| 18 | Reviews CRUD | AmitGurbani, iOSDevSK | `(generated endpoint) reviews ...` | — |
| 19 | Shipping classes CRUD | AmitGurbani | `(generated endpoint) shipping-classes ...` | — |
| 20 | Customers CRUD + batch + downloads | AmitGurbani, techspawn | `(generated endpoint) customers list\|get\|create\|update\|delete\|batch\|downloads` | — |
| 21 | Coupons CRUD + batch | AmitGurbani, techspawn | `(generated endpoint) coupons ...` | — |
| 22 | Tax rates + tax classes CRUD | AmitGurbani, iOSDevSK | `(generated endpoint) taxes ...` / `tax-classes ...` | — |
| 23 | Shipping zones / locations / methods CRUD | AmitGurbani, iOSDevSK | `(generated endpoint) shipping-zones ...`, `shipping-zone-locations ...`, `shipping-zone-methods ...` | — |
| 24 | Shipping methods list | iOSDevSK | `(generated endpoint) shipping-methods list\|get` | — |
| 25 | Payment gateways list/get/update | AmitGurbani, iOSDevSK | `(generated endpoint) payment-gateways ...` | — |
| 26 | Webhooks CRUD + batch | AmitGurbani | `(generated endpoint) webhooks ...` | — |
| 27 | Settings groups + options read/update | AmitGurbani | `(generated endpoint) settings ...` | — |
| 28 | System status + tools | AmitGurbani, iOSDevSK | `(generated endpoint) system-status status\|tools\|run-tool` | — |
| 29 | Sales report | wppoland, Yuri-Lima, all | `(generated endpoint) reports sales` | — |
| 30 | Top sellers report | AmitGurbani | `(generated endpoint) reports top-sellers` | — |
| 31 | Totals reports (orders/customers/products/coupons/reviews) | AmitGurbani | `(generated endpoint) reports orders-totals\|customers-totals\|...` | — |
| 32 | Store data: continents / countries / currencies | official docs (absent from wtx-labs spec) | `(generated endpoint) store-data ...` | Missing from every competing spec and MCP server |
| 33 | Refunds (store-wide) | official docs | `(generated endpoint) refunds list` | — |
| 34 | Intelligent / filtered product search | iOSDevSK `wc_intelligent_search` | `(behavior in woocommerce-pp-cli search) ...` | Local FTS5 over synced catalog — works offline, no API round-trip, regex + SQL-composable |
| 35 | Products by brand / category / attribute | iOSDevSK | `(behavior in woocommerce-pp-cli products list) ...` | Native filter args plus local SQL for combinations the API can't express |
| 36 | Store audit / health | Yuri-Lima prompt, `woocommerce-claude` store-health-monitor, respira-press | `woocommerce-pp-cli doctor` | Checks store URL, permalinks, key scope, Authorization-header stripping, route availability |
| 37 | Inventory check | Yuri-Lima prompt, `woocommerce-claude` inventory-risk-review | `(behavior in woocommerce-pp-cli products list) ...` | Native stock filters; velocity/forecast promoted to transcendence |
| 38 | Order report | Yuri-Lima prompt, `woocommerce-claude` weekly-store-review | `woocommerce-pp-cli analytics` | Local group-by across synced orders, no per-question API call |
| 39 | Catalog audit | `woocommerce-claude` catalog-audit | `(transcendence)` — see `catalog audit` | Automattic's is LLM-driven prose; ours is a deterministic local scan |
| 40 | Failed-order triage | `woocommerce-claude` failed-order-triage | `(transcendence)` — see `orders triage` | Ours adds gateway-failure-rate comparison over time from local history |
| 41 | Refund triage | `woocommerce-claude` refund-triage | `(transcendence)` — see `refund-rate` | Ours attributes refunds to products via local line-item join |
| 42 | Coupon performance triage | `woocommerce-claude` coupon-performance-triage | `(transcendence)` — see `coupon roi` | Ours computes realized revenue per coupon from local order join |
| 43 | Customer value review | `woocommerce-claude` customer-value-review | `(transcendence)` — see `customers ltv` | Ours computes true LTV + cohort retention locally |
| 44 | Payment / shipping / geography method review | `woocommerce-claude` (3 skills) | `woocommerce-pp-cli analytics --type orders --group-by <field>` | One generic local group-by replaces three bespoke reports |
| 45 | Tax reconciliation | `woocommerce-claude` tax-reconciliation | `woocommerce-pp-cli sql` + `analytics` | Arbitrary SQL over synced tax/order data |
| 46 | Revenue-drop triage | `woocommerce-claude` revenue-drop-triage | `(transcendence)` — see `revenue explain` | Needs historical snapshots; only possible with a local mirror |
| 47 | Generic authenticated request escape hatch | official SDKs (get/post/put/delete) | `(behavior in woocommerce-pp-cli sql)` + generated endpoint surface | Full typed surface means the escape hatch is rarely needed |
| 48 | OAuth 1.0a signing for HTTP stores | official SDKs | `(behavior in woocommerce-pp-cli doctor)` | Basic-over-HTTPS default; query-param fallback for header-stripping hosts |
| 49 | Webhook delivery testing | RequestBin/Hookbin (docs-recommended external tools) | `(generated endpoint) webhooks create\|update` | — |
| 50 | Public catalog read without credentials | `wpscrape` (1★, products/categories/site only) | `(generated endpoint) catalog list\|get\|collection-data`, `catalog-taxonomy ...`, `catalog-reviews list` | Full Store API surface (33 filter args) vs. wpscrape's 3 commands; plus persistence, history, and aggregates |
| 51 | Multi-store / multi-host profiles | `order-fetcher` (`config add/remove/view`) | `(behavior in woocommerce-pp-cli doctor)` + `WOOCOMMERCE_BASE_URL` / config `base_url` | Per-store config + env override; every command retargetable without reconfiguration |
| 52 | Order export to CSV with column shaping | `order-fetcher` (`--columns/--omit/--include/-o`) | `(behavior in woocommerce-pp-cli orders list)` via `--csv` + `--select` | Generic across every resource, not just orders |
| 53 | Order filtering by SKU / SKU prefix | `order-fetcher` (`--sku`, `--sku-prefix`) | `(behavior in woocommerce-pp-cli sql)` | The REST API cannot filter orders by line-item SKU at all; local join makes it possible |
| 54 | Bulk product create/update/remove from files | `woco` (`run`/`run update`/`run remove`) | `(generated endpoint) products batch` | Batch endpoint + `--dry-run` preview |
| 55 | Bulk price update from spreadsheet | `WooPriceUpdater` | `(generated endpoint) products batch` | — |
| 56 | Bulk order-status update from CSV | `Woo-Order-Status-Updater` | `(generated endpoint) orders batch` | — |
| 57 | Catalog snapshot / changes / history / compare | `shopextract` (multi-platform, 1★) | `(transcendence)` — see `catalog watch` / `catalog diff` | WooCommerce-native and far deeper: full Store API filter set, price+stock+sale history, any store, no credentials |
| 58 | System-status tool listing and execution | WP-CLI `wc tool list` / `wc tool run` | `(generated endpoint) system-status tools\|run-tool` | Works remotely over REST; WP-CLI requires shell access to the server |
| 59 | Data-pipeline style extraction (spec/check/discover/read) | `airbyte-source-woocommerce` | `woocommerce-pp-cli sync` + `sql` | Incremental sync into queryable SQLite instead of a one-way ELT dump |
| 60 | Store security / configuration audit | `woobean-cli` (**never implemented** — empty repo) | `woocommerce-pp-cli doctor` | An acknowledged gap nobody filled; doctor covers key scope, permalinks, header stripping, route availability |

**Explicitly NOT absorbed (out of scope, and why).** WP-CLI's `wc hpos *`, `wc palt *`, `wc blueprint *`, `wc com *`, `wc update`, and `wc tracker snapshot` are server-side commands that require shell access to the WordPress host. They have no REST equivalent and cannot be reached by any HTTP client. This CLI is REST-only and will say so rather than pretend otherwise.

## Transcendence (only possible with our approach)

Produced by the novel-features subagent: 3 passes (customer model → 16 candidates → adversarial cut), 8 killed, 8 survivors. All survivors score >= 8/10.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Order triage | `orders triage` | hand-code | Buckets synced orders by status + age from `date_modified`, and computes failed-order rate per `payment_method_title` against the trailing period. The rate-vs-history comparison needs stored local history no single API call returns. | none |
| 2 | Stock velocity & reorder list | `stock velocity --window 30d` | hand-code | Aggregates order `line_items` per product/variation over a window, then divides current `stock_quantity` by units-per-day for days-of-cover. No endpoint returns a demand denominator; `products list` gives stock with no velocity. | none |
| 3 | Revenue decomposition | `revenue explain --window 7d --against prior` | hand-code | Diffs two periods of synced orders into order-count, AOV, per-product, refund and coupon contribution deltas. `reports sales` returns one period's totals and cannot decompose a delta. | Use this command to explain a change in revenue between two time periods, broken into order-count, basket-size, product, refund, and coupon contributions. Do NOT use this command to slice a single period by one dimension; use 'analytics --type orders --group-by <field>' instead. Do NOT use it for product-level return quality; use 'refund-rate' instead. |
| 4 | Refund rate by product | `refund-rate --window 90d` | hand-code | Joins order-refunds to the parent order's `line_items` in SQLite for refunded-units over sold-units per product. Refunds and orders are separate endpoints and neither carries a per-product rate. | Use this command to find which products are refunded most often, as a rate against units sold. Do NOT use this command to explain a revenue change over time; use 'revenue explain' instead. |
| 5 | Customer LTV & cohorts | `customers ltv --cohort month` | hand-code | Groups synced orders by `customer_id` for LTV and order count, then buckets by first-order month for repeat-purchase retention. Structurally impossible in the Reports API. | none |
| 6 | Catalog completeness audit | `catalog audit` | hand-code | Deterministic rule scan over synced products + variations for missing image/SKU/description/category, zero price, unpriced or stockless variations. Automattic's equivalent is LLM prose; ours is a rule list with counts and IDs. | Use this command to find defects in the catalog of the store you own — missing images, empty descriptions, unpriced variations. Do NOT use this command to track another store's catalog over time; use 'catalog watch' instead. Do NOT use it to compare two catalog snapshots; use 'catalog diff' instead. |
| 7 | Public catalog snapshot | `catalog watch --store <url>` | hand-code | Pulls `wc/store/v1/products` from ANY store with zero credentials (verified live: 200, `x-wp-total: 1676`) and writes a timestamped price/sale/stock snapshot, normalizing `currency_minor_unit` on write. | Use this command to record a snapshot of any public WooCommerce store's catalog, including stores you have no credentials for. Do NOT use this command to see what changed between snapshots; it only records — use 'catalog diff' for the comparison. Do NOT use it to check your own store for missing images or unpriced variations; use 'catalog audit' instead. |
| 8 | Catalog diff | `catalog diff --store <url> --since 30d` | hand-code | Compares two locally stored snapshots for added/delisted SKUs, price moves with magnitude, sale starts and ends, and stock flips. Issues no API call at read time — pure local history. | Use this command to see what changed in a store's catalog between two recorded snapshots — new products, delistings, price moves, sale windows. Do NOT use this command to fetch fresh data; run 'catalog watch' first to record a snapshot. Do NOT use it to audit your own catalog for defects; use 'catalog audit' instead. |

**Data-source strategy:** `orders triage`, `stock velocity`, `revenue explain`, `refund-rate`, `customers ltv`, `catalog audit` → `local`. `catalog watch` → `live`. `catalog diff` → `computed`.

**Hand-code commitment:** all 8 transcendence rows are `hand-code` (0 `spec-emits`). Each is ~50–150 LoC plus `root.go` wiring.

**Stubs:** none. No row in this manifest ships as a stub.

### Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Coupon ROI (`coupon roi`) | Reviewed monthly at campaign close, not weekly; framework `analytics --type orders --group-by coupon_lines` already delivers realized revenue per code. | `revenue explain` |
| Store capability fingerprint (`store fingerprint`) | Fails weekly-use — you fingerprint a store once at onboarding — and needs a curated namespace→extension table that ages badly. | `doctor` |
| Cross-store price ladder (`catalog compare`) | Matching equivalent products across independent merchants is semantic matching (LLM-dependency kill); exact-SKU fallback almost never holds across stores. | `catalog diff` |
| Sale cadence predictor (`catalog sale-cadence`) | Every sale start/end is already a diff row; the only increment is a periodicity prediction a deterministic CLI should not assert. | `catalog diff` |
| Dual-namespace price integrity (`price integrity`) | Fires on a launch-window bug that is rare in steady state; the minor-unit normalization it tests belongs in `catalog watch`'s write path as verified behavior. | `catalog audit` |
| Fulfillment SLA (`orders sla`) | The REST API exposes no status-transition history, so the "entered processing" timestamp would have to be invented — reimplementation kill. | `orders triage` |
| Churn-risk customers (`customers at-risk`) | One derived column on the LTV command's own output; splits one cohort query across two commands. | `customers ltv` |
| Attribute coverage (`catalog attributes coverage`) | A single rule inside the catalog defect scan; fragments the same pass across two commands. | `catalog audit` |
