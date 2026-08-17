# Raindrop local feature layer

Preserve these changes when reprinting:

- Keep `.raindrop-token` as a lower-precedence, owner-only credential source for local checkouts. Never copy its value into generated artifacts.
- Keep the public binary name `raindrop` while retaining the generated Go module layout.
- Preserve `internal/store/extras.go` migrations for history, cleanup plans, inbox sessions, reading state, and triage workflows.
- Preserve the hand-authored commands in `internal/cli`: change history, safe duplicate and tag plans, inbox review/apply, revisit and reading queues, related bookmarks, clusters, highlight digest/export, sync status/diff, offline search alias, and durable triage.
- Preserve the `golang.org/x/text` security floor at v0.39.0 or newer.
- Preserve `/raindrops/0` as the canonical all-bookmarks sync, search, and bulk-update endpoint; `/raindrops` supports none of those operations.

Reprint acceptance: `go test ./...`, `go vet ./...`, `make build-all`, Printing Press verify/dogfood/workflow/skill/narrative gates, bounded authenticated read-only sync, and token-leak scan must pass.
