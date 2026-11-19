# Batch D simplified retention verification

I authored both the implementation and its new regression tests. All fixtures are synthetic.
No tests were deleted outright. The reconcile-era shipped-update test was replaced and renamed.
No commit or schema-version bump was made.

## Rewritten tests

| Test | Reason |
| --- | --- |
| TestEtsyRetentionShippedUpdateReconciles → TestEtsyRetentionShippedUpdateAwaitsPrune | A shipped update must persist (including typed is_shipped=1), with deletion deferred until explicit prune. Reviews survive receipt pruning. Keeps shared-listing, other-receipt, dependent and learning-cleanup assertions. |
| TestEtsyRound2ReviewCycle | Receipt-linked reviews now survive receipt pruning and subsequent writes; there is no persisted transaction deny set. |
| TestEtsyRound2OrphanAge | Orphan purchase rows use age at prune only. Re-import is allowed without a pruned-parent marker. Fixture now supplies a positive shipped timestamp so the unshipped veto does not apply. |
| TestEtsyRound2ParentDecisionAndTwins | Writing a dependent of an old stored parent preserves stored twin projections instead of deleting them. |
| TestEtsyErasureEmbeddedTransactions | The erase branch still removes linked reviews; the receipt-prune branch now keeps them. |
| TestEtsySuppressionMissingTimestamp | Original missing-timestamp rejection and newer-record checks remain. Only the contradictory final stale-copy assertion changed: suppression cannot delete existing copies. |
| TestEtsyBuyerAllowlistClassification | Removed the obsolete lazy transaction table exception; the privacy regex remains. |
| TestEtsyPrivacyTablesSQLDenied | Removed creation/testing of the obsolete lazy transaction table; remaining privacy tables stay denied. |

## New tests and mutants

| Test | Regression mutation it catches |
| --- | --- |
| TestEtsySimplifiedReviewsAndNeverStored | Reintroduce receipt-to-review pruning; skip the reviews floor; reject stored review updates; reject never-stored old unshipped receipts; permit never-stored old shipped receipts. |
| TestEtsySimplifiedUnsuppressRestores | Retain transaction denial metadata after erase, or fail to remove buyer/receipt suppression. Receipt, transaction and review must all return through batch writes for both forms. |
| TestEtsySimplifiedAssociationAndOrphan | Restore SQLite length(NUL-suffix) matching, lose scoped association cascades in erase/prune, or remove the null shipped_timestamp orphan veto. |
| TestEtsySimplifiedExplicitVacuumRerun | Restore the victims>0 condition for explicit erasure vacuum. |
| TestEtsySimplifiedScale | Restore scanning parent lookups or quadratic selection/counting: first 4000+4000 preview must finish within 2s, and filtering plus writing 2000+2000 pruned records within 3s. Also catches pruned re-import. |
| TestEtsySimplifiedLearnCleanupScope | Restore sync-time cleanup or id-only search_learnings matching across resource types. |
| TestEtsySimplifiedEmptyPruneVacuum | Skip explicit empty prune vacuum or report rebuild completed with no content FTS table. |
| TestEtsySimplifiedUnsuppressedCount | Restore a literal count or ignore positive buyers on legacy generic receipts. Checks receipt-suppressed rows after applied erasure. |
| TestEtsySimplifiedFakeSync | Bypass receipt or dependent write filtering during fake-server sync, or delete an unrelated retained old receipt. Also requires a new receipt to survive. |
| TestEtsySimplifiedUnsuppressCLI | Accept buyer unsuppression together with receipt selection, lose the space-separated buyer value in NoOptDefVal remapping, or skip explicit empty CLI vacuum. |

## Validation

Build, vet, formatting and diff whitespace checks pass.
Focused synthetic store tests cover simplified retention, prior retention/erasure cases and parent scale.
Selected CLI, MCP denylist and MCP conformance tests pass with the requested credential variables unset.
The full repository gate was deliberately not run: the dispatch ground rules prohibit the long gate.
