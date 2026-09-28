# Security re-review — round 2

**PASS — no remaining or new in-scope security findings.** Reviewed the frozen current source and the actual cluster A/B/C product diffs; this was not limited to the prior notebook-owned fixes. No product edits, reproducer reruns, broad suites, receipts, or source-origin requests were made during this round.

## Prior security findings

- **S1 closed:** `internal/mcp/intents.go:58–70` and `:87–104` collect recipe positionals separately, place static/generated flags first, insert `--`, then append the literal list/ID values. Required empty values still fail before child execution. A flag-shaped ID now reaches numeric-ID validation instead of selecting `--dry-run`, an output sink, or another global option. Current executable stdio proof covers rejected `--dry-run`/`--json` IDs and empty values, plus a successful comparison of a real saved list whose name begins with `--`. No shell evaluation was introduced.
- **S2 closed for the approved emitted scope:** `internal/client/client.go:1330–1339` limits every nonbinary ordinary HTTP response before further processing, including identity and transparently decompressed bodies and error responses. The existing 32 MiB reader and manually decoded size guard are used. The sole `io.ReadAll(resp.Body)` in the emitted client is inside the explicit binary branch. The binary timeout carve-out at `:1303–1305` remains intact. Native source transport retains its independent decoded 4 MiB limit. Error paths close the HTTP body and return a size error rather than saving an oversized response.
- **S3 closed:** `internal/cli/deliver.go:147–162` creates a randomized exclusive file in the selected directory, writes using the returned descriptor, checks write/close/rename failures, and removes residual temporary files. It does not reopen or write the predictable `.tmp` path or follow the final target symlink while writing. Single-segment filename validation remains in place. The existing sentinel, concurrent-reader/writer, private-mode, and fresh-emission proofs cover the concrete prior attacks.

## Other clusters and regression boundaries

- **Source identity:** `internal/source/parser.go:352–407` resolves independent canonical/schema identity through the strict restaurant URL validator and rejects wrong venues, conflicting identities, missing independent routes, and the same ID under a different canonical route. Fragment-only schema node identifiers do not establish a venue by themselves. `ParseDetail` invokes validation at `:445` before the fetch path can cache HTML or replace a snapshot. No new URL fetch, permissive host, or redirect exception was added.
- **Source sort:** `internal/source/discovery.go:443–465` validates the single active highest-rated tab and conflicting next-page sort values. This runs while parsing each listing before its raw response is cached; empty results do not invent a ranking. Existing geography/category/budget/keyword validation, pagination limits, body limits, and context propagation remain intact.
- **Storage/index:** `internal/notebook/notebook.go:44` adds constant idempotent DDL within the existing initialization transaction. It introduces no user-controlled SQL, changed protection predicate, or note/membership overwrite. Before/after private-fixture totals and indexed-query evidence agree.
- **Budget deltas:** `internal/cli/tabelog_lists_refresh.go:41–81` compares budget facts separately from provenance using new maps. It retains evidence-state comparison and complete before/current source objects for genuine changes. It does not mutate persisted snapshots or user notes, weaken identity validation, or change refresh concurrency/deadlines.
- **Checkpoint/render/projection:** Source `find/show` no longer call the obsolete non-context-bound raw-store checkpoint after normalized persistence; storage failures still propagate. The source summary now includes facilities and the select-help example uses actual domain fields. Projection's all-miss branch returns its structured error before emitting partial-match warnings, so the machine wrapper receives one diagnostic. The changes add no code evaluation or new I/O path.

## Evidence inspected

- Cluster A product patch/provenance in `.printing-press-patches/phase17-cluster-a-*`, including independent identity/sort before/after oracle and executable cases.
- Cluster B product/generator patches and hashes in `.printing-press-patches/phase17-cluster-b-*`; [cluster-b-fixes.md](cluster-b-fixes.md), focused product/fresh-generation logs, and saved/unsaved query-plan evidence.
- [phase17-mcp-recipe-product.patch](phase17-mcp-recipe-product.patch), [phase17-projection-product.patch](phase17-projection-product.patch), [cluster C focused proof](phase17-cluster-c-focused-tests.log), [successful literal-positional stdio proof](phase17-cluster-c-valid-stdio-tests.log), and [fresh-generation proof](phase17-cluster-c-generated-tests.log).
- The build owner's integrated [phase17-integrated-results.jsonl](phase17-integrated-results.jsonl) was inspected for context rather than rerun. Code and boundary review remain the basis of this verdict.

## Documented scope limits

- Optional OAuth-specific response readers in the generic generator template are not emitted by this no-auth Tabelog CLI and remain unchanged. This review does not claim a universal auth-client repair; no account/token work was performed.
- The known generator root/Cobra limitation for human-prose flag-parse errors before `RunE` remains a separate template retro item, excluded from product-round convergence by the parent. Successful partial projections intentionally retain warnings. This is not represented as a universal machine-diagnostic fix or a new security finding.
- Generator-reserved `internal/cliutil` and `internal/mcp/cobratree` implementations were not reviewed or edited. No new out-of-scope security candidate was observed at their inspected call boundaries.

Security convergence: prior S1/S2/S3 findings are closed within the agreed shipping scope; remaining in-scope security findings: **0**.
