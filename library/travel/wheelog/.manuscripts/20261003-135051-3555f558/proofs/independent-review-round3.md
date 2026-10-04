# WheeLog independent review — round 3

Status: HOLD. Final readiness is held for the descriptor-pinned immutable snapshot fix, the reproduced writable-DSN path bug and restoration of the unsupported exclusivity introduction and schema-upgrade wording. This is a review gate, not publication approval.

## Scope and open findings

The same dedicated reviewer reviewed the post-PR saved-cache change, canonical read/write aliases, lazy source fallback, platform link-count helpers, documentation and actual rebuilt CLI/MCP output. No other reviewer was launched; no source or GitHub changes were made.

- **R3-5 — P1, internal/store/wheelog_read.go:139, saved-reader connection is not bound to the verified inode.** In the pathname-based immutable reader, temporarily replacing the database for the lazy SQLite open and restoring it before the final stats can return substituted observations while both identity checks pass. Root supplied a reproduced regression. Fix underway: pin an opened, fstat-verified source descriptor, copy to a private immutable snapshot, guard descriptor/path/sidecar state across copying and reading, and make SQLite read only that snapshot. Final code/runtime review remains pending.
- **S1 recurrence — P2, README.md:156 and SKILL.md:69.** The Press runner restored “These capabilities aren't available in any other tool for this API.” The integration cannot substantiate exclusivity. Restore the approved source-specific introduction after the last runner; verify the final published documents.
- **R3-6 — P1, internal/store/wheelog_write.go:47 (generated DSN in internal/store/store.go:177).** A literal question mark in an existing selected database filename truncates the generated writable DSN. With synthetic files named `saved?literal.db` and `saved`, `shortlist remove 166345 --db saved?literal.db` exits 0 and claims removal, while the selected file retains its row and the unselected `saved` loses its row. A missing selected question-mark path also creates/migrates the truncated file before returning an error. Construct a writable DSN that preserves the real path, or refuse ambiguous paths before any writable open/migration. Test both existing caches and the missing-path case. Actual shipping proof: `independent-review-round3-writer-uri-before.json`.

- **D6 — P2, README.md:309 and AGENTS.md:62.** The upgrade notes say merely opening a database upgrades its one-way schema and makes older binaries refuse it. Saved-only opens now skip migrations and leave the schema untouched. Qualify these notes as writable opens and exempt saved-only reads.

## Resolved findings and inspected evidence

- The Greptile P1 is resolved in the current saved-read flow: it skips migrations, directory creation, table creation and chmod; missing cache/table is empty; corruption is an explicit error. Auto discovery/inspection use the source first and lazily load saved fallback only after a source failure.
- The reviewer reproduced stale aggregate counts through symlink and hard-link aliases with a canonical writer's committed WAL. Canonical path resolution, selected-path identity checks and single-link refusal close those alias cases. Active WAL/rollback journal state is explicit `cache_visibility_unavailable`, and closing the writer reveals the newest zero-count observation.
- The initial portable metadata probe failed open on Windows, whose Stat metadata omits link counts. The platform helper now reads NumberOfLinks with FILE_READ_ATTRIBUTES, shares existing readers/writers/deletes, closes its handle explicitly and fails closed on unavailable metadata. Native hard-link behavior and the Windows cross-build pass; no native Windows runtime was available.
- Source save/refresh/remove now canonicalize the selected target before writable open and guard selected identity/single-link state around mutations and commits. Fresh tests reject hard links before alternate sidecars, alias addition and retargeting. An actual shipping CLI symlink save retained latest count 4 and previous count 0 after closing an existing canonical writer. Root's actual MCP writer proofs reject all three hard-link mutations and retain the same transition.

## Checks completed before the pending pinned-read change

- Fresh source: `go test -count=1 ./internal/store ./internal/cli -run 'TestWheelog(Snapshot|ReadGuard|Saved|Auto|Writer|SourceSave)'` — PASS.
- Cross-build: `GOOS=windows GOARCH=amd64 go build ./...` — PASS.
- Independent shipping CLI checks: canonical symlink retention; hard-link changes/remove rejection; zero alternate sidecars — PASS.
- Independent actual stdio MCP: 22 tools; all eight domain tools; no import/hollow shortlist; 0400 cache retains exact counts and bytes/permissions without sidecars; active committed WAL errors; checkpointed newest count 0 — PASS.
- Root's five-tool saved-reader and three-tool writer output artifacts were inspected. These support the same count/error/source behavior. Source privacy and original query/date/distance/report-gap bounds remain unchanged by this patch.

Evidence: `independent-review-round3-final-runtime.json`, `independent-review-round3-mcp.json`, `independent-review-round3-alias-before.json`, `independent-review-round3-alias-after.json`, `independent-review-round3-semantics.json`, and `greptile-fix/mcp-{saved-read,writer}-summary.json` plus sampled responses. Earlier native proof hashes predate the pinned snapshot change; they will be replaced/supplemented after rebuilding.

## Phase statuses

- Phase 14 — SKILL semantic: HOLD (S1 exclusivity recurrence).
- Phase 15 — README/SKILL/AGENTS correctness: HOLD (S1, D6; final snapshot wording pending).
- Phase 16 — output plausibility: HOLD (substituted-row regression must pass on the new runtime).
- Phase 17 — code correctness/security/source/privacy/migrations: HOLD (R3-5 pinned-read fix and R3-6 wrong-database mutation).

## Phase 14 semantic checks

| Check | Status | Assessment |
|---|---|---|
| Trigger phrases | PASS | Specific public spot and equipment evidence triggers match the actual domain. Anti-triggers exclude guarantees, contributor histories and account actions. |
| Verified command set | PASS | Rebuilt MCP exposes all eight domain tools. Import and the hollow shortlist grouper remain absent. Previously verified generic commands are unchanged. |
| Novel feature descriptions | PASS | The five approved features remain concrete: question matrix, source detail expansion, exact two-observation changes, audit queue and saved straight-line proximity. |
| Publish gates/stubs | PASS | Domain children run real implementations. Publication readiness remains held for the concrete findings in this report. |
| Authentication guidance | PASS | Public RPC reads are anonymous. No credentials, cookies or resident browser are required for supported source reads. |
| Recipes and evidence claims | PASS | Concrete recipes preserve aggregate question counts, unknown dates, bounded coverage and local mutations. Original round2 recipe verification remains applicable. |
| Marketing claims | HOLD | README and SKILL again contain the unsupported universal exclusivity sentence. Restore the approved source-specific introduction. |

The reviewed source/binary snapshot is recorded in `independent-review-round3-snapshot-before-pin.json`. It predates the pending fixes. The same reviewer remains available for resolution after rebuilding; no PASS or publication approval is granted for this snapshot.
