# Fix 29 — strict response-shape guards

- `internal/cli/imports_watch.go` owns the shared collection decoder. Collection
  consumers must reject valid JSON objects unless they contain a supported array
  envelope; only explicitly named single-resource endpoints may opt into objects,
  and error/message envelopes remain invalid.
- `contacts_verify_line_type.go`, `imports_blame.go`, and `send_check.go` must
  propagate collection-shape failures through their existing partial API-error
  output paths. `imports_watch.go` and `automations_hours_audit.go` likewise reject
  error/message-shaped documents before treating them as domain records.
- `reports_compare.go` must fail with its partial comparison envelope when either
  fetched window contains no comparable numeric metrics.
- Regression coverage lives alongside the existing command tests in
  `imports_watch_test.go` and `reports_compare_test.go`.
