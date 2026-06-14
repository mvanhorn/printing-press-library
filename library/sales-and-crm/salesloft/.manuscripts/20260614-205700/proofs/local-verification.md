# Local verification for salesloft-pp-cli

This CLI was generated from a read-first OpenAPI surface and verified locally before publishing.

## Verification run

- `go build ./...`: passed
- `go test ./...`: passed
- `cli-printing-press shipcheck --dir /Users/debmukherjee/printing-press/library/salesloft --spec /Users/debmukherjee/printing-press/library/salesloft/spec.yaml --no-live-check --json`: passed

## Live API phase5 status

Live vendor API acceptance was skipped because this environment does not have salesloft credentials. The machine-readable skip marker is `phase5-skip.json` in this proofs directory.

## Shipped agent-native features

- `sync`: local SQLite mirror for repeatable/offline analysis
- `search`: full-text search over synced or live data
- `analytics`: read-only SQL against synced records
- `which`: agent-facing command discovery
