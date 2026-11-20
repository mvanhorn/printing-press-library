# Readwise sync complete books list

Generated-tree patch: supplement the Readwise v2 `GET /export/` sync with a
paced `GET /books/` list pass.

Why this belongs here:
- `GET /export/` only returns books that have at least one highlight, so using
  it as the sole books source omits zero-highlight books from the offline
  mirror.
- The `/export/` book object also lacks `/books/` list metadata such as
  `last_highlight_at`, `updated`, `highlights_url`, and `source_url`, leaving
  typed book columns empty after sync.
- Readwise `GET /books/` is page-based and capped at 20 requests/minute, so the
  hand-authored sync path must use the slower proactive pacing already used for
  Readwise's other 20/minute list endpoints.
- The `/books/` upsert intentionally runs after `/export/` so the richer book
  row is the final idempotent write while `/export/` remains the source for
  highlights.

Files:
- `internal/cli/readwise_sync_export.go`
- `internal/cli/readwise_sync_export_test.go`
