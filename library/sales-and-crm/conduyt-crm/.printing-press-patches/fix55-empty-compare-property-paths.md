# Fix 55: empty report comparison property paths

- `reports_compare.go` renders an empty JSON property as `[""]` and inserts
  property separators by segment position, keeping public metric names injective
  without changing the existing escaping for dots and brackets.
- Command-level regressions cover an empty-property collision with an ordinary
  nested path, distinction from the document root, and an empty property below
  an indexed dimension member.
