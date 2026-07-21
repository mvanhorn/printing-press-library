# Hybrid identify command

Preserve `internal/cli/identify.go` as a markerless `registerNovelCommand`
extension. It ranks synced `components` and component parts deterministically,
then uses NameThatUI's two-stage semantic-search endpoint when allowed. Keep
the local fallback, ambiguity passthrough, provenance metadata, and dry-run
without database or network access; do not wire it into generated `root.go`.
