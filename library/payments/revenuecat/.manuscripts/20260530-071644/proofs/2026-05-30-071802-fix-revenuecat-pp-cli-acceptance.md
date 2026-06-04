# Phase 5 Live Dogfood Acceptance — revenuecat-pp-cli

Live API: https://api.revenuecat.com/v2, project proj006f456b (read-only v2 Secret key).
Project is pre-launch: 5 test customers, $0 MRR, 0 active subs, 0 products/subscriptions/entitlements defined.

## Gate: PASS
Binary-owned matrix (`dogfood --live --level full`): **203/203 passed, 0 failed.**
phase5-acceptance.json: status=pass, level=full, auth=bearer_token.

## Novel commands verified live (manual smoke)
- revenue-snapshot: PASS — metric ids confirmed (mrr, active_subscriptions, active_trials, revenue, +active_users, new_customers); persists snapshot + diff.
- mrr-trend: PASS — after a parser fix (see below). Real periods (2026-02..05), resolution=month, --limit honored.
- trial-funnel: PASS — conversion_to_paying confirmed to return a COUNT (not ratio).
- churn-watch / dunning-alert / entitlement-rollup: PASS — correct empty results + honest notes (no subscriptions/entitlements exist to join yet).
- webhook-audit: PASS — empty (no webhooks configured), honest note.
- refund-cascade: PASS — graceful 404 on missing id, typed exit 5; default trace-only.

## TODO(verify) items RESOLVED against live data
- Chart `values` shape: **was wrong, fixed.** Real shape is row objects {cohort:<unix_SECONDS>, measure:<idx>, value:<num>, incomplete}, grouped by cohort with values indexed by measure. Original parser misread timestamps as MRR values. Fixed in rc_helpers.go points() + tests; re-verified live.
- Overview metric ids: confirmed (active_subscriptions/active_trials/mrr/revenue/active_users/new_customers).
- conversion_to_paying: confirmed a count.
- Chart resolution: "month"/"day" string resolution; --period passes through as `resolution` query param.

## Generator-level issues found (retro candidates; NOT novel-code bugs)
- offerings sync: HTTP 400 — generated sync sends `expand=metadata,packages` but the API only allows `expand=items.package`/`items.package.product`. Generated expand-param derivation is wrong for this endpoint.
- integrations sync: 1 resource errored (webhooks list path/param).
- internal/store UpsertBatch parent_id not populated for hierarchical resources (pre-existing; 4 generated tests fail on pristine checkout).

## Behavioral coverage limit (honest)
The local-join logic (subscriptions×invoices for dunning; entitlements×active_entitlements×subscriptions for rollup; churn dollar exposure) is structurally correct and empty-data-correct, but could not be exercised against POPULATED data because the project has no subscriptions/entitlements yet. Re-verify these joins once real subscription data exists.

Verdict: ship.
