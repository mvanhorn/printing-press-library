# Fix 53: root marker collision

- `reports_compare.go` keeps the document root as a distinct empty internal
  collection path and applies the human-readable `$` label only when formatting
  comparison output. A JSON property literally named `$` therefore remains an
  ordinary key for collection kinds, member classification, and fixed metrics.
