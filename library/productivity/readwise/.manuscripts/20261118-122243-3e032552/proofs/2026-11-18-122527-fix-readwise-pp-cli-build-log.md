Manifest transcendence rows: 9 planned, 9 built. Phase 3 will not pass until all 9 ship.

# readwise-pp-cli Phase 3 build log (run 20261118-122243-3e032552)

## Slice 0: merge foundation (orchestrator, 2026-11-18)
- Reprint strategy: regen-merge the library CLI (master eb24962) onto the fresh 4.35.6 tree in a scratch merge workspace ($MERGE_DIR), never against the library.
- The base 4.24.0 template emission was synthesized by building press v4.24.0 on GOTOOLCHAIN=go1.26.11, because the press's own base synthesis fails on Go 1.27 (enetx/http2). With the base, the classification is CLEAN=62, BODY-DRIFT=7, VALUE-DRIFT=7, WITH-ADDITIONS=22, NOVEL=9, COLLISION=2, PUBLISHED-ONLY=5, NEW=132.
- The first --apply was REFUSED by the patch records (exit 5). It would have deleted mirror.go, mirror_test.go and two MCP surface tests, and reverted hand body edits in connections, dedupe, digest and triage stale. All 12 envelope markers survived. See regen-merge-apply-refusal.log.
- Applied with the records held out, then restored them. Copied the fresh non-Go embeds (playbooks/MANIFEST.md, platform testdata).
- Result: the tree does not compile at the template seams (cobratree signatures, store ftsMatchQuery). Handed to slice A.

## Slice A: compile + restore recorded patches (codex), DONE
- The gate passes (re-run by the orchestrator): go build, go vet, `go test -count=1 ./...` (15 pkgs), and `publish validate` patches=passed.
- Restored mirror.go and mirror_test.go, tool_surface_test.go, walker_surface_test.go, and the bodies of connections --library-wide, dedupe, digest and triage stale. Adapted the cobratree, store and helpers seams to the 4.35.6 signatures. The canonical envelope is retained.
- One marker changed (mcp-search-sql-byte-budget): `db.Search(query, limit, offset)` became `db.SearchContextOffset(queryCtx, query, limit, offset)`, for the 4.35.6 cancellation API.
- New hand-owned file internal/store/search_count.go (+test) shares the native short-token FTS fallback for totals.
- Superseded by 4.35.6: the auth set-token argv handling, agent-context schema, generic search display, the triage parent wiring, and the SQL materialization limits.
- INCIDENT: the slice ran the merged binary against the default store dir, which wrote data.db.owner and touched the header/WAL. The schema stayed v4 and the tables are value-identical to the snapshot. Logged to issues-encountered.jsonl.
- NOTE: StoreSchemaVersion=14 vs on-disk 4. Installing the new binary will one-way migrate the live store, so snapshot first.
- Full detail: SLICE-A-REPORT.md in the merge workspace.

## Slice B: drift behaviours the spec cannot express (codex), DONE
- Deletion-aware incremental export sync: `includeDeleted=true` on incremental runs, tombstones removed from the typed tables and FTS, `deleted` count in the summary.
- `documents bulk-update`: 200, 207 and 400 handling. A 207 prints per-item results, warns "N of M updates failed", and exits 5.
- `review get`: the 404 "Review not found." response becomes an exit-0 empty review. Any other 404 is still exit 3.
- `documents list --tag` is repeatable (max 5, AND semantics, empty matches untagged). The local path fails closed. The MCP schema exposes it as an array.
- Tag-list shapes: book-tags is a plain array, highlight-tags is the paginated envelope. Both decode.
- 4 new patch records (reader-bulk-update-partial-success, daily-review-absent, reader-repeatable-tags, book-tag-array-response). readwise-sync-export-rate-limits was extended. 19 JSON records in total.
- One mutation per item, each confirmed failing (see slice-AB-codex-report.md). The live store was untouched: same hash, no WAL.

## Phase 3 close (orchestrator)
- Ran `go mod tidy` (regen-merge had flattened the indirect requires). The merged tree is now the build in $CLI_WORK_DIR. The pristine fresh output is kept at $FRESH_PRISTINE.
- Rebuilt .printing-press.json from this run's fresh manifest plus novel_features. The merge had carried the library's stale description, the 22 tool count, and no homepage.
- Changed the webhook add Example host from hooks.slack.com to example.com, an unverified-host dogfood issue.
- Completion gate:
  - Per-row resolution: 47 PASS, 29 SKIP (generated/stub), 0 MISS. Transcendence: 9/9 PASS. See proofs/phase3-resolution.txt.
  - P1 spot check: highlights list, documents save and books update. help=0. dry-run=0 (books update needs its <id>, giving a structured usage error without it). JSON valid.
  - Dogfood backstop: verdict PASS, novel_features_check planned 9, found 9.
- Deferred to Phase 5 live: whether `highlights update --location-type` is honoured by the API (the flag stays removed unless it is).
- Known nit for Phase 4: `webhook add` returns nil from its dryRunOK branch, which gives empty stdout under --json.

