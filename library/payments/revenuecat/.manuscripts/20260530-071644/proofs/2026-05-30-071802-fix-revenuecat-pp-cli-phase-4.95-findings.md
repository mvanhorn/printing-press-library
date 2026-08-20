# Phase 4.95 Local Code Review — revenuecat-pp-cli

Reviewer: general-purpose subagent (adversarial), scope = hand-authored novel
code only (8 commands + rc_helpers.go + revenuecat_project.go + migration).
Generated code out of scope. Review path: direct subagent dispatch (Agent tool).

## Autofix summary (fixed in-place, build+vet+test+shipcheck re-green after)
3 must-fix + 1 should-fix correctness bugs autofixed:
1. rc_helpers.go `points()` object-form — was collecting the timestamp into Values
   and reading a random map value (nondeterministic MRR/churn/trial numbers).
   FIX: skip the timestamp key, collect remaining keys in sorted order.
2. dunning_alert.go — per-customer recoverable invoice total double-counted into
   the project RecoverableUSD when a customer has multiple recoverable subs.
   FIX: count each customer's invoice total toward the project total once.
3. entitlement_rollup.go — empty customer id counted as one distinct active
   customer (collapsing all anonymous rows). FIX: skip rows with empty cid.
4. refund_cascade.go — POST refund discarded the HTTP status. FIX: treat >=400
   as an error so a non-2xx envelope can't be reported as a successful refund.

## Clean axes (verified)
- refund_cascade money-action safety: default trace-only, --apply required,
  short-circuits under --dry-run AND IsVerifyEnv(); destructive POST cannot fire
  under verify/dogfood/dry-run.
- SQL: NULL-safe scans throughout (sql.NullString + map decode), SELECT-only,
  bound params, correct flat/hierarchical resource_type IN(...) coverage.
- Empty slices marshal as [] not null. defer db.Close()/rows.Close() present.
- Verify-friendly RunE order correct on all 8 commands.

## Deferred to Phase 5 live dogfood (TODO(verify) markers, cannot confirm offline)
- Chart query-param names (mrr-trend --period -> resolution; trial-funnel
  --since -> start_date): a wrong param name silently returns default-resolution
  data, not an error. Confirm against live charts once the Secret key is set.
- Overview metric ids (mrr/arr/active_subscriptions/active_trials/revenue).
- conversion_to_paying count-vs-ratio semantics.
- Invoice unpaid linkage (no status/subscription_id field; customer-granularity join).

## Retro candidates (generator-level, NOT patched in printed CLI)
- internal/store UpsertBatch parent_id not populated for hierarchical resources
  (customers_subscriptions, customers_purchases, customers_virtual_currencies,
  offerings_packages): 4 generated tests fail on a pristine checkout. File against
  the Press. Novel commands read customer_id from JSON, not the parent_id column,
  so no runtime impact on this CLI.

Verdict: SHIP (structural). Behavioral confirmation pending Phase 5 live dogfood.
