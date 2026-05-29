# Lemon Squeezy CLI Absorb Manifest

## Source Tools Inventory

| Tool | Type | Resources covered |
|---|---|---|
| lmsqueezy/lemonsqueezy.js | Official JS SDK v4.x | all 19 resources |
| NdoleStudio/lemonsqueezy-go | Community Go SDK | stores, customers, products, variants, files, orders, order-items, subscriptions, subscription-items, subscription-invoices, usage-records, discounts, discount-redemptions, license-keys, license-key-instances, checkouts, webhooks |
| seisigmasrl/lemonsqueezy.php | Community PHP SDK | core CRUD subset |
| codewriterbv/lemonsqueezy-java | Community Java SDK | core CRUD subset |
| hyper-designed/lemon_squeezy | Dart/Flutter client | core CRUD subset |
| YawLabs/lemonsqueezy-mcp | MCP server | store, products, customers, subscriptions, discounts, license-keys |
| atharvagupta2003/mcp-lemonsqueezy | MCP server (Python) | subscriptions, checkouts, products (with audit logging) |
| IntrepidServicesLLC/lemonsqueezy-mcp-server | MCP server | payments, subscriptions, customers (+ Salesforce sync) |
| adrianwedd/lemonsqueezy-claude-skills | 6 Claude skills | customer support, sales analytics, discount codes, refunds |
| abakermi/lemonsqueezy-admin | OpenClaw CLI skill | orders, subscriptions, customers |
| wdonofrio/lemonsqueezy-py-api | Python SDK | license keys, checkouts, webhooks, usage records, discounts |
| Popinek/lemonsqueezyLicense | Python tool | license-key issuance + validation |

## Absorbed (match or beat everything that exists)

### Doctor / Auth / Identity
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Verify API key + show authenticated user | lemonsqueezy.js getAuthenticatedUser | (generated endpoint) users me | Pipe-friendly JSON; doctor command also reports rate-limit status |
| 2 | Health probe | spec `/v1/health` | (generated endpoint) health check | Lightweight uptime check for CI |

### Stores
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 3 | List all stores | every SDK | (generated endpoint) stores list | Local SQLite mirror + denormalized revenue snapshots survive offline |
| 4 | Retrieve a store | every SDK | (generated endpoint) stores get | Cached locally; supports `--data-source local` |

### Products / Variants / Prices / Files
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 5 | List products | lemonsqueezy.js, Go SDK | (generated endpoint) products list | FTS5 search by name/description |
| 6 | Retrieve product | lemonsqueezy.js, Go SDK | (generated endpoint) products get | Includes via `--include=variants,store` |
| 7 | List variants | lemonsqueezy.js, Go SDK | (generated endpoint) variants list | Local price comparison across variants |
| 8 | Retrieve variant | lemonsqueezy.js, Go SDK | (generated endpoint) variants get | |
| 9 | List prices | lemonsqueezy.js, Go SDK | (generated endpoint) prices list | Local cache for billing reports |
| 10 | Retrieve price | lemonsqueezy.js, Go SDK | (generated endpoint) prices get | |
| 11 | List files | lemonsqueezy.js, Go SDK | (generated endpoint) files list | |
| 12 | Retrieve file | lemonsqueezy.js, Go SDK | (generated endpoint) files get | Download URLs cached |

### Customers
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 13 | List customers | every SDK, YawLabs MCP, Intrepid MCP, abakermi CLI | (generated endpoint) customers list | Offline FTS5 on name/email; cross-table joins to orders/subscriptions |
| 14 | Retrieve customer | every SDK | (generated endpoint) customers get | |

### Orders / Order Items
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 15 | List orders | every SDK, abakermi CLI | (generated endpoint) orders list | Local mirror with refund-status + receipt-URL FTS |
| 16 | Retrieve order | every SDK | (generated endpoint) orders get | |
| 17 | List order items | lemonsqueezy.js, Go SDK | (generated endpoint) order_items list | |
| 18 | Retrieve order item | lemonsqueezy.js, Go SDK | (generated endpoint) order_items get | |

### Affiliates
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 19 | List affiliates | api-evangelist spec | (generated endpoint) affiliates list | First CLI to surface affiliates (most SDKs skip this) |
| 20 | Retrieve affiliate | api-evangelist spec | (generated endpoint) affiliates get | |

### Subscriptions
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 21 | List subscriptions | every SDK, every MCP, abakermi CLI | (generated endpoint) subscriptions list | Status filtering + local churn analytics |
| 22 | Retrieve subscription | every SDK, every MCP | (generated endpoint) subscriptions get | |
| 23 | Update subscription (pause/resume/change variant) | lemonsqueezy.js, Go SDK, Yawlabs MCP | (generated endpoint) subscriptions update | `--dry-run` + JSON:API attribute editing |
| 24 | Cancel subscription | lemonsqueezy.js, Go SDK, Yawlabs MCP | (generated endpoint) subscriptions delete | Confirms before destructive call |

### Subscription Items / Invoices / Usage
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 25 | List subscription items | lemonsqueezy.js, Go SDK | (generated endpoint) subscription_items list | |
| 26 | Retrieve subscription item | lemonsqueezy.js, Go SDK | (generated endpoint) subscription_items get | |
| 27 | Update subscription item (metered quantity) | lemonsqueezy.js, Go SDK | (generated endpoint) subscription_items update | |
| 28 | Retrieve current usage for sub item | lemonsqueezy.js, Go SDK | (generated endpoint) subscription_items current_usage | |
| 29 | List subscription invoices | lemonsqueezy.js, Go SDK | (generated endpoint) subscription_invoices list | Local MRR/ARR rollup |
| 30 | Retrieve subscription invoice | lemonsqueezy.js, Go SDK | (generated endpoint) subscription_invoices get | |
| 31 | Create usage record (metered) | lemonsqueezy.js, Go SDK | (generated endpoint) usage_records create | `--dry-run`; idempotency via `Idempotency-Key` |
| 32 | List usage records | lemonsqueezy.js, Go SDK | (generated endpoint) usage_records list | |
| 33 | Retrieve usage record | lemonsqueezy.js, Go SDK | (generated endpoint) usage_records get | |

### Discounts / Redemptions
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 34 | List discounts | every SDK, YawLabs MCP, adrianwedd discount-codes skill | (generated endpoint) discounts list | Local FTS5 on code; filter by status/store |
| 35 | Retrieve discount | every SDK | (generated endpoint) discounts get | |
| 36 | Create discount | lemonsqueezy.js, Go SDK, adrianwedd discount-codes skill | (generated endpoint) discounts create | `--dry-run`; supports stdin JSON |
| 37 | Delete discount | lemonsqueezy.js, Go SDK | (generated endpoint) discounts delete | Confirms before destructive call |
| 38 | List discount redemptions | lemonsqueezy.js | (generated endpoint) discount_redemptions list | Live monitor for sale moments |
| 39 | Retrieve discount redemption | lemonsqueezy.js | (generated endpoint) discount_redemptions get | |

### License Keys
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 40 | List license keys | every SDK, YawLabs MCP, Popinek tool | (generated endpoint) license_keys list | Local FTS5 on key, customer email; filter by status |
| 41 | Retrieve license key | every SDK, Popinek | (generated endpoint) license_keys get | |
| 42 | List license-key instances | lemonsqueezy.js, Go SDK | (generated endpoint) license_key_instances list | Per-seat audit trail |
| 43 | Retrieve license-key instance | lemonsqueezy.js, Go SDK | (generated endpoint) license_key_instances get | |

### Checkouts
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 44 | List checkouts | lemonsqueezy.js | (generated endpoint) checkouts list | |
| 45 | Retrieve checkout | lemonsqueezy.js | (generated endpoint) checkouts get | |
| 46 | Create custom checkout link | lemonsqueezy.js, Go SDK, every MCP | (generated endpoint) checkouts create | `--dry-run`; emits ready-to-share URL |

### Webhooks
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 47 | List webhooks | lemonsqueezy.js | (generated endpoint) webhooks list | |
| 48 | Retrieve webhook | lemonsqueezy.js | (generated endpoint) webhooks get | |
| 49 | Create webhook | lemonsqueezy.js | (generated endpoint) webhooks create | |
| 50 | Update webhook | lemonsqueezy.js | (generated endpoint) webhooks update | |
| 51 | Delete webhook | lemonsqueezy.js | (generated endpoint) webhooks delete | |

### Cross-cutting (framework)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 52 | Local sync of all resources | none (no competitor has this) | (behavior in lemonsqueezy-pp-cli sync) | Per-resource + `--full`; respects `updated_at` cursor |
| 53 | Offline FTS5 search | none | (behavior in lemonsqueezy-pp-cli search) | Cross-resource ranked search |
| 54 | Cross-entity SQL | none | (behavior in lemonsqueezy-pp-cli sql) | SELECT-only; arbitrary joins across resources |
| 55 | Doctor + auth verification | none | (behavior in lemonsqueezy-pp-cli doctor) | Auth probe + cache report + rate-limit status |
| 56 | MCP server surface | every MCP | (behavior via cobratree mirror) | Beats all 3 MCPs by mirroring the full CLI tree with read-only annotations |

## Transcendence

Pending novel-features subagent.

### Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| T1 | Revenue snapshot | revenue-snapshot | hand-code | Joins denormalized 30-day/lifetime store counters with local orders for refund-adjusted net — no single LS endpoint returns this | Use this for a point-in-time revenue/sales rollup including one-off orders. For weekly time-series of MRR with new/churn/expansion split, use 'mrr-trend' instead. |
| T2 | MRR trend | mrr-trend --weeks 12 | hand-code | Time-series MRR with new/churn/expansion/contraction classification requires cross-table SQL on subscriptions + subscription-invoices | Use this for time-series MRR. For a single point-in-time revenue rollup that includes one-off orders, use 'revenue-snapshot' instead. |
| T3 | Churn watch | churn-watch --since 7d | hand-code | Diff against prior-sync snapshot table — pure local SQLite operation, impossible without our mirror | Use this for subscription status transitions in a window. For invoice-level failed charges where the subscription is still recoverable, use 'dunning-alert' instead. |
| T4 | Dunning alert | dunning-alert | hand-code | LS-specific join: invoices.status=failed AND subscription.status IN active/past_due — requires both tables locally | Use this for recoverable failed-charge windows. For status-change events on the subscription itself, use 'churn-watch' instead. |
| T5 | License-key roll-up | license-rollup | hand-code | Triple-table join (keys × instances × variants) — no LS endpoint returns per-variant or per-key activation aggregates | Use this for seat/usage distribution across keys. To act on one refunded order (disable keys, audit instances), use 'refund-cascade' instead. |
| T6 | Refund cascade | refund-cascade <order-id> | hand-code | LS-specific 4-endpoint orchestration (order → items → keys → instances → disable) with `--apply` mutation gating | Use this for the post-refund disable cascade on a specific order. For routine "find keys with abnormal seat counts" sweeps, use 'license-rollup' instead. |
| T7 | Campaign watch | campaign-watch [discount-code...] | hand-code | Live capacity + redemption velocity + sellout projection from local discount-redemptions — no LS endpoint computes pace | Use this for live capacity + pace tracking during a sale. For broad discount inventory regardless of activity, use the generated 'list-discounts' instead. |
| T8 | Webhook audit | webhook-audit | hand-code | Cross-store group-by-URL with stale-host heuristic (localhost/ngrok/*.test/*.local detection) — LS dashboard is per-store only | Use this for cross-store webhook coverage + stale-host detection. For pruning the dead ones, pipe through the generated 'delete-webhook'. |
