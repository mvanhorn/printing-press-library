# T1 reconciliation — complete

Scope: reconcile the current generated Tella CLI with the published customizations. The seven T2 additions, live API tests, shipcheck, installation, staging, commits, PRs and publication were not performed. Work stayed in the Tella catalog tree and `.pp-matrix`; the pinned Press binary was not rebuilt or installed.

## Merge decisions

## Preserved behavior

- All 16 prior `novel_features` entries are unchanged. The actual editing workflows, old custom tests, snapshot capture, explicit rollback, partial/indeterminate failure reporting, divergence refusal and exclusive-editing confirmation remain. Existing HTTP fixtures exercise their behavior.
- `videos storyboard <videoId> --include-transcript` still composes the prior editable timeline; the new generated `videos storyboard get-video` endpoint is also reachable. Likewise `videos timeline <id>` retains its flags and behavior alongside the generated child endpoint.
- Retained transcript aliases call `/transcript`, with uncut requests forcing `includeCut=true`. `get-transcript --start-ms/--end-ms` and the current long time flags coexist. Invalid windows, views and include-cut combinations fail before HTTP. The current generated results/provenance envelope remains; prior assertions now inspect its payload. Error tests assert the current structured error envelope and preserved error code, rather than demanding empty stdout.
- Thumbnail `--inpoint-ms` still sends `startTimeMs`; default JSON URL responses remain. Scale is required and validated from flags and stdin within 1–3.5. `apply-edits` retains the exact 24 supported types and 1–200 operation bounds. Doctor still performs an uncached authenticated account request. Export waiting retains uncached polling, progress, URLs and the hidden deprecated `--inbox-limit`.
- Retained preview-by-default audio, Studio Voice, transcript correction and silence-removal behavior while keeping fresh endpoint fields and response checks. Old JSON-object flags (`--dimensions`, `--point`, `--focus-point`, `--background`, `--trim`) coexist with generated scalar flags. Legacy required object members populate the current scalar flags so generated required-field validation is still exercised. HTTP tests verify repeated time/word ranges and legacy blur/zoom requests.
- Archive/sync preserve list-envelope extraction, zero rows for empty lists and comma-list webhook filters from 305ac7f731. Archive fixtures now cover the expanded generated resource list and scoped checkpoints. A resume fixture seeds an actual existing resource because the new framework deliberately restarts empty stores. Archive summaries include rows persisted before a later page failure. No singleton envelope rows are stored.
- Source uploads support video, audio, image, LUT and font with kind-appropriate metadata. Local fixtures verify upload bytes, source IDs, status and that neither API Authorization nor session cookies reaches the signed upload host. Ephemeral session-cookie overrides remain excluded from persisted config; the existing credential-isolation tests pass.
- Current generated `UpsertTranscript(json.RawMessage)` is separate from retained `UpsertCachedTranscript(...)`. The old plural `transcripts` and `transcripts_fts` data/schema and original FTS row-ID algorithm remain. A migration test opens a prior transcript cache, searches it, updates it, and proves the old FTS hit is replaced without duplicating rows.

## MCP and non-Go artifacts

## Attribution, provenance and historical files

- Original creator remains Greg Ceccarelli (@gregce) in the manifest, Go headers, README, SKILL, LICENSE and NOTICE. Course creator Lavery remains a contributor, including README and NOTICE surfaces.
- Fresh manifest/spec/docs and tool metadata retain current generation provenance. `spec_url` is `https://api.tella.com/openapi.json`; the manifest checksum matches the archived generated spec.
- Old manuscripts and all prior patch entries remain. The customization ledger adds `reprint-current-framework-workflows.json`; no registry or generated skill mirror edits were made.

## Test migration and disk failure audit

All prior test declarations survive except the two generated tests tied to the old singular thumbnail table:

| Prior test | Current equivalent |
| --- | --- |
| `TestMigrate_AddsColumnsOnUpgrade_Thumbnail` | `TestMigrate_AddsColumnsOnUpgrade_VideosThumbnail` |
| `TestUpsertBatch_PopulatesThumbnailTable` | `TestUpsertBatch_PopulatesVideosThumbnailTable` |

The new spec separates source and video thumbnail resources. These current tests exercise migration columns and typed batch persistence for `videos_thumbnail`; source-thumbnail equivalents also pass. The migration does not drop the old thumbnail table. No failing safety/custom test was deleted to pass a gate.

Internal-disk exhaustion truncated `internal/store/upsert_batch_test.go` to zero bytes. It was restored from the retained fresh source and the two custom test declarations were re-added from the prior source, adapted to the current Search argument order. Final audit found no unexpected empty product files, all JSON parsed, all Go syntax/format checked, all recorded patch paths exist, and all packages built/tested. GOCACHE, GOTMPDIR and TMPDIR stayed on the invocation's external SSD scratch after the pause; no shared caches were cleared.

## Verification

- PASS `go mod tidy` — `<local-evidence>`
- PASS `go build ./...` — `<local-evidence>`
- PASS `go vet ./...` — `<local-evidence>`
- PASS `go test -count=1 ./...` — `<local-evidence>`
- PASS targeted compatibility/upload fixtures, existing cache migration, MCP novel coverage, spec operation coverage, and unique sibling checks; full-suite log is `<local-evidence>`.
- PASS final provenance/ledger/manuscript/path/artifact audit — `<local-evidence>`; whitespace gate is clean.

### Exact preserved feature command paths

- `transcripts search`
- `videos viewed`
- `webhooks tail`
- `webhooks replay`
- `clips edit-pass`
- `clips transcript-diff`
- `sources upload-file`
- `videos clips add`
- `exports wait`
- `clips captions`
- `workspace stats`
- `videos storyboard`
- `videos apply-storyboard`
- `videos clips insert-file`
- `videos clips add-broll`
- `videos clips cut-words`
- `videos clips replace-word-ranges`
- `videos audit`
- `playlists edit-pass`
- `videos clips clean`
- `videos clips undo-last-cuts`
- `videos clips restore-cuts`

## Remaining limits

Live Tella auth, reads and writes are **unverified** in this task, as explicitly required. HTTP tests use local fixtures only. T2 workflow additions and shipcheck remain for the next task. Generated TODO scaffolds are not represented as implemented workflows. Existing unofficial Find Mistakes remains behind its original explicit gate until T2 replaces it. The API still offers no conditional cuts update; divergence checks remain advisory and restore still requires exclusive editing.
