# Fix 49: round 33 collection and total integrity

- `reports_compare.go` distinguishes one-sided dimension-member churn from a
  missing outer dimension collection. Member churn is non-fatal only when the
  outermost indexed collection exists as an array or object in both windows;
  an absent collection makes every metric beneath it a partial API-class
  failure naming the collection and missing window.
- `helpers.go` retains the first positive pagination total and returns the
  typed pagination truncation error if a later page reports a different total,
  so complete-enumeration callers cannot trust a drifting final total.
