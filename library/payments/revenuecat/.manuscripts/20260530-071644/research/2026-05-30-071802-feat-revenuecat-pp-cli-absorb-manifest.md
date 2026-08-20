# RevenueCat-pp-cli — Absorb Manifest

Source: RevenueCat Developer API v2 (OpenAPI 3.0.3), 74 paths / 100 operations,
base `https://api.revenuecat.com/v2`, bearer-token auth. First print.
No competing RC CLI/MCP found in research — the absorbed surface is the v2 API
itself; "added value" is offline store + agent-native output over every endpoint.

## Absorbed (match or beat everything that exists)

Generator emits typed commands for all 100 v2 operations. Grouped by family:

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Customers list/get/delete | RC v2 API | (generated endpoint) customers list/get/delete | offline store, --json/--select/--csv |
| 2 | Customer actions: grant/revoke entitlement, transfer, assign_offering, restore_purchase | RC v2 API | (generated endpoint) customers actions * | scriptable, --dry-run, typed exits |
| 3 | Customer sub-resources: aliases, attributes, active_entitlements, invoices, purchases, subscriptions, virtual_currencies | RC v2 API | (generated endpoint) customers <sub> | offline join targets |
| 4 | Subscriptions list/get | RC v2 API | (generated endpoint) subscriptions list/get | offline store |
| 5 | Subscription actions: cancel, extend, refund | RC v2 API | (generated endpoint) subscriptions actions * | --dry-run, typed exits |
| 6 | Subscription sub-resources: entitlements, transactions, authenticated_management_url | RC v2 API | (generated endpoint) subscriptions <sub> | composable |
| 7 | Entitlements CRUD | RC v2 API | (generated endpoint) entitlements crud | offline search |
| 8 | Entitlement actions: archive/unarchive, attach/detach products | RC v2 API | (generated endpoint) entitlements actions * | scriptable |
| 9 | Products CRUD + create_in_store + archive/unarchive | RC v2 API | (generated endpoint) products * | offline store |
| 10 | Offerings CRUD + archive/unarchive + packages | RC v2 API | (generated endpoint) offerings * | offline store |
| 11 | Packages get + attach/detach products | RC v2 API | (generated endpoint) packages * | composable |
| 12 | Purchases list/get + refund | RC v2 API | (generated endpoint) purchases * | typed output |
| 13 | Invoices list + file download | RC v2 API | (generated endpoint) customers invoices | scriptable |
| 14 | Virtual currencies CRUD + balance/transactions | RC v2 API | (generated endpoint) virtual_currencies * | offline store |
| 15 | Webhook integrations CRUD | RC v2 API | (generated endpoint) integrations webhooks | scriptable |
| 16 | Overview metrics (point-in-time MRR/ARR/actives/revenue) | RC v2 API | (generated endpoint) metrics overview | currency flag |
| 17 | Ranged revenue metric | RC v2 API | (generated endpoint) metrics revenue | date range |
| 18 | Chart data by chart_name (mrr, churn, trials, refund_rate, ltv, retention, ...) | RC v2 API | (generated endpoint) charts get | enum chart names |
| 19 | Projects, Apps (+ keys, storekit config), Collaborators, Audit logs, Paywalls, Media assets | RC v2 API | (generated endpoint) project resources | offline store |

Every absorbed row resolves through the generator's typed endpoint surface; the
local SQLite store, `sync`, `search`, and `sql` commands cover offline/agent use.

## Transcendence (only possible with our approach)

8 hand-code novel commands (the Phase 3 hand-code commitment), all read-side
except `refund-cascade --apply`. Mirror the shipped `lemonsqueezy-pp-cli`
family, adapted to RC's v2 data model. The `--agent` output of revenue-snapshot,
mrr-trend, and churn-watch follows the LS tier-keyed JSON shape so the future
`partnerup-revenue-cli` can swap drivers.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Revenue snapshot | revenue-snapshot | hand-code | Persists each run to a local `snapshots` table and diffs vs the prior row — point-in-time + delta no single dashboard view gives; emits LS-shape tier-keyed `--agent` JSON | Use this for the current-moment revenue rollup and its diff vs last run. Do NOT use it for the MRR-over-time line; use 'mrr-trend'. |
| 2 | MRR trend | mrr-trend | hand-code | Joins the `mrr` + `mrr_movement` chart series into one new/expansion/contraction/churn movement table with week-over-week deltas — synthesis across two chart series | Use this for MRR over time and its movement breakdown. Do NOT use it for a single current-moment total; use 'revenue-snapshot'. |
| 3 | Churn watch | churn-watch | hand-code | Joins the `churn` chart against the local `subscriptions` mirror filtered to billing-issue/grace/expired/cancelled, summing per-sub dollar exposure | Use this for who churned and the dollar exposure. Do NOT use it for the recoverable still-failing window; use 'dunning-alert'. |
| 4 | Dunning alert | dunning-alert | hand-code | Local join of `subscriptions` in grace/billing-issue × their unpaid `invoices`, ranked by recoverable amount — the recoverable window the dashboard can't compose | Use this for the recoverable failed-billing window (still grace/billing-issue). Do NOT use it for already-expired churned subs; use 'churn-watch'. |
| 5 | Entitlement rollup | entitlement-rollup | hand-code | Three-way local join of project `entitlements` × per-customer `active_entitlements` × `subscriptions` status, flagging customers whose entitlement state disagrees with subscription state | none |
| 6 | Refund cascade | refund-cascade <id> | hand-code | Walks subscription → transactions → refund history → entitlement loss for one id from the local mirror; `--apply` calls the live refund action (data-source auto) | Use this to trace or issue a refund for one subscription or purchase and see the entitlement fallout. Do NOT use it for aggregate refund-rate trends; use 'charts get refund_rate'. |
| 7 | Trial funnel | trial-funnel | hand-code | Joins `trials_new` + `conversion_to_paying` chart series into a stage-to-stage funnel with per-stage drop-off — synthesis across two distinct chart enums | none |
| 8 | Webhook audit | webhook-audit | hand-code | Groups the local `integrations/webhooks` mirror by destination host, flagging duplicate or stale destinations — local grouping the dashboard list view doesn't do | none |

### Dropped vs the LS family (surfaced for gate review)
- `campaign-watch` — KILLED. LS tracked capped discount codes; RC v2 has no
  campaign/capped-code primitive. The reframe (offering/package conversion pace)
  is a thin `offerings` × `actives_new` overlay run occasionally, not weekly.
  The user can re-add it at the gate if they want the paywall-pace view.

Stubs: none. All 8 transcendence rows ship fully.
