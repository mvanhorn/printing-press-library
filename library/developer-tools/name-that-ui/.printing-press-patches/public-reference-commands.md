# Public reference command family

Preserve `internal/cli/public_reference.go` as a markerless
`registerNovelCommand` extension. It promotes the public `translate` and
`updates` surfaces without editing generated `root.go`. Keep `// pp:data-source
public`, the bounded unauthenticated page/XML fetches, source provenance,
explicit partial-failure warnings, and the dry-run/no-local-mirror contract.
Regeneration must not replace these source-backed parsers with whole-page text
scraping or inferred framework mappings.
