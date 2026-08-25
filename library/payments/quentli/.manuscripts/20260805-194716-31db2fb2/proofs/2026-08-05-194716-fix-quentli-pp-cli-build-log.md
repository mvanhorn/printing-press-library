# Quentli CLI Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Built (Priority 2 transcendence) — all hand-code, all local-SQLite
1. dunning — collection/dunning queue (invoices x customers join)
2. reconcile — CFDI SAT tax-invoice reconciliation (payments x tax-invoices)
3. subs at-risk — at-risk subscriptions (subscriptions x payments)
4. revenue — revenue report (payments aggregation)
5. webhooks health — webhook delivery health (webhook-events x webhooks)
6. customer balance — single-customer financial snapshot (multi-entity join)

## Priority 0/1 (generator-emitted)
- Full generated endpoint surface for all 55 endpoints (customers, catalog, invoices,
  payment/setup sessions, payments, refunds, subscriptions, tax-invoices, webhooks,
  auth-links, portal sessions).
- Local mirror (sync/search/sql/analytics) + 6 novel store-query commands.
- Cloudflare MCP pattern applied (54 endpoints > 50).

## Wiring notes
- webhooks health required a registerNovelCommand hook (webhooks is a generator-owned
  command; the novel stub conflicted). Attached via init() hook in webhooks_health.go.
- All novel commands declare // pp:data-source local, are drain-first, use db.List +
  sync-hint helpers, and route output through printJSONFiltered/printAutoTable.
- Shared money helper: internal/cli/money.go (minor-currency formatting).

## Intentionally deferred
- None of the 6 approved transcendence features deferred.
- payment-methods resource is not syncable at top level (only GET /v1/customers/{id}/payment_methods),
  so subs at-risk/customer balance derive payment-method health from subscription fields + failed payments.
