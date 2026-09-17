# Fix 52: root collection comparison

- `reports_compare.go` records the document root as `$`, so root-array indexed
  member churn follows the same collection-kind rules as nested dimensions.
  Same-kind root arrays remain non-fatal, including an empty window, while root
  array/object/scalar kind changes produce a partial typed failure naming `$`.
