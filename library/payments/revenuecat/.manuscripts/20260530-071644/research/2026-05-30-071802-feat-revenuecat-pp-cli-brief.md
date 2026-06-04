# RevenueCat (REST API v2) CLI Brief

## API Identity
- Domain: `api.revenuecat.com/v2` — RevenueCat's server-side Developer API v2
  (OpenAPI 3.0.3, title "Developer API", version 2.0.0). RevenueCat is the
  dominant cross-platform in-app subscription/billing SaaS for mobile apps.
- This is the **billing/subscriber** API, NOT the revenuecat.com marketing site
  (a separate website-content manuscript exists at
  `manuscripts/revenuecat/20260528-143019` and is intentionally left untouched
  for a future `revenuecat-content-pp-cli`).
- Users: mobile app teams running subscriptions through RevenueCat who need
  programmatic visibility into MRR/ARR, churn, trials, entitlements, refunds,
  and webhook health from the shell or an agent — without clicking through the
  dashboard. For PartnerUp specifically, this is the mobile-rail twin of the
  shipped `lemonsqueezy-pp-cli` (web rail).
- Data profile: project-scoped. **Every** path is
  `/projects/{project_id}/...`, so the CLI needs a configured project id
  (mirrors the LS CLI's store id). 74 path templates, 100 operations across
  domains: App, Audit Log, Charts & Metrics, Collaborator, Customer,
  Entitlement, Offering, Package, Product, Virtual Currency, Purchase,
  Subscription, Invoice, Paywall, Integration, Project.

## Reachability Risk
- Low. Official, documented, stable REST API. Auth is a standard bearer token.
- v2 is still expanding coverage vs v1 (some v1-only use cases remain), but
  every endpoint this CLI targets (metrics, charts, customers, subscriptions,
  entitlements, purchases, invoices, webhooks) is present in v2.
- Rate limiting is per-domain (e.g. Customer Information = 480 req/min);
  charts/metrics share their own domain budget. Fan-out commands must respect
  this — use the framework's adaptive limiter and surface RateLimitError.

## Auth
- `auth.type: bearer_token`. Spec declares `securitySchemes.bearerAuth`
  (`type: http`, `scheme: bearer`, `bearerFormat: auth-scheme`). Header:
  `Authorization: Bearer <key>`.
- Key is a **v2 Secret API key** (`sk_...`), created in the RC dashboard under
  Project Settings → API Keys → Secret API keys. Distinct from the public SDK
  keys (`EXPO_PUBLIC_...`, `appl_`/`goog_`) already in
  `apps/mobile/.env.local`. Recommend a READ-ONLY secret key — every headline
  command is read-side.
- Canonical env var: `REVENUECAT_API_KEY` (with a `_SECRET_KEY` fallback to be
  decided at generate time). Project id env var: `REVENUECAT_PROJECT_ID`.
- Per repo convention, both go in `~/.zshenv` so subagents/Bash see them in
  non-interactive shells. Never commit the key value.

## Top Workflows
1. **Revenue snapshot** — "what's my MRR / ARR / active subs / revenue right
   now?" → `metrics/overview` (point-in-time, currency-selectable) +
   `metrics/revenue` (ranged).
2. **MRR / ARR trend over time** — `charts/{chart_name}` with `mrr`,
   `mrr_movement`, `arr` chart names; period-over-period deltas.
3. **Churn / retention watch** — `charts/{chart_name}` with `churn`,
   `subscription_retention`, plus `subscriptions` list filtered by status
   (active / in-grace / billing-issue / expired).
4. **Trial funnel** — `trials`, `trials_new`, `trials_movement`,
   `conversion_to_paying` charts.
5. **Entitlement rollup** — per-customer `active_entitlements`, project
   `entitlements`, and entitlement→product attachments.
6. **Refund accounting** — `refund_rate` chart + purchase/subscription/
   transaction refund actions and history.
7. **Webhook health audit** — `integrations/webhooks` config + delivery state.
8. **Dunning / billing-issue alerts** — subscriptions in grace/billing-issue
   state + customer invoices.

## Table Stakes (absorb)
- Full typed endpoint coverage of all 100 v2 operations (list/get/create/
  update/delete/actions). The generator emits these.
- `doctor` that validates the bearer key + project id against the live project.
- `--json` / `--select` / `--csv` / `--compact` on every read.
- Local SQLite store + `sync` + offline `search` / `sql` over customers,
  subscriptions, entitlements, products, offerings, purchases, invoices.

## Data Layer
- Primary entities: Project, App, Customer, Subscription, Entitlement, Product,
  Offering, Package, Purchase, Invoice, Transaction, Webhook integration,
  Virtual currency.
- Sync cursor: list endpoints are cursor-paginated (`starting_after` style /
  `next_page`). Persist a per-resource sync cursor.
- FTS/search: customers (by id/alias/attributes), products (by store id),
  subscriptions (by status/product), entitlements (by lookup key).

## User Vision (from kickoff)
- Build the RC mirror of the shipped `lemonsqueezy-pp-cli`: same novel command
  family — `revenue-snapshot`, `mrr-trend`, `churn-watch`, `dunning-alert`,
  `entitlement-rollup`, `refund-cascade`, `campaign-watch`, `webhook-audit` —
  adapted to RC's v2 data model.
- `--agent` JSON output shape should match the LS CLI so unified tooling
  (future `partnerup-revenue-cli`) can swap drivers and join on shared tier
  names (`plus`/`gold`/`platinum`, `hobbyist`/`fulltime`/`pro`).
- Exit bar: Greptile 5/5 + CI green (per the established Press PR flow).

## Product Thesis
- Name: `revenuecat-pp-cli` — "Every RevenueCat v2 endpoint, plus a local
  database, offline search, and revenue/churn/refund intelligence the dashboard
  charts can't compose."
- Why it should exist: launch-week mobile-subscription visibility is otherwise
  dashboard-only. This makes MRR/churn/refund/webhook state queryable and
  agent-drivable, and gives the future unified `partnerup-revenue-cli` an
  RC driver with the same JSON shape as the LS one.

## Build Priorities
1. Foundation: project-scoped client + bearer auth + `doctor` + SQLite store +
   sync/search/sql over the core entities.
2. Absorb: all 100 typed v2 endpoint commands.
3. Transcend: the LS-shape novel command family, built on `metrics/*` and
   `charts/{chart_name}`, joined with local subscription/customer state.
