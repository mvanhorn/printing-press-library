# WheeLog independent review — round 4

**PASS. No open findings in the requested scope.** The same dedicated reviewer completed the resolution review directly; no additional reviewer was launched and no source or GitHub writes were made. This records the review gates only, not publication approval.

Reviewed frozen source and shipping binaries after the final snapshot filename change and the owner's final-document signal. Final binary SHA-256: CLI `7cb9e321061b7fc3618f4e1a34e935ea43e284b8ef35f739716a2c13ea248fd2`; MCP `d91729b33537eab01d89aeb7fdf8c6779a93ab8913c9656ae5b485d09e9139dd`. Complete source/document fingerprints are in `independent-review-round4-snapshot.json`. Both shipping artifacts are newer than frozen Go source.

## Resolved findings

| Finding | Severity | Resolution and evidence |
|---|---|---|
| Greptile saved-read writable-open failure | P1 | Saved reads bypass migration, creation and chmod. Auto search/inspect/compare try live first and read saved fallback lazily; cache failures retain explicit source/fallback reasons. Fresh store/CLI regressions pass. |
| Symlink/hard-link stale WAL observations | P1 | Canonical target resolution, selected-path checks, platform single-link validation and WAL/journal guards reject uncertain snapshots. Actual CLI/MCP output refuses active committed WAL and shows the newest zero count after checkpoint. |
| Windows link-count visibility | P1 | FILE_READ_ATTRIBUTES/NumberOfLinks replaces the missing Stat metadata probe, handles close/errors explicitly and fails closed on unavailable metadata. Native hard-link regressions and Windows cross-build pass. |
| Canonical/alias writer observation loss | P1 | Canonical writable opening and one reserved sql.Conn keep Init/List/Observe/Remove on the supported database connection. Selected identity and single-link guards bracket mutations/commits. Canonical/symlink concurrency retains latest 4, previous 0; hard-link/retarget cases refuse mutations. |
| R3-5: lazy SQL consumed substituted rows | P1 | `wheelog_snapshot.go:29` pins an opened/fstat-verified source descriptor, copies a bounded private snapshot, checks descriptor/path/sidecar stability and lets SQL open only the private snapshot. The actual swap-and-restore row substitution regression now returns selected count 2 rather than substituted count 0. |
| R3-6: question-mark writable DSN changed the wrong cache | P1 | `wheelog_write.go:16` and `:44` reject raw/resolved URI-delimiter paths before writable opening. Existing two-cache, new-path and symlink-target cases refuse the operation and preserve both caches without creating a truncated filename. Final shipping CLI repeats the previously reproduced wrong-cache case successfully. |
| S1: unsupported exclusivity claim | P2 | Final README:156 and SKILL:69 state that the workflows combine verified public evidence with a bounded saved shortlist. The universal exclusivity sentence is absent after the final Press runners. |
| D6: saved reads falsely implied schema migration | P2 | Final README:309 and AGENTS:62 qualify upgrades as writable operations and explicitly exempt saved-only reads. |

Private snapshots use a 0700 temporary directory and 0600 file, a 64 MiB cap, context-aware copying and cleanup after success/error. Actual shipping checks cover URI-delimiter TMPDIR, SQL corruption, an over-limit sparse file, deadline cancellation, absent cache and read-only permissions. No private snapshot remains after these checks.

The supported writer review covers ordinary SQLite concurrency, canonical/symlink WAL behavior, hard-link and URI traps, and selected-path retarget refusal. Deliberate external same-user inode swap/restore during writable open was explicitly excluded by the root task; no copy-back or filesystem-locking redesign was requested or reviewed.

## Phase 14 — SKILL semantic: PASS

Each assessment is under 50 words.

| Check | Assessment |
|---|---|
| Trigger phrases | Specific public spot and equipment evidence triggers match the actual domain. Anti-triggers exclude guarantees, contributor histories and account actions. |
| Verified command set | Rebuilt MCP exposes all eight domain tools. Import and the hollow shortlist grouper remain absent. Previously verified framework commands are unchanged. |
| Novel feature descriptions | The five approved features remain concrete: question matrix, source detail expansion, exact two-observation changes, audit queue and saved straight-line proximity. |
| Publish gates/stubs | Domain children execute real implementations. Final shipcheck passes and no placeholder domain tool is exposed. Review dispatch remains separate from publication approval. |
| Authentication guidance | Supported public RPC reads are anonymous over ordinary HTTPS. No credentials, cookies or resident browser are required. |
| Recipes and evidence claims | Concrete recipes preserve aggregate counts, unknown dates, bounded coverage and local mutation scope. Snapshot size/path restrictions and migration behavior are now disclosed accurately. |
| Marketing claims | Final documents describe source-specific workflows and bounded evidence. Universal exclusivity and unsupported route, suitability, current-condition or measurement claims are absent. |

## Phase 15 — README/SKILL/AGENTS correctness: PASS

Final documents were inspected after the last runner. Command names, flags, positional IDs, concrete examples, file prerequisites, doctor Quick Start and dry-run action summaries retain the round2 corrections. The new snapshot/64 MiB/cleanup/reserved-connection/URI restrictions match the implementation. Writable-only schema notes are consistent with saved reads skipping migration. Anonymous source guidance and local learning/mutation scope remain explicit.

## Phase 16 — actual output plausibility: PASS

Inspected real sanitized source outputs and all five feature proofs in earlier rounds; provider transport and domain evidence semantics remain unchanged. Current rebuilt CLI/MCP samples preserve exact question IDs/counts/requirements and distinguish positive evidence, unreported zero counts, unknown clocks and explicit cache failures. Saved coverage, straight-line distance, audit reasons and exact change values remain preserved in default agent output.

Final stdio MCP has 22 tools, including all eight domain tools, with no import or hollow shortlist tool. A 0400 cache returns the selected counts without byte/permission changes or source sidecars; active committed WAL produces `cache_visibility_unavailable`; after checkpoint the newest count 0 and `unreported` requirement are retained. Source errors, cache errors and absent cache are not silently collapsed into invented observations.

The owner completed final canonical shipcheck and fresh live matrix: 106 passed, 0 failed; 68 declared skips are reported separately. The owner reports no hollow domain coverage. The final summary and check artifacts were inspected; independent runtime proofs below substantiate the affected cache/output paths.

## Phase 17 — code/security/source/privacy/migrations: PASS

Reviewed new snapshot/read/write helpers, platform link helpers, Store connection lifecycle, atomic observation/removal transactions, CLI source/cache mode handling, default output keep fields and durable schema2 patch metadata. Read-only snapshots do not change the source file/schema. Missing cache/table is empty; corrupt data and uncertain WAL/journal/identity state are explicit errors. Supported writes retain SQLite coordination and exact two-observation rotation; guards fail before unsafe alias/URI mutations.

The original anonymous public source allowlist and strict semantic envelope remain before generated cache/logging/output. No contributor profiles, raw narratives/photos/comments/TrackLogs, measured dimensions, individual report dates, current-condition guarantees or route guarantees are introduced. Date input windows remain inclusive calendar windows with UTC source echo; missing record updates remain unknown. Original caps remain five detail reads/five pages/50 output/50 saved/two observations, with the new 64 MiB private-copy bound.

Fresh narrowed source tests passed:

```text
go test -count=1 ./internal/store ./internal/cli -run 'TestWheelog(Snapshot|PrivateSnapshot|ReadGuard|Saved|Auto|Writer|SourceSave)'
GOOS=windows GOARCH=amd64 go build ./...
```

The owner also completed full Go tests/vet and final shipcheck/live gates. Local dependencies retain exact modernc.org/libc v1.77.1. No reserved cliutil/cobratree or global updater changes were requested. Native Windows execution was unavailable; the Windows helper was inspected and cross-built, not runtime-tested on Windows.

## Proofs

- `independent-review-round4-runtime.json`: twelve independent source/cache/CLI/MCP scenarios, including cap/deadline/error cleanup and URI path preservation.
- `independent-review-round4-mcp.json`: actual stdio responses for read-only, active WAL and checkpointed newest-zero stages.
- `independent-review-round4-final-smoke.json` and `independent-review-round4-final-mcp.json`: small repeat against the final rebuilt shipping artifacts after the benign private filename change.
- `independent-review-round4-snapshot.json`: final source/docs/binary fingerprints.
- `independent-review-round3-writer-uri-before.json`: actual wrong-cache mutation before the fix.
- `greptile-fix/shipcheck-final.json`, `live-full-summary.json`, `go-test.txt`, `go-vet.txt`, and saved-reader/writer MCP proofs: owner validation inspected for supporting evidence.

Final statuses: **phase14 PASS; phase15 PASS; phase16 PASS; phase17 PASS.**
