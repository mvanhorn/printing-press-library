# Phase 4.95 Local Code Review

## Review path
Direct subagent dispatch: documentation semantics, output plausibility, and security/correctness reviewers.

## Autofix summary
2 feature-level plausibility findings were fixed in-place before the final acceptance run: agent-envelope mirror ingestion now classifies products/items/ETA correctly, and normalized mass/volume units are labeled in their base units. The generated binary and full live dogfood were rerun afterward.

## Template-shape retro candidates
The source reviewer identified generator-reserved patterns that were not patched in this printed CLI:

- `internal/client/client.go` redirect handling strips only `X-API-Key`; arbitrary configured credential headers/query values may cross a host redirect.
- `internal/cli/deliver.go` and `internal/cli/feedback.go` use fixed-timeout standalone HTTP clients rather than the root command timeout/context.
- `internal/store/store.go` contains nullable sync-state scans into non-null Go types.
- `internal/cli/feedback.go` can expose a configured feedback endpoint in response text.
- `cmd/quickcommerce-pp-mcp/main.go` allows non-loopback HTTP MCP binding without authentication.

These are emitted generator/template behavior and should be fixed in Printing Press rather than hidden by a one-off printed-CLI patch.

## Documentation fixes
- Replaced generic `example-value`/`42` examples with real QuickCommerce values.
- Corrected nonexistent `list` troubleshooting guidance.
- Added QuickCommerce API-key fields to the MCPB manifest and made the client profile optional.
- Corrected `--deliver` prose to state that output is also written to stdout.

## Convergence outcome
No unresolved in-scope feature or documentation findings remain. Final binary test and full live dogfood passed.
