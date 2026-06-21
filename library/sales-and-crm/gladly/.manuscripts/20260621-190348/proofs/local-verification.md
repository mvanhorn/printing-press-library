# Local verification for gladly-pp-cli

This CLI was generated from a read-first OpenAPI surface and verified locally before publishing.

## Verification run

- `go test ./...`: passed
- `go vet ./...`: passed
- `go build ./...`: passed
- `gladly-pp-cli --help`: passed
- `gladly-pp-cli version`: passed
- `gladly-pp-cli doctor --json`: passed with expected no-credentials diagnostics
- `gladly-pp-mcp --help`: passed

## Live API phase5 status

Live vendor API acceptance was skipped because this environment does not have Gladly credentials. The machine-readable skip marker is `phase5-skip.json` in this proofs directory.

## Shipped agent-native features

- `sync`: local SQLite mirror for repeatable/offline analysis
- `search`: full-text search over synced or live data
- `analytics`: read-only SQL against synced records
- `which`: agent-facing command discovery
- MCP stdio server: runtime Cobra-tree tool surface
