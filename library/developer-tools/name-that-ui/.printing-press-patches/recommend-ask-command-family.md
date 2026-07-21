# Recommend and ask command family

Preserve `internal/cli/recommend.go` and `internal/cli/ask.go` as markerless
`registerNovelCommand` extensions. They are local-only, deterministic helpers
over the synced component and style mirrors; regeneration must not register
them in generated `root.go`, add a live endpoint, shell out, or introduce an
LLM route.
