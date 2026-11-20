# Readwise sync export and proactive pacing

Generated-tree patch: add a Readwise-specific `sync` runner outside generated
`internal/cli/sync.go`.

Why this belongs here:
- Readwise v2 bulk sync should use `GET /export/`, which returns books with
  nested highlights and is not in the 20 requests/minute list bucket.
- Reader v3 `GET /list/` and `GET /tags/` are capped at 20 requests/minute per
  token and need proactive pacing from the first request.
- Readwise cursors are endpoint-specific: v2 `GET /export/` can return numeric
  `nextPageCursor` values, while Reader v3 returns string cursors. The local
  runner must accept both forms and continue cursor pagination until the API
  returns an empty cursor, unless an explicit max-page cap is set.
- The generated default sync uses one unpaced client and relative v3 paths, so a
  narrow hand-authored runner is needed until Printing Press can model these
  per-resource endpoint/rate-limit rules upstream.

Files:
- `internal/cli/readwise_sync_export.go`
- `internal/cli/readwise_sync_export_test.go`
- minimal dispatch in `internal/cli/sync.go`

Incremental exports must set includeDeleted=true with updatedAfter, partition tombstones before upsert, and atomically remove both typed/generic highlight rows and FTS entries. Full sync also sends the parameter. Preserve deletion counts in human and JSON summaries. The short deletion transaction must finish before any Upsert call (SQLite has one writer).

E2 correction: request includeDeleted on every export walk, including full sync and cursor resumes. Checkpoints preserve the completed watermark and the run filter; full reset clears it. Book and highlight tombstones are applied per page in one transaction. All resources use the same event stream and summary; context errors remain fatal and preserve their causes.
