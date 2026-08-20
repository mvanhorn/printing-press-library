# RevenueCat-pp-cli — Novel Features Brainstorm (audit trail)

> Subagent output, Phase 1.5 Step 1.5c.5. First print (prior=none). Personas
> grounded in the brief's Users + Top Workflows. 8 survivors, all hand-code.

## Customer model

**Mara — solo-founder of the PartnerUp mobile app, checks revenue every morning before her standup.**
- Today: opens RC dashboard Overview tab for MRR/active-subs, clicks into Charts to eyeball MRR-movement and churn, bounces to a third tab for billing-issue subscribers, keeps a half-broken Google Sheet of weekly MRR by hand. Cannot answer "which subscribers churned this week and how much MRR did they take" without manual scrolling. No diff vs last Monday.
- Weekly ritual: Monday 8:45am — pull MRR/ARR/active-subs/trials, glance week-over-week, scan for grace/billing-issue subs, before 9am standup.
- Frustration: dashboard shows charts but won't *compose* them into one pasteable/pipeable line; nothing is diffable over time.

**Devin — contract mobile dev who runs PartnerUp release weeks, lives in the terminal.**
- Today: watches for billing breakage during launches. On a "I paid but lost my Gold entitlement" email he checks customer → active entitlements → subscription status → refund history across four dashboard screens. Can't script "every customer whose entitlement state disagrees with subscription state."
- Weekly ritual: daily release-week sweep of failed/grace subscriptions + spot-check that webhook deliveries to PartnerUp's Cloud Function are healthy (a dropped webhook silently desyncs entitlements).
- Frustration: entitlement-vs-subscription-vs-refund reconciliation is a manual four-tab join; webhook health is invisible until something breaks.

**Priya — future operator of the unified `partnerup-revenue-cli` who needs LS and RC to speak the same JSON.**
- Today: runs `lemonsqueezy-pp-cli` for the web rail (clean `--agent` JSON keyed on tier names). Mobile rail has no comparable shape — RC data only leaves the dashboard differently. Can't produce combined "MRR across web + mobile by tier" without hand-normalizing two exports.
- Weekly ritual: consolidated revenue read across both rails, joined on shared tier names, for the founder report.
- Frustration: no RC driver with the LS `--agent` contract, so rails can't be joined mechanically.

## Candidates (pre-cut)

14 candidates generated (C1–C14). Sources: (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries, (e) user vision (LS mirror).

Inline kill/keep applied at Pass 2:
- C12 ltv-cohort, C13 retention-curve, C14 actives-movement — thin 1:1 wrappers of single `charts/{chart_name}` enums already absorbed by generated `charts get`; cut.
- C10 agent-export — not a command; the LS-shape `--agent` contract is a framework flag, folded into C1/C2/C3 outputs; cut as standalone.
- C4 trial-funnel — RC has no LS analog but `trials_new` → `conversion_to_paying` is real cross-chart synthesis; kept to Pass 3.
- C8 campaign-watch — LS tracked capped discount codes; RC has no campaign/capped-code primitive. Reframed to offering/package conversion pace; kept to Pass 3 for adversarial test.

## Survivors and kills

### Survivors (transcendence rows)

| # | Feature | Command | Score | Buildability | How It Works | Long Description |
|---|---------|---------|-------|--------------|--------------|------------------|
| 1 | Revenue snapshot | `revenue-snapshot` | 9/10 | hand-code | `metrics/overview` + `metrics/revenue`, persists each run to local `snapshots` table, diffs vs prior row, emits LS-shape tier-keyed `--agent` JSON | Use this for the current-moment revenue rollup and its diff vs last run. Do NOT use it for the MRR-over-time line; use 'mrr-trend'. |
| 2 | MRR trend | `mrr-trend` | 8/10 | hand-code | Joins `mrr` + `mrr_movement` chart series into one new/expansion/contraction/churn table with WoW deltas | Use this for MRR over time and its movement breakdown. Do NOT use it for a single current-moment total; use 'revenue-snapshot'. |
| 3 | Churn watch | `churn-watch` | 8/10 | hand-code | Joins `churn` chart against local `subscriptions` mirror (billing-issue/grace/expired/cancelled), sums per-sub dollar exposure | Use this for who churned and the dollar exposure. Do NOT use it for the recoverable still-failing window; use 'dunning-alert'. |
| 4 | Dunning alert | `dunning-alert` | 8/10 | hand-code | Local join of `subscriptions` in grace/billing-issue × unpaid `invoices`, ranked by recoverable amount | Use this for the recoverable failed-billing window (still grace/billing-issue). Do NOT use it for already-expired churned subs; use 'churn-watch'. |
| 5 | Entitlement rollup | `entitlement-rollup` | 8/10 | hand-code | Three-way local join: project `entitlements` × per-customer `active_entitlements` × `subscriptions` status; flags entitlement/subscription disagreements | none |
| 6 | Refund cascade | `refund-cascade <id>` | 7/10 | hand-code | Walks subscription → transactions → refund history → entitlement loss for one id; `--apply` calls live refund (data-source auto) | Use this to trace or issue a refund for one subscription or purchase and see the entitlement fallout. Do NOT use it for aggregate refund-rate trends; use 'charts get refund_rate'. |
| 7 | Trial funnel | `trial-funnel` | 7/10 | hand-code | Joins `trials_new` + `conversion_to_paying` chart series into a stage-to-stage funnel with per-stage drop | none |
| 8 | Webhook audit | `webhook-audit` | 6/10 | hand-code | Local `integrations/webhooks` mirror grouped by destination host, flags duplicate/stale destinations | none |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C8 campaign-watch | RC has no capped-code/campaign primitive; reframed version is a thin `offerings` × `actives_new` overlay run at most occasionally, not weekly | `mrr-trend` |
| C10 agent-export | Not a command — LS-shape tier-keyed `--agent` JSON is a framework flag; folded into snapshot/trend/churn outputs | `revenue-snapshot` |
| C11 customer-360 | Wrapper-vs-leverage fail: absorbed `customers get` + sub-resources + `search`/`sql` already assemble it; real need (cross-customer disagreement) is `entitlement-rollup` | `entitlement-rollup` |
| C12 ltv-cohort | Thin 1:1 wrapper of `ltv_per_customer`/`cohort_explorer` chart enums already in `charts get` | `mrr-trend` |
| C13 retention-curve | Thin 1:1 wrapper of `subscription_retention` chart enum already in `charts get` | `churn-watch` |
| C14 actives-movement | Thin 1:1 wrapper of `actives`/`actives_movement` chart enums already in `charts get` | `mrr-trend` |
