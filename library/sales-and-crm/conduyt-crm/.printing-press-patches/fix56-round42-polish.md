# Fix 56: round 42 polish

- Import side-effect verification prefers the current `retryableFailed` and
  `exhaustedFailed` counters, accepts legacy counters only when the current key
  is absent, and fails closed on invalid or conflicting dual-key values.
- Published-only hours audits classify an automation after filtering its graph
  to audited action types, so the automation and excluded summary counts
  reconcile even when a published graph contains only trigger/control nodes.
