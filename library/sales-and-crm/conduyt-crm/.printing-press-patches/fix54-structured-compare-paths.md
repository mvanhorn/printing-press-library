# Fix 54: structured report comparison paths

- `reports_compare.go` represents report metric and collection paths as typed
  property and array-selector segments. Internal map keys use an injective
  length-prefixed encoding, while ancestry and dimension classification operate
  on segments rather than rendered strings. Human-readable output escapes dots
  and brackets in literal property names, so those properties cannot collide
  with nested properties or generated array selectors.
