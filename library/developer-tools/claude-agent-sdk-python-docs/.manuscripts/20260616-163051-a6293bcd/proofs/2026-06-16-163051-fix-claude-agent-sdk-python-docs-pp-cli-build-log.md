Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

## Built
- Added a hand-authored docs intelligence layer over Claude Code raw Markdown pages.
- Added absorbed commands: `read`, `search`, `symbol`, `examples`, and `guide`.
- Replaced generated novel placeholders with real implementations: `verify`, `context`, `diff`, `recipe`, `map`, `coverage examples`, and `audit-links`.
- Implemented docs fetch, Markdown section parsing, symbol extraction, code example extraction, link auditing, code import verification, and structured JSON output.

## Deferred
- None intentionally. The diff command reports entity hashes and a baseline path, but does not mutate the baseline unless a user writes that file themselves.

## Verification so far
- `go test ./...` passes.
- Behavioral samples for `symbol`, `context`, `examples`, and `verify` returned relevant structured output.
