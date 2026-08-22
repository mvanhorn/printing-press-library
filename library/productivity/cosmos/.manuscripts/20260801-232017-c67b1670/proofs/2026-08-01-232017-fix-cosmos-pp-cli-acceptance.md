# Acceptance Report: Cosmos

- Level: Full dogfood
- Gate: PASS
- Tests: 118/118 passed
- Skipped: 98 inapplicable, fixture-blocked, or write-side checks
- Authentication: bearer credential available to the isolated runner
- External mutations: none; destructive testing was not enabled

## Fixes applied

1. Removed raw GraphQL transport commands from agent and MCP discovery while retaining their captured query documents as internal adapters.
2. Added stable help examples without displacing the runner's live fixture resolver.
3. Made an empty snapshot history return an actionable `needs_snapshots` result instead of an error.
4. Bounded similarity-trail expansion to eight API calls.
5. Decoded HTML entities and removed Cosmos `<n>` highlight markup from normalized text output.

## Printing Press observations

- The static dogfood analyzer treats delegated novel handlers as hand-rolled even though all five live analysis commands call the shared captured-query adapter.
- Three unused helpers are generator-owned and remain as generator retro candidates.
- The generic data-pipeline analyzer does not recognize the CLI's private JSON membership snapshots.

