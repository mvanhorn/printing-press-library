# Fix 46: Phase 5 live dogfood contracts

- Normalize only the exact Conduyt double-wrapped paginated list envelope and
  preserve its page metadata while projecting rows through JSON, table, CSV,
  selectors, pagination, and write-through caching.
- Keep specialized collection decoders aligned with that shared shape; true
  queue totals override fetched-row caps.
- Preserve the live-first/local-fallback contract for hours audit, leave
  scorecards uncapped by default, and separate the read-only line-type estimate
  from the explicitly paid `run --until-done` subcommand.
- Preserve the named generated endpoint commands' dogfood annotations and
  required query-flag validation. Regression coverage lives beside the CLI
  tests, including `phase5_live_dogfood_test.go`.
