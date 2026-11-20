# Peer Review Correctness and Security Fixes

## Why this local patch exists

Independent review found correctness, concurrency, and privacy issues in the
hand-authored Readwise novel command and sync paths. These fixes belong in the
generated tree until the upstream Printing Press catalog learns equivalent
behavior.

## Reprint guard

Preserve these behaviors on reprint:

- Novel read-only commands open SQLite with `store.OpenReadOnly` and expose a
  `--db-path` override.
- Novel table-count helpers validate table names and propagate query errors.
- `sync` skips malformed books in `/export/` pages with a warning so cursors can
  advance, while transport and page-level decode failures remain hard errors.
- The fixed pacer allows the first/refilled request immediately and still paces
  subsequent requests at the configured interval.
- Sync JSON error events avoid echoing raw non-API error text.
- `mirror` writes private export directories/files, refuses non-incremental
  overwrites unless `--force` is set, and still supports incremental skips.

Files:
- `internal/cli/ask.go`
- `internal/cli/backlog.go`
- `internal/cli/connections.go`
- `internal/cli/dedupe.go`
- `internal/cli/digest.go`
- `internal/cli/helpers.go`
- `internal/cli/mirror.go`
- `internal/cli/read_next.go`
- `internal/cli/readwise_sync_export.go`
- `internal/cli/triage_stale.go`
- related tests in `internal/cli/*_test.go`
