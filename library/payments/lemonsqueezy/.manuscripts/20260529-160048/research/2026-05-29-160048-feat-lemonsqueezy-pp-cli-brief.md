# Lemon Squeezy CLI Brief

## API Identity
- **Domain**: SaaS billing and digital-commerce platform (subscriptions, license keys, one-off products, files, checkouts, discounts, webhooks, affiliates)
- **Users**: indie SaaS founders, makers selling digital downloads, course/newsletter creators, SaaS companies that picked LS as a merchant-of-record alternative to Stripe (LS handles global tax, paddle-style)
- **Data profile**: JSON:API REST, ~20 first-class resources, predictable list/get + scoped mutations on subscriptions/discounts/webhooks/checkouts/usage-records, denormalized 30-day and lifetime revenue/sales fields on the store object (worth local-mirroring)

## Reachability Risk
- **None** — `/v1/users/me` returns 200 with the supplied Bearer key; `/v1/stores` returns proper JSON:API with pagination + denormalized revenue fields; subscriptions/health endpoints accessible
- No known 403/bot-protection on the public REST surface
- Rate limit: 300 req/min per docs; standard Bearer auth, no clearance cookies

## Top Workflows

1. **Catalog setup before launch** — create store(s), products, variants with price tiers, attach files (e.g. desktop download builds), bulk-author discount codes for sale moments. Currently a tedious dashboard flow.
2. **Recurring revenue + churn watch** — list subscriptions, filter by status (active/past_due/cancelled/expired), pull MRR ramp from `subscription-invoices`, compute churn windows, catch dunning failures before they cascade.
3. **License key automation** — issue/list/deactivate/disable license keys for product variants, query `license-key-instances` to see how many seats per key, bulk re-issue or invalidate after a refund.
4. **Webhook ops** — register, rotate, prune, and replay-debug webhooks across stores; the LS webhook flow is the lifeline between LS and an app like PartnerUp.
5. **Discount + sale orchestration** — pre-bake a discount campaign (e.g., Founding-Member 50/2-year, 50/1-year, 50/lifetime), watch `discount-redemptions` in near-real-time, cap or expire discounts when limits hit.

## Table Stakes (every Lemon Squeezy tool must have)
- List/get every resource: stores, products, variants, prices, files, orders, order-items, affiliates, subscriptions, subscription-items, subscription-invoices, usage-records, discounts, discount-redemptions, license-keys, license-key-instances, checkouts, webhooks, users/me
- JSON:API filter[*] support (e.g., `filter[store_id]`, `filter[status]`, `filter[email]`)
- `include=` for related resources (store with products, order with items, subscription with customer)
- Pagination: `page[number]` + `page[size]` (max 100)
- Mutations: subscription PATCH (pause/resume/change variant), subscription DELETE (cancel), discount POST/DELETE, checkout POST (create custom checkout link), webhook POST/PATCH/DELETE, usage-record POST (metered billing)
- Auth: Bearer token via `LEMONSQUEEZY_API_KEY`
- Doctor / health / `users/me` for credential verification

## Data Layer
- **Primary entities** (sync-worthy): stores, products, variants, prices, customers, orders, order-items, subscriptions, subscription-items, subscription-invoices, discounts, discount-redemptions, license-keys, license-key-instances, webhooks, affiliates
- **Sync cursor**: `updated_at` per resource; LS supports `filter[updated_at][gte]=...` indirectly via JSON:API conventions
- **Sub-resource**: `subscription-items/{id}/current-usage` (metered-billing snapshot)
- **FTS targets**: customer email/name, product name/description, order receipt URL, license key, discount code, webhook URL
- **Denormalized analytics on stores**: `thirty_day_revenue`, `thirty_day_sales`, `total_revenue`, `total_sales` (lifetime + 30-day rolling, free of charge)
- **Denormalized on orders**: status, refunded flag, total/subtotal/discount/tax in `*_usd` and store-currency variants — pre-rolled metrics in every row

## Codebase Intelligence
- **Official SDK**: lmsqueezy/lemonsqueezy.js v4.x — TypeScript, tree-shakeable, function-per-endpoint shape (`getAuthenticatedUser`, `listSubscriptions({filter, include, page})`, `createCheckout`, etc.). 20 resource folders mirror the API exactly: checkouts, customers, discountRedemptions, discounts, files, license, licenseKeyInstances, licenseKeys, orderItems, orders, prices, products, stores, subscriptionInvoices, subscriptionItems, subscriptions, usageRecords, users, variants, webhooks
- **Auth**: HTTP Bearer; env var convention `LEMONSQUEEZY_API_KEY`; key created at https://app.lemonsqueezy.com/settings/api
- **Rate limiting**: 300 req/min per IP; 429 with `Retry-After` header
- **Data model**: JSON:API envelope (`{data, included, links, meta}`); relationships under `data.relationships`; pagination meta at `meta.page.{currentPage, lastPage, perPage, total}`; links at top-level `{first, last, next, prev}`
- **Architecture insight**: store object carries denormalized revenue/sales counts (30-day + lifetime), so an analytics dashboard does not need to walk every order — sync stores once, you get a baseline; sync orders for drill-down

## Build Priorities
1. **Generate full CRUD scaffolding** for all 19 resources from the OpenAPI (list/get + the documented POST/PATCH/DELETE on subscriptions/discounts/checkouts/webhooks/usage-records)
2. **Local SQLite mirror** of all primary entities, with FTS5 indexes on customers, products, orders, license-keys, discounts
3. **Sync command** with per-resource and `--full` modes; respect `updated_at` cursor where available
4. **Transcendence**: revenue snapshot, churn-watch, license-key roll-up, webhook replay buddy, discount-campaign monitor, sub state diff/drift, dunning-failure detector, Founding-Member-tier orchestrator

## Product Thesis
- **Name**: `lemonsqueezy-pp-cli`
- **Display name**: Lemon Squeezy
- **Why it should exist**: The official SDK is a TypeScript library — not a CLI. Every existing community CLI/MCP is a thin wrapper around a single subset (license keys, or subscriptions, or storefront ops). Nothing ships a local SQLite mirror, cross-entity SQL, offline FTS, churn-watch, or Founding-Member tier campaign orchestration. Indie SaaS founders need a single agent-native binary that turns the dashboard into one pipeable surface and surfaces revenue/churn/license signals without the dashboard click-walk.
