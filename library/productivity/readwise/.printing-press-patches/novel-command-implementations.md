# Novel Command Implementations

## Why this local patch exists

The generated `readwise-pp-cli` tree included eight novel command scaffolds whose
`RunE` handlers returned TODO errors. This patch fills those hand-authored units
with local SQLite-backed implementations so the printed CLI's advertised novel
features are usable.

## Reprint guard

On reprint, preserve the implemented command files and tests unless the upstream
Printing Press catalog has learned equivalent implementations for:

- `ask`
- `backlog`
- `dedupe`
- `read-next`
- `digest`
- `connections`
- `mirror`
- `triage stale`

The implementations are intentionally scoped to the generated stub files under
`internal/cli/` and their existing `_test.go` companions. They use the generated
store schema, helper output functions, `ParseDurationLoose`, and local SQLite
data only; no Readwise account mutation is introduced.
