# Fix 48: round 32 completeness semantics

- `send_check.go` must return an API-class inconclusive result when an empty
  live audience also carries pagination evidence that contacts were omitted.
- `reports_compare.go` treats one-sided indexed dimension members as expected
  non-comparable data without making the result partial; one-sided fixed metrics
  and structurally non-comparable values remain fail-closed.
- Preserve the runnable unfiltered first `reports compare` help example and keep
  the optional assigned-user example syntactically valid.
