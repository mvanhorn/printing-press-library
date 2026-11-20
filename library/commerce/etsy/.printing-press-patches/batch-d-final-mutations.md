# Final Batch D mutation verification

Each mutation was applied individually, the named synthetic test failed, and the original implementation was restored. No commits were made.

- `TestEtsyFinalGuestTransactionKinds`: guest transaction PK guard.
- `TestEtsyFinalFloorDependents`: floor receipt memory.
- `TestEtsyFinalFloorDependents`: orphan parent memory.
- `TestEtsyFinalOrphanVeto`: orphan unshipped veto.
- `TestEtsyFinalShippedSelection`: unknown shipped kept.
- `TestEtsyFinalShippedSelection`: unshipped count only selected.
- `TestEtsyFinalGenericStorageID`: generic caller storage ID.
- `TestEtsyFinalFilterReadOnly`: filter read transaction.
- `TestEtsyFinalFilterReadOnly`: filter write mutex.
- `TestEtsyFinalReviewLinkWarning`: review link count.
- `TestEtsyFinalFloorDependents`: clear floor count.
- `TestEtsySimplifiedUnsuppressRestores`: guest unsuppress cleanup.
- `TestEtsySimplifiedUnsuppressRestores`: pre-unsuppress re-entry.
- `TestEtsyFinalTransparencyCLI`: CLI review warning.
- `TestEtsyFinalTransparencyCLI`: CLI readmission count.
- `TestEtsyFinalBuyerReceiptUnsuppressCLI`: buyer receipt diagnostic.
- `TestEtsyFinalLegacyReviewRejectedReplacement`: move legacy purge before replacement acceptance.
- `TestEtsyFinalFloorSyncRemembersReceipt`: feed only prefiltered receipt rows to upsert, losing rejected receipt memory.

Validation: build, vet, gofmt and diff checks passed. Full store, CLI, MCP and MCP conformance package runs passed (store and MCP re-run with the gate's credential variables unset after updating the old orphan expectation). Final CLI regressions and patch-record checks passed. The complete ./... test gate is reserved for the dispatcher.

The builder authored both implementation and tests. The dead constrained etsyErasureRowsWhere branch and direct assertion were removed; the Go resourceParentID end-to-end association regression remains. No schema version or release ledger was changed. The pre-existing listings_reviews batch ID-extraction issue remains out of scope.
