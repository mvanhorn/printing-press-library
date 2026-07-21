# Style command family

Preserve `internal/cli/style.go` as a markerless `registerNovelCommand`
extension. It provides local, read-only NameThatUI style-mirror commands over
the `style_details` resource, including deterministic identification,
conservative resolution, and source-backed section filtering. Regeneration
must not move this registration into the generated `root.go`.
