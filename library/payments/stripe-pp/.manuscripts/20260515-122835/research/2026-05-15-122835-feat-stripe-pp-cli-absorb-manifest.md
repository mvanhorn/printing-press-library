# Stripe CLI — Absorb Manifest

## Customer Model

The Stripe power user is a **finance/RevOps analyst, founder, or support engineer** running a SaaS or marketplace on Stripe. Their daily ritual is answering money questions about their own business — "who churned?", "is this payout right?", "why did this charge fail?", "what's $X customer's LTV?". They live in agents (Claude/Cursor) and pipe JSON to `jq`. They need **complete, joined, deterministic answers in one command** — not a 1-minute wait on Stripe Search API or a 5-click dashboard journey.

## Absorbed (match or beat what exists)

### From `stripe-cli` (official, Go — github.com/stripe/stripe-cli)

| # | Feature | Our Implementation | Added Value |
|---|---------|--------------------|-------------|
| 1 | `login` / token management | `auth set-token` + `auth status` (generator emits) | Env var `STRIPE_SECRET_KEY` precedence; warns loudly on `sk_live_*` |
| 2 | `resources get/post/delete` | Per-resource typed CRUD across all 50 generator-truncated resources | Typed flags, `--json`, `--select`, `--csv`, `--dry-run` everywhere |
| 3 | `events list/resend` | Absorbed via generator `events list/get` | Local mirror means events also query against synced history |
| 4 | `samples` | NOT absorbed (dev-loop scope, out of v1) | — |
| 5 | `listen` (webhook forwarding) | NOT absorbed (dev-loop scope, complementary to stripe-cli) | — |
| 6 | `trigger` | NOT absorbed (dev-loop scope) | — |
| 7 | `logs tail` | NOT absorbed (out of v1) | — |
| 8 | `serve` | NOT absorbed (dev-loop scope) | — |

### From `@stripe/mcp` (official MCP server — github.com/stripe/agent-toolkit)

| # | Feature | Our Implementation | Added Value |
|---|---------|--------------------|-------------|
| 9 | `list_customers` / `create_customer` / `retrieve_customer` | Generator `customers list/get/create` + `customers profile` novel | Local SQLite means list is instant and supports cross-entity filters |
| 10 | `list_products` / `create_product` / `update_product` | Generator `products list/get/create/update` | `--json --select` ergonomics |
| 11 | `list_prices` / `create_price` | Generator `prices list/get/create` | Same |
| 12 | `list_coupons` / `create_coupon` | Generator `coupons list/get/create` | Same |
| 13 | `list_invoices` / `create_invoice` / `finalize_invoice` | Generator `invoices list/get/create/finalize` | Local mirror enables aging + open-balance queries |
| 14 | `create_subscription` / `list_subscriptions` / `update_subscription` / `cancel_subscription` | Generator `subscriptions list/get/create/update/cancel` | Plus novel `subscriptions churn` |
| 15 | `list_payment_intents` | Generator `payment_intents list/get` | Plus novel `payments failed` triage |
| 16 | `create_payment_link` | Generator `payment_links list/get/create` | — |
| 17 | `create_refund` / `list_disputes` / `update_dispute` | Generator `refunds list/get/create`, `disputes list/get/update` | Local mirror enables refund-impact attribution |
| 18 | `retrieve_balance` | Generator `balance retrieve` | — |
| 19 | `get_stripe_account_info` | Generator `accounts retrieve` + `doctor` extension | — |
| 20 | `search_stripe_resources` | Novel `search "<query>"` over local FTS5 mirror | **Beats it:** no 1-min lag, no 10k cap, joinable to full rows |
| 21 | `search_stripe_documentation` | NOT absorbed (Stripe-docs scope) | — |

### From `stripe-go/python/node` SDKs

| # | Feature | Our Implementation | Added Value |
|---|---------|--------------------|-------------|
| 22 | `.list / .retrieve / .create / .update / .delete` per resource | Generator emits the typed surface | Same surface, agent-native I/O |
| 23 | `expand[]` parameter | Passed through via `--expand` flag where generator surfaces it | Limited to 4 levels per Stripe; novel commands replace `expand` with local joins |
| 24 | Idempotency keys | Generator auto-injects `Idempotency-Key` on POST | Stored in SQLite; replay-safe |
| 25 | Cursor pagination + `auto-paginating-iterators` | Generator `--all` / `--limit` flags | Same |
| 26 | Search API (`/v1/*/search`) | Generator exposes search endpoints; novel `search` replaces with local FTS5 | Beats lag + 10k cap |

### From `dj-stripe` (Django ORM mirror — pypi `dj-stripe`)

| # | Feature | Our Implementation | Added Value |
|---|---------|--------------------|-------------|
| 27 | Postgres-mirror pattern | Our `sync` writes to SQLite (single-binary, no Postgres dep) | Single binary, agent-portable |
| 28 | Webhook-driven sync | We use Events API pull (no webhook server needed) | No public ingress required |

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score | Status |
|---|---------|---------|-------------------------|-------|--------|
| T1 | **Mirror sync** | `sync --since <ts> --entities <list>` | Foundational: incremental backfill into SQLite via Events API + list endpoints with resumable cursors. No incumbent has a local store. | 10 | shipping |
| T2 | **Payout explain** | `payouts explain <po_id>` | Decomposes a payout into balance_transactions (charges/refunds/fees/adjustments) with per-customer attribution via local SQL JOIN. Dashboard requires CSV export → spreadsheet. | 10 | shipping |
| T3 | **Failed payment triage** | `payments failed --since <date> --group-by decline_code` | Groups failed PaymentIntents by `decline_code` with counts + $ at risk + sample customers. Stripe has no group-by-decline view; Search API can't aggregate. | 9 | shipping |
| T4 | **Subscription churn audit** | `subscriptions churn --since <date> --group-by cancel_reason --mrr-delta` | Joins canceled subs to invoice MRR, groups by `cancellation_details.reason`, sums delta. Sigma costs extra; Dashboard has no aggregated $ churn report. | 9 | shipping |
| T5 | **Customer 360** | `customers profile <cus_id_or_email>` | Joins customer + subs + lifetime charges/refunds + open invoices + LTV in one query. Dashboard splits across 5 tabs. | 9 | shipping |
| T6 | **Why did this charge fail** | `charges why <ch_id>` | Returns decline_code + network message + customer recent activity + similar 7d failures. 5+ Dashboard clicks today. | 9 | shipping |
| T7 | **FTS search across entities** | `search "<query>" --entities customers,invoices,charges` | FTS5 over local mirror (emails, descriptions, metadata, statement_descriptors). Beats Search API lag + 10k cap. | 9 | absorbed-via-generator (the generator emits `search` framework-wide; we extend with `--entities` filter) |
| T8 | **Customer timeline** | `customers timeline <cus_id>` | Merge-sorted event stream across charges/refunds/invoices/subs/disputes for one customer. Dashboard splits across 5 tabs. | 8 | deferred-to-v1.1 (cut for v1 time budget; the data is in the store, the command is straightforward to add later) |

**Shipping novel commands for v1:** T1, T2, T3, T4, T5, T6. T7 ships via the generator framework's universal `search`. T8 deferred to v1.1.

## Killed candidates (audit trail)

| Name | Score | Reason |
|---|---|---|
| Customer LTV ranking | 7 | Covered inside Customer 360's `lifetime_value` field. |
| Idempotency replay safety | 7 | Engineering safety, not analyst's weekly question. |
| Cohort retention | 7 | Monthly cadence, not weekly. |
| Disputes early-warning | 6 | Lower frequency; partially covered by `charges why`. |
| Invoice aging | 7 | B2B/AR specific; defer until that user appears. |
| Payout reconciliation diff | 8 | Strong but requires external CSV input; `payouts explain` covers 90%. |
| Duplicate charge detector | 8 | Surgical tool, lower freq than failed-triage. |
| MRR walk | 8 | Overlap with churn `--mrr-delta`. |
| Subscriptions about to renew | 8 | Forward-looking; persona is reactive in v1. |
| Stale customer cleanup | 6 | Hygiene, not money. |
| Coupon/promo effectiveness | 6 | Niche audience. |
| Fee analysis | 7 | Subsumed by `payouts explain` breakdown. |
| Revenue by product/price | 7 | Requires Product/Price in mirror; defer. |
| Webhook diff vs mirror | 6 | Engineering audit, not analyst. |
| Refund eligibility check | 6 | Single API call; no compound value. |
| Refund impact check | 8 | Subsumed by Customer 360 + payouts explain. |
