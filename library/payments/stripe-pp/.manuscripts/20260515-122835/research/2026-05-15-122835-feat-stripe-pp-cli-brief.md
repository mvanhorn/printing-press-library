# Stripe CLI Brief

## API Identity
- **Domain:** Payments infrastructure — card processing, subscriptions, invoicing, marketplaces (Connect), embedded finance (Treasury, Issuing), tax, revenue reporting.
- **Users:** Developers (integration / dev-loop), Finance/RevOps analysts (reconciliation, MRR/churn), Founders/operators (revenue questions), Support engineers (charge triage, refunds).
- **Data profile:** ~500 endpoints, ~200 resources. High-gravity entities form a rich join graph: Customer ↔ PaymentIntent ↔ Charge ↔ BalanceTransaction ↔ Payout; Customer ↔ Subscription ↔ Invoice ↔ InvoiceItem; Charge ↔ Refund ↔ Dispute. Event API provides audit/delta stream.

## Reachability Risk
- **None.** Stripe is a first-class paid SaaS API. Test mode is free with identical surface (`sk_test_*` keys). No IP allowlists, no enterprise gating. Rate limits: 25 req/s test, 100 req/s live read; Search API ~20 req/s; ~1-minute search lag. 429 backoff and `Idempotency-Key` header on writes are the only operational gotchas.

## Top Workflows
1. **Failed-payment triage** — list PaymentIntents in `requires_payment_method`/`failed` over last 7d, join Customer + `last_payment_error`, export CSV for dunning.
2. **Refund + impact check** — issue refund, verify BalanceTransaction net impact on the next Payout (incl. fees).
3. **Subscription churn audit** — Subscriptions canceled in window, group by cancel reason, attach MRR delta + last invoice.
4. **Payout reconciliation** — pick a Payout, expand all contributing BalanceTransactions (charges, refunds, adjustments, fees) and tie to bank deposit.
5. **Customer 360** — given email or `cus_…`, dump charges, subscriptions, invoices, disputes, payment methods, lifetime value in one view.

## Table Stakes (must match stripe-cli + @stripe/mcp)
- Auth (`login`, env-var Bearer)
- Generic resource CRUD over the OpenAPI surface (customers, products, prices, subscriptions, invoices, payment_intents, charges, refunds, disputes, payouts, balance, events, coupons, payment_methods, payment_links)
- `--json` + `--select` + `--csv` output
- `--dry-run` for mutations + auto `Idempotency-Key`
- Pagination (cursor + `--all` + `--limit`)
- Doctor / health-check (auth + reachability + rate-limit headroom)
- Search API exposure

## Data Layer
- **Primary entities (SQLite + FTS5):** Customer, PaymentIntent, Charge, Invoice, Subscription, Refund, BalanceTransaction, Payout. Stretch: Dispute, Price, Product, Event.
- **Sync cursor:** `events` API (created cursor, type filter), plus per-entity `listSince` for resources with hard date filters. Maintain last-cursor per entity in a `sync_state` table.
- **FTS/search:** FTS5 over Customer (email, name, description, metadata), Charge/PaymentIntent (statement_descriptor, description, metadata), Invoice (number, description). Cross-entity FTS over a unified `searchable` virtual table.

## User Vision
Not provided — user picked "Vamos (recomendado)" at briefing.

## Product Thesis
- **Name:** `stripe-pp-cli`
- **Why it should exist:** `stripe-cli` is a dev-loop tool (webhook forward, log tail, sample apps). `@stripe/mcp` is a write-oriented online agent surface. Neither serves the analyst/operator asking *questions about their money*. This CLI is **agent-native + offline + compound-analytics-first**: local SQLite mirror, FTS5 search, cross-entity joins, `--json --select` everywhere, idempotency baked in, zero dev-loop overlap with stripe-cli.

## Build Priorities
1. **`stripe sync`** — incremental pull of Customers, PaymentIntents, Charges, Invoices, Subscriptions, Refunds, BalanceTransactions, Payouts into SQLite + FTS5 (events-cursor + per-entity since).
2. **`stripe customers find/get`** — filter by email, metadata, sub-state; `--with-charges --with-subscriptions` joins.
3. **`stripe payments list/get`** — `--failed`, `--by-customer`, `--since`, `--reason`; surfaces `last_payment_error`.
4. **`stripe subscriptions churn`** — canceled in window, group-by reason, MRR delta.
5. **`stripe payout explain`** — expand a payout into every contributing BalanceTransaction; reconcile to bank deposit total.
6. **`stripe refund create/list`** — idempotency-key auto, fee + net impact line.
7. **`stripe search`** — local FTS5 across mirror (customers, metadata, descriptions, statement descriptors).
8. **`stripe doctor`** — auth check, reachability, rate-limit headroom, test/live mode warning on `sk_live_`.

## Competitive Landscape (for absorb manifest)
- **stripe-cli** (official, Go): `login`, `listen`, `trigger`, `logs tail`, `events resend`, `samples`, `serve`, generic `get/post/delete`, generic resource CRUD. Online-first, no store, no compound queries, dev-loop bias.
- **@stripe/mcp** (official, npm + remote `mcp.stripe.com`): ~25-48 tools — `create_customer`, `list_customers`, `create/list_product`, `create/list_price`, `create/list_coupon`, `create/list_invoice`, `finalize_invoice`, `create/list/update/cancel_subscription`, `list_payment_intents`, `create_payment_link`, `create_refund`, `list/update_dispute`, `retrieve_balance`, `get_stripe_account_info`, `search_stripe_resources`, `search_stripe_documentation`.
- **stripe-python / stripe-node / stripe-go SDKs**: reference SDKs — every resource has `.list / .retrieve / .create / .update / .delete` plus search where applicable. Used as feature reference.
- **dj-stripe** (Django ORM mirror, Postgres-first): proves the local-mirror pattern; not a CLI.
- **Community Claude skills** (wrsmith108/stripe-mcp-skill, fcakyon stripe plugin, IncomeStreamSurfer stripe skills, alirezarezvani stripe-integration-expert, claude.com/plugins/stripe): integration playbooks wrapping the official MCP — not data-query CLIs.

## Pain Points (drive transcendence features)
1. Rate limits + 429 churn during list+expand campaigns.
2. `expand[]` is bounded (4 levels) — users hand-roll N+1 joins.
3. Idempotency keys easy to forget → double charges.
4. Test clocks rate-limit on bulk-sub advancement.
5. Search API eventual consistency (~1 min lag), 10k cap, separate DSL — cross-entity joins (e.g., "refunds for customers in plan X") must be client-side.

## Sources
- https://github.com/stripe/stripe-cli
- https://github.com/stripe/agent-toolkit (`@stripe/mcp`)
- https://docs.stripe.com/mcp
- https://docs.stripe.com/rate-limits
- https://docs.stripe.com/api/idempotent_requests
- https://docs.stripe.com/billing/testing/test-clocks/api-advanced-usage
- https://pypi.org/project/dj-stripe/
