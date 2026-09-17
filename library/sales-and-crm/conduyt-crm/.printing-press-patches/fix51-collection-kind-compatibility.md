# Fix 51: collection kind compatibility

- `reports_compare.go` treats a metric as a dimension member only when its path
  contains an index selector, using the collection immediately before the last
  selector. Member churn is non-fatal only when that collection has the same
  array or object kind in both windows. Absent, null, scalar, and array/object
  collection changes produce a partial typed failure naming both kinds, even
  when either collection has no numeric descendants; ordinary nested object
  metrics remain fixed metrics.
