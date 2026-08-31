# internal/mcp/intents.go nondeterministic emission

## Why this patch belongs in the printed tree

Observed directly in this session: two back-to-back `generate --force` runs
against the same spec, research.json, and traffic-analysis.json produced
different output. One run emitted `internal/mcp/intents.go` (defining
`RegisterIntents`) as expected. The very next run, with no source changes,
emitted `internal/mcp/tools.go` calling `RegisterIntents(s)` but did **not**
emit `internal/mcp/intents.go` at all -- a build failure
(`undefined: RegisterIntents`) that the generator's own post-merge
validation caught and preserved a pre-failure snapshot for, but did not
self-heal. This is entirely generator-owned, fully generated content; it is
not a hand-edit. Worth a Printing Press retro on its own.

## Reprint guard

If a reprint's `go build ./...` fails with
`internal/mcp/tools.go:NN: undefined: RegisterIntents`, the print silently
dropped `internal/mcp/intents.go` (and its paired `recipe_intents_test.go`).
Recovery: regenerate again (this bug did not reproduce twice in a row in
this session), or restore both files from the last known-good
`generate --force` snapshot / prior build. Do not hand-author a
replacement -- `RegisterIntents` is a generated recipe-lifted-intent
dispatcher tied to the current `research.json` recipes; a stale hand-copy
will silently drift from the recipes text.
