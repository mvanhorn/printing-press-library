# Phase 17 cluster C durable repairs

S1 is fixed in both the generator and printed MCP handlers. Static and named flags precede a literal `--` boundary; list and ID values follow it. Required empty/whitespace guards remain. A real stdio call with ID `--dry-run` now rejects without network or files, while an offline comparison of a list literally named `--dry-run` returns its actual saved record.

T1 is fixed in both generated and printed helpers. A wholly invalid selection returns one actionable error, including valid fields, before prose warnings. Successful partial selection keeps its original warning. The existing client dry-run sentinel still downgrades the error at its real seam and emits one warning there.

Failing real-seam evidence: `proofs/phase17-cluster-c-before-tests.log`. Passing product and emitted-module evidence: `phase17-cluster-c-focused-tests.log`, `phase17-cluster-c-valid-stdio-tests.log`, and `phase17-cluster-c-generated-tests.log`. Freshly generated code compiled and executed; no printed-only hotfix.

Separate upstream patches: `mcp-recipe-boundary.patch` and `projection-diagnostic.patch`; separate printed patches in `proofs`. `phase17-cumulative-toolchain.patch` includes the prior three repairs plus cluster B's bounded response and exclusive delivery templates. The JSON proof records all changed-source hashes, patch hashes, upstream provenance, and the final binary hash.

Only run-local source and binary changed. Source and caches remain outside the run research directory. The global binary and module source are untouched. Root limited this task to focused and fresh-generation checks; the integrated product suite belongs to the builder. Previously documented broad Press environment failures remain; no golden/publication-ready claim is made.
