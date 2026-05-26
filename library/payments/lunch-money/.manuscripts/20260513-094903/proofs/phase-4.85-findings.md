# Phase 4.85 Output Review Findings (Wave B warnings only)

## Fixed
- WARNING **list-shape null vs []** — 6 list-shape commands (triage, transactions subscriptions, transactions duplicates, recurring missing, accounts stale, budgets burn) returned JSON `null` for empty results. Changed declarations to `make([]T, 0)` so empty case marshals as `[]`. Verified.

## Deferred (not fixed in-session; suitable for Phase 5.5 polish)
- WARNING **empty-state TTY UX** — Empty-state TTY/table output for triage, transactions duplicates, accounts stale, budgets burn shows only the header row with no "no results" message. Add a stderr footer ("No results.") after the header when row count is 0.
- WARNING **net-worth on conflates zero with no-data** — Returns `{"total":0,"by_account_type":null}` for dates with no preceding balance history. Add `"has_history": false` flag + stderr note when no balance_history rows precede the requested date.

Both deferred items are minor UX/output enhancements; both pass the 7/7 plausibility checks at the shipcheck level.
