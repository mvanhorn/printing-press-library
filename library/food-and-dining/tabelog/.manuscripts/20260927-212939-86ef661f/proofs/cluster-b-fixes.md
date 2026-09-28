# Phase 17 cluster B fixes

Fixed M1, M2, S2, and S3 within the approved cluster. Focused product tests, fresh-generation behavioral regressions, preserved binary streaming tests, and a private verification CLI build pass. No origin requests, phase receipts, shared Press rebuild, or broad product suite were run.

- **M1:** Notebook initialization creates an idempotent index on membership `restaurant_id`. The existing protection/eviction transactions are unchanged. A private fixture with 10,000 snapshots and 9,000 saved members was initialized/upgraded through actual CLI binaries. Before and after, protected/unsaved counts and payload totals were identical: 9,000 saved records/2,376,000 bytes and 1,000 unsaved records/264,000 bytes. The protection query changed from a covering membership scan to an indexed `restaurant_id` seek; three-run median decreased from 1,172.8 ms to 2.0 ms in this fixture. These are fixture query timings, not a claim about every real command.
- **M2:** Budget comparison uses raw text, numeric bounds, and evidence state separately from nested source provenance. A source-only `listing`→`listed` transition no longer becomes a lunch-price change. The current complete snapshot still retains `source: listed`, the detail surface, and notes. An actual root-command refresh against captured public listing/detail fixtures proves the unchanged unknown lunch budget yields no delta while new detail evidence remains present.
- **S2:** Generated ordinary request responses use the existing 32 MiB limited reader; this covers plain, identity, transparent gzip, and error bodies before unbounded allocation. Explicit binary responses keep their previous read/stream behavior. Native Tabelog source requests retain their independent decoded 4 MiB limit. Matching request-transport template changes are recorded; optional OAuth-specific branches are not emitted by this AuthNone CLI and were not part of the exercised source path.
- **S3:** Delivery exclusively creates a randomized file in the destination directory, writes through its returned file descriptor, closes/checks errors, renames, and removes residual temporary files. The predictable `.tmp` symlink remains untouched, its unrelated sentinel retains its contents, and sixteen concurrent writers yield complete atomic outputs without temporary-file leftovers. Matching template changes are recorded.

Owned product files:

```text
internal/notebook/notebook.go
internal/cli/tabelog_lists_refresh.go
internal/client/client.go
internal/cli/deliver.go
internal/client/response_size_test.go
internal/cli/deliver_atomic_test.go
internal/cli/refresh_budget_facts_test.go
```

Owned generator files (run-local v4.32.5 source):

```text
internal/generator/templates/client.go.tmpl
internal/generator/templates/deliver.go.tmpl
internal/generator/response_delivery_security_test.go
```

Durability records are in the working CLI's `.printing-press-patches/phase17-cluster-b-product.patch`, `phase17-cluster-b-generator.patch`, and `phase17-cluster-b-provenance.json`. The provenance record includes the upstream tag/commit, owned before/after file hashes, restoration instructions, and proof paths. The generator owner received the template-ready signal for the cumulative toolchain build; this agent did not rebuild that shared binary.

Verification evidence:

- [cluster-b-red.log](cluster-b-red.log): original nonbinary/body and temporary-file defects; explicit binary payload already passed.
- [cluster-b-m2-red.log](cluster-b-m2-red.log): provenance-only lunch-budget delta observed through the actual root command before the fix.
- [cluster-b-green.log](cluster-b-green.log): nine focused product cases pass, including transaction/retention behavior, four oversized nonbinary variants, a binary payload exceeding the nonbinary cap, private file modes, sentinel/concurrency, and fixture-based refresh.
- [cluster-b-generator-green.log](cluster-b-generator-green.log): `TestGeneratedResponseSizeAndExclusiveDelivery` and `TestGeneratedClientStreamingTimeoutCarveOut` pass against disposable freshly emitted clients.
- [cluster-b-retention-before.json](cluster-b-retention-before.json), [cluster-b-retention-after.json](cluster-b-retention-after.json): unchanged saved/unsaved totals, actual query plans, and measured times.
- [cluster-b-retention-check.py](cluster-b-retention-check.py): repeatable private-fixture setup/check. Run its `before` stage on a fresh fixture home with the pre-fix binary, then `after` with the current binary; it does not use the source origin.

The private verification CLI is `proofs/cluster-b-verification-cli`. No changes were made to source parsing/discovery/helpers or MCP/intents; those remain owned by the other fix clusters.
