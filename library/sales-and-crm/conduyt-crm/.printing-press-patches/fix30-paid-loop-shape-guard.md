# Fix 30 — paid-loop response-shape guard

- `internal/cli/contacts_verify_line_type.go` validates every successful
  verification response before counting the batch or issuing another paid POST.
- Error/message envelopes, missing or wrongly typed required fields, and invalid
  `byLineType` maps stop immediately through the existing partial API-error path.
- Regression coverage proves each malformed response issues exactly one POST.
