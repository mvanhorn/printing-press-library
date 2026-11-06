# T9 main reconciliation

## Reconciliation decisions against both parents

- Archive retains the reprint's twelve resources, shared syncResource, context/deadline/cancellation, error propagation, scoped checkpoints, real ID extraction and stdout/stderr behavior. Main's separate paging loop would duplicate the shared loop; its purge now runs there for both archive and explicit sync. The channel file remains byte-exact to HEAD.
- Shared page parsing recognizes empty arrays as non-nil lists, including resource-name envelopes. A list response purges both pre-fix resource and resource-singleton rows once per resource, including zero lists. Failure propagates as an integrity error. Singleton validation and non-JSON/failure response handling remain.
- Stage 2 contained no existing Store.Delete method. Main's new method was ported using lockForWrite/unlockAfterWrite (including SQLite sidecar hardening), BeginTx/ExecContext cancellation and the current ftsRowID(resourceType,id) composite key. Resource and FTS deletion share one transaction. Delete delegates to DeleteContext for existing main callers; shared sync passes its active context. No duplicate Delete or legacy one-argument FTS key remains.
- Existing cursor strategy already follows hasMore plus nextCursor for short token pages; page/offset fallbacks and stuck-page/cursor protection remain. Existing natural empty-page completion precedes page-cap warnings. No duplicate main pagination expression was inserted.
- Offline zero-row lists consult the stored canonical partition's completed sync watermark. Never-synced lists still error, and unsupported row-selecting filters still fail closed. Existing valid-row filtering and projections remain.
- Webhook filter-types retains the current shared ParseStringList implementation, explicit Changed semantics, validation, context-aware request and partial-failure handling. Main's improved help text is retained.
- All seven tests from main's 477-line matrix_fixes_test.go remain in the 510-line adapted file. Search arguments use current Search(query,limit,...resourceTypes); missing Get rows recognize sql.ErrNoRows; archive fixtures account for all twelve resources and scoped resume checkpoints; sync event assertions read stderr, and the new machine-summary stdout is also asserted. Empty envelope nil/non-nil assertions are preserved. No tests deleted or assertions weakened.
- New TestDeleteContextAtomicFTSFailureAndCancellation proves canceled deletes preserve the resource/search hit, and failure of the second FTS statement rolls back the first resource DELETE. Main's existing tests separately assert wrong-resource isolation, resource counts, preserved real IDs, empty synced offline lists, cursor traversal and direct zero orphan FTS count.


## Preservation and runtime evidence


Manifest, README, NOTICE, tools manifest, MCP tool registration/policy, paid consent constructor, profile resolver and HTTP client safety are byte-exact to HEAD. Greg Ceccarelli creator / Cathryn Lavery contributor remain unchanged on all three surfaces. All 21 novel contracts, seven workflow groups, vault bearer auth and retired cookie behavior, both linkNeedsConfirm POST entries with read_only false, explicit paid --yes, strict recipe/download/undo safeguards and request idempotency are retained. All older patch records remain; only the existing reprint patch gains this reconciliation explanation and new test path. The new main patch record is unchanged. Registry and cli-skills were not manually edited; unrelated automatic merge outputs were left alone.


## Validation

All requested gates passed with GOCACHE=<build-cache>/go-cache and GOTMPDIR/TMPDIR=<build-cache>/tmp:

- Full go test -count=1 ./...: PASS, T9-test.log.
- go vet ./...: PASS, T9-vet.log.
- go build ./...: PASS, T9-build.log.
- CLI and MCP builds to <local-evidence> and <local-evidence> PASS, T9-cli-build.log / T9-mcp-build.log.
- Focused main matrix regressions: PASS, T9-focused.log.
- git diff --check for Tella: PASS, empty T9-diff-check.log.