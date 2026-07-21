# Component command family

Preserve `internal/cli/component.go` as a markerless `registerNovelCommand`
extension. It provides local, read-only NameThatUI component mirror commands
over the `components` resource, including conservative resolution and
source-backed guidance. Regeneration must not move this registration into the
generated `root.go`.
