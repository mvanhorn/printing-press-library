## Customer model

Four personas grounded in the brief's Users + Top Workflows (cobranza, portal links, recurring subscriptions, SAT CFDI, reconciliation).

1. **La Cobranza / Cuentas por Cobrar Ops (Mariela)** — Operations person at a Mexican SME (despacho, gimnasio, escuela, clínica) whose job is to get invoices paid.
   - **Today**: Lives in the Quentli dashboard + a spreadsheet; re-opens each overdue invoice by hand to see who it belongs to, whether it was paid, and whether a reminder or a fresh pay link is needed.
   - **Weekly ritual**: Every Monday she lists "who owes us," copies emails/WhatsApp numbers out, sends reminders (invoices reminders), regenerates payment-link sessions for stragglers, and marks paid what arrived over the weekend.
   - **Frustration**: No single offline answer to "what is my outstanding balance" or "who is behind and by how much." She juggles 4 tabs and a spreadsheet to build the exact list the platform already has inside it.

2. **FinOps / Revenue Reconciler (Regina)** — Accountant-engineer pairing responsible for making sure every completed payment is backed by a SAT CFDI tax invoice and that the books match the payment rails.
   - **Today**: Runs payments and tax-invoices reports, then manually cross-checks that completed/refunded payments have a valid (timbre) or canceled CFDI.
   - **Weekly ritual**: Before SAT filing she reconciles the month: every `PAYMENT_COMPLETED` should have a `VALID` tax invoice; every refund should reconcile against actual returned money.
   - **Frustration**: No command answers "which completed payments have no valid CFDI yet" without exporting and eyeballing two lists.

3. **Subscriptions / Retention Engineer (Diego)** — Technical owner of recurring billing (colegiaturas, memberships, suscripciones). He sets up `AUTOMATIC` subscriptions against saved card/bank-account payment methods.
   - **Today**: Watches dashboards for payment failures; when a charge misses he digs to find the customer, their payment method, and the failed `PAYMENT_ATTEMPT_FAILED`.
   - **Weekly ritual**: Reviews subscription health — which active subs are about to fail, which saved payment methods are expired/unconfirmed, which plan churned.
   - **Frustration**: Silent churn. He cannot see, in one pass, every active subscription tied to a broken (expired/unconfirmed/deleted) payment method — the single biggest cause of involuntary churn.

4. **Portal / Growth Ops (Pablo)** — Builds and distributes the "cobra con un link" payment sessions and customer/portal auth links.
   - **Today**: Creates a payment session with line items, then emails/SMS/WhatsApps the hosted link; for portal he creates an auth link so customers see invoices and manage subscriptions.
   - **Weekly ritual**: Checks which sessions converted to payments and which customers never opened their portal.
   - **Frustration**: Session objects have get/create but no aggregate view; he can't reconcile "links sent" vs "money received" without tracking session IDs himself.

---

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Long Description |
|---|------|---------|-------------|---------|--------|------------------|
| C1 | Outstanding balances | `quentli outstanding [--since 7d]` | List customers with unpaid invoice balances (totalAmount - amountPaid), totaled and grouped, offline. | Mariela (cobranza) | (c) invoices+customers join | `none` |
| C2 | Overdue invoices | `quentli overdue [--days 30]` | Rank unpaid invoices past dueDate by days overdue + amount left. | Mariela | (c) invoices local join | `none` |
| C3 | Collection / dunning queue | `quentli dunning [--since 1w]` | Per-customer portfolio queue: outstanding + overdue + next action (send reminder vs try-payment vs resend pay link) with email/WhatsApp contact. | Mariela | (c) invoices+payments+customers join + (a) persona | Points to `customer balance` for single-customer drill-down. |
| C4 | CFDI tax-invoice reconciliation | `quentli reconcile [--period 1m]` | Cross-check completed/refunded payments and paid invoices against SAT tax-invoice status (VALID/CANCELED/PENDING) to find gaps before filing. | Regina (finops) | (c) tax_invoices+payments+invoices join + (b) SAT content patterns | `none` |
| C5 | Revenue report | `quentli revenue [--since 30d]` | Payments/refunds by status, type, currency over a window; net collected vs returned. | Regina, Diego | (c) payments local aggregation | `none` |
| C6 | Subscriptions at-risk | `quentli subs at-risk` | Active subscriptions whose payment method is expired/unconfirmed/deleted or whose latest `PAYMENT_ATTEMPT_FAILED`; ranked by recovery urgency. | Diego (retention) | (c) subscriptions+payment_methods+payments join + (a) persona | Points to `customer balance` for per-subscription drill. |
| C7 | Churn report | `quentli churn [--since 30d]` | Customers whose subscriptions moved to CANCELED in the window, with last payment date and amount. | Diego | (c) subscriptions+customers join | `none` |
| C8 | Webhook delivery health | `quentli webhooks health [--since 24h]` | Failed/RETRYING webhook events per webhook, plus `PAYMENT_ATTEMPT_FAILED` events surfaced as an operational alert, with a retry path. | Pablo / eng | (c) webhook_events local join + (b) webhook content patterns | `none` |
| C9 | Link sessions status | `quentli paylink status [--since 7d]` | Local mirror of payment/setup sessions we created, showing which converted to a payment. | Pablo | (c) sessions local mirror | `none` |
| C10 | Currency amount helper | `quentli amount 150000` | Render minor-currency amounts as `MXN 1,500.00`. | Mariela, Regina | (c) local format helper | `none` |
| C11 | Cards / payment-methods at risk | `quentli cards expiring` | Customers with expired or unconfirmed payment methods on active subscriptions. | Diego | (c) payment_methods+subscriptions join | Merged into `subs at-risk`. |
| C12 | Refund tracker | `quentli refunds [--since 30d]` | Refunds by amount/status over a window. | Regina | (c) refunds local | Merged into `revenue`. |
| C13 | Invoice aging buckets | `quentli aging` | AR-aging buckets (current, 30/60/90+) across the invoice mirror. | Mariela | (c) invoices local | Merged into `dunning`. |
| C14 | Customer balance drill-down | `quentli customer balance <id>` | Single customer: outstanding invoices, active subscriptions, payment methods, CFDI linkage. | Mariela, Diego | (c) multi-entity join per customer | Long: `dunning` for whole portfolio; this is per-customer drill. |

Inline rubric triage (cut/reframe immediately):
- **C9 paylink status** — no list endpoint for sessions in the spec (only create/get); the local mirror would only hold sessions this CLI itself created, so "did it convert" is unverifiable in dogfood. → reframe toward webhooks; effectively cut as standalone.
- **C10 currency helper** — useful but trivial as its own command; it is a render routine, not a data command. Fold into every survivor; no standalone command.

---

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|------------------|
| 1 | Collection / dunning queue | `quentli dunning [--since 1w] [--db]` | 9/10 | Mariela (cobranza) | hand-code | Joins `invoices` (isPaid, amountPaid, totalAmount, dueDate, collectionMethod) + `customers` (email, phoneNumber) in local SQLite to compute outstanding & overdue per customer and a next action (reminder vs try-payment vs resend link), rendered via the minor-currency helper | Brief thesis "who is behind on an invoice?", "what is my outstanding balance?"; workflows #1 cobranza & #5 | For the whole-portfolio collection queue. Use `customer balance <id>` for a single customer's drill-down. |
| 2 | CFDI tax-invoice reconciliation | `quentli reconcile [--period 1m]` | 8/10 | Regina (finops) | hand-code | Joins `tax_invoices` (status VALID/CANCELED/PENDING, uuid) + `payments` (status) + `invoices` (isPaid) in local SQLite to flag completed/refunded payments missing a valid timbre | Brief workflow #4 "emit SAT CFDI... track status (timbre + cancellation)"; data layer tax_invoices | `none` |
| 3 | Subscriptions at-risk | `quentli subs at-risk [--db]` | 8/10 | Diego (retention) | hand-code | Joins `subscriptions` (status, collectionMethod) + `payment_methods` (confirmed, expired, deletedAt) + `payments` (failed attempts) in local SQLite to surface active subs on broken/missing payment methods or failed attempts | Brief workflow #3 "recurring subscriptions and automatic charges"; "which subscriptions are about to fail?" thesis | Whole-portfolio recovery queue; use `customer balance <id>` for a single subscription's details. |
| 4 | Revenue report | `quentli revenue [--since 30d] [--csv]` | 7/10 | Regina, Diego | hand-code | Aggregates local `payments` by status/type/currency and `refunds` by amount over a window, computing net collected vs returned in minor units via the currency helper | Brief workflow #5 "reconcile revenue"; data layer payments | `none` |
| 5 | Webhook delivery health | `quentli webhooks health [--since 24h] [--db]` | 7/10 | Pablo / eng | hand-code | Aggregates local `webhook_events` by status (FAILED/RETRYING) per webhook and surfaces `PAYMENT_ATTEMPT_FAILED` events as an operational alert with retry routing | Brief workflow #5 "watch payments and webhook events, detect failures"; webhook-event enum PAYMENT_ATTEMPT_FAILED | `none` |
| 6 | Customer balance drill-down | `quentli customer balance <id> [--json] [--select]` | 7/10 | Mariela, Diego | hand-code | Single-key joins in local SQLite of `invoices`, `subscriptions`, `payment_methods`, `payments`, `tax_invoices` for one customerId to render a one-screen financial snapshot (outstanding, subs, methods, CFDI) | Brief thesis "offline... agent-natively"; multi-entity data layer | For one customer's financial snapshot. Use `dunning` for the whole-portfolio collection queue. |

All six survivors are hand-code (local aggregate joins), declare `// pp:data-source local`, are drain-first, and use the sync-hint helpers; none call `store.Upsert` inside a BeginTx write txn.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C1 outstanding balances | Subsumed by `dunning`, which computes the same outstanding per customer plus a next action — a standalone read adds no decision | `dunning` |
| C2 overdue invoices | Overdue ranking is the core of `dunning`; standalone version is a thinner duplicate with no action step | `dunning` |
| C7 churn report | Low weekly use for this persona vs `subs at-risk` (which prevents churn before it happens); CANCELED filter is a thin wrapper over the same join | `subs at-risk` |
| C9 paylink status | No list endpoint for sessions in the spec; local mirror only holds sessions this CLI created, so "did it convert" is unverifiable in dogfood — low confidence | `webhooks health` (INVOICE_PAID / PAYMENT_COMPLETED events) |
| C10 currency helper | A render routine, not a data command; trivial as a standalone bare command (scope creep toward an app); folded as an internal helper into every survivor | `dunning` / `revenue` (renders minor units) |
| C11 cards/payment-methods at risk | Identical join and same user pain as `subs at-risk`; standalone would double-maintain the same at-risk logic | `subs at-risk` |
| C12 refund tracker | Aggregation subset of `revenue`; no standalone persona need that revenue doesn't serve | `revenue` |
| C13 invoice aging buckets | AR-aging is one column of the `dunning` queue; standalone is a narrower duplicate competing for the same screen | `dunning` |
