# Fix 47: live contract closure

- Pagination aggregation must return a typed truncation error with already-fetched
  rows for repeated pages/cursors, missing cursors, page-cap hits, and later-page
  failures, as well as a reported total inconsistent with the fully enumerated
  rows. Hours audit renders those rows as partial and verifies checked totals;
  generic callers remain fail-closed.
- Dialer coverage rejects impossible provider totals without reporting a queue
  depth below the rows observed.
- Send-check batches `/contacts/dnc-status?ids=` at 500 IDs, consumes
  `data.statuses[*].smsBlocked` and `voiceBlocked`, and treats omitted or truncated
  statuses as partial. Imports blame consumes nested rows/meta/tabs and the live
  side-effect counter names. Funnel reports resolve an omitted ID only after
  complete pagination proves there is a single tenant pipeline.
- Keep runner `pp:happy-args` values in semicolon-token grammar and retain the
  verification run command's help example.
