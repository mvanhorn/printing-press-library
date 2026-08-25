# Quentli CLI Brief

## API Identity
- Domain: Payments, invoicing, and subscription billing for LATAM (Mexico-centric cobranza/facturación platform, Stripe-like).
- Users: engineers and operations automating payment collection, CFDI tax invoicing, recurring subscriptions, and payment links.
- Data profile: transactional — customers, invoices, payments, subscriptions, catalog concepts, discounts, tax invoices (SAT CFDI), webhook events. Amounts are in minor currency units (e.g. 150000 = MXN 1,500.00).

## Reachability Risk
- Low. Official OpenAPI 3.0.3 spec at docs.quentli.com/openapi.yaml. Live probe of GET /v1/customers returns 401 `{{"error":{"message":"You are not authenticated"}}}` without a key — expected for an auth-required API (no key provided).
- Auth: Organization API key, HTTP Bearer `Authorization: Bearer sk_...`.
- Tier/permission hints from 4xx body: "You are not authenticated".
- Probe-safe endpoint used: `GET /v1/customers`.

## Top Workflows
1. Create a hosted payment link (payment session) with line items and email/SMS/WhatsApp it to a customer — the flagship "cobra con un link" flow.
2. Create authenticated customer/portal auth links to let customers view invoices and manage subscriptions.
3. Create recurring subscriptions and process automatic charges (cargos automáticos) against saved payment methods.
4. Emit SAT CFDI tax invoices for invoices/payments and track their status (timbre + cancellation).
5. Watch payments and webhook events, detect failures and refunds, and reconcile revenue.

## Table Stakes (competitor features to match)
- Full CRUD per resource matching every official endpoint (customers, payment concepts, discounts, invoices, payments, subscriptions, tax invoices, payment methods, webhooks, payment/setup sessions, auth/portal links).
- `--json`, `--select`, `--compact`, `--csv`, `--dry-run`, typed exit codes.
- Paginated list commands with filters (customers support a rich `filter` in qs format).
- Auth via env var.

## Data Layer
- Primary entities: customers, invoices, payments, subscriptions, payment_concepts, discounts, tax_invoices, webhooks, webhook_events.
- Sync cursor: paginated lists keyed by createdAt/updatedAt.
- FTS/search: cross-entity search over synced names/descriptions/IDs/emails.

## Product Thesis
- Name: Quentli CLI (`quentli-pp-cli`).
- Why it should exist: a payments/invoicing CLI with a local mirror so ops can answer "who is behind on an invoice?", "what is my outstanding balance?", "which subscriptions are about to fail?" offline and agent-natively — not just fire single API calls.

## Build Priorities
1. Created: hosted payment-link sessions and auth/portal links (flagship).
2. Reconciliation/ops transcendence over the local mirror (outstanding balance, overdue invoices, failing subscriptions).
3. Full generated endpoint surface for CRUD on every resource.
4. Webhook/session helpers.
