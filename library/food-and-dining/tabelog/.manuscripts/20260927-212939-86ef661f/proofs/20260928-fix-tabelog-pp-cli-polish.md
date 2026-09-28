# Phase19 mid-pipeline polish

Status: complete; ship recommendation. The corrected fresh canonical full live acceptance passes 60/60 mandatory cases with 0 failures, and all four actual local mutations and their effects are verified. Publication is not authorized, and neither publish-validation gate is invoked.

The input bundle uses the absolute working directory and run-local Press binary, pinned working/spec.json and run/research.json,5 planned/5 built novel features, no missing rows, no prior sub60 reprint, and no partial override. The same-run parent-held polish lock is the authorized coordination mechanism. It remains held; no library switch or competing print was started.

## Before/after diagnostic evidence

| Gate | Before | After | Evidence |
|---|---:|---:|---|
| Scorecard |82/A |82/A | polish-scorecard-{before,after}.json and .txt |
| Verify |93.75/PASS |93.75/PASS |15/16,0 critical; polish-verify-{before,after}.json |
| Dogfood |raw WARN |raw WARN |5 planned/found;10/10 examples;0 dead flags; two scoped framework warnings below |
| Live-check sample matrix |3 pass,2 mutating skips |3 pass,2 mutating skips | exercised: compare, alternatives, audit |
| SKILL validation |0 findings |0 findings/10 compact recipes | polish-skill-{before,after}.json |
| Workflow validation |workflow-pass |workflow-pass | no separate workflow manifest; actual list lifecycle owned by executable/live acceptance |
| go vet |0 |0 | polish-vet-{before,after}.log |
| gosec relevant handwritten |4 |0 | raw25 to21;21 individual decisions in phase19-security-triage.md |
| Tools audit |1 pending |0 pending,1 accepted | polish-tools-{before,after}.txt and tool ledger |
| PII audit |0 |0 | manuscripts-dir pinned to run; no pending/incomplete findings |
| Publish validate |skipped (mid-pipeline) |skipped (mid-pipeline) | no publishing action or offer |

The independent output reviewer assessed all four checks on three eligible passing source-backed offline samples, returning PASS with no findings. Mutating add/refresh examples were honestly excluded from that sample review; they are exercised by the separate canonical actual mutation acceptance. See polish-output-review.md/json.

## Applied cleanup and preservation

The handwritten cleanup explicitly marks three best-effort closes on already failing read/initialization paths, preserving the original failures and leaving successful Close/commit failure propagation intact. The choices-cache read receives a narrow G304 comment because its fixed private .choices.json path cannot be selected by a restaurant label. The tiny generated-constructor delegate is gofmt formatted only. Durable patch and before/after source hashes are in .printing-press-patches/phase19-polish-cleanup.patch and its provenance JSON, mirrored under proofs.

The builder fixed truthful remove local-write metadata and the late refresh/recipe hint correction. Its focused before/after proof passes 19 events with 0 failures, including actual stdio catalog and tenant ownership assertions. The preserved refresh handler/schema advertises open-world=true; offline comparison is read-only/non-destructive/closed-world; the refresh recipe is non-read-only/non-destructive/open-world. Reserved cliutil/cobratree and generator templates are untouched by polish. Durable metadata patch/provenance: phase19-mcp-hints-product.patch, phase19-mcp-hints-proof.json and phase19-mcp-hints-fix.md, also carried under the working patch directory.

Fresh backups of current README/SKILL/AGENTS and source/list/E2E docs preceded dogfood synchronization. The generated sync prose was inspected and compact root-authored README/SKILL/AGENTS restored exactly after both dogfood invocations. All six preserved docs match phase19-current-docs/snapshot.json; local install instructions, source/budget/error semantics and real command examples are intact.

A bounded local-library divergence scan found the official public-library clone but no valid copy of this CLI. The name-like internal/tabelog subdirectory is not a CLI. Working remains canonical; no public pull/sync happened. See phase19-current-docs/divergence-result.json.

## Accepted scoped/tooling findings

- Five generated compatibility helpers are uncalled in this no-auth HTML CLI. They cover unused binary/auth/query/noop compatibility, do not run, and do not expose extra MCP tools. Removing generator scaffolding only to improve dead-code score is deferred as a template follow-up.
- Generic defaultSyncResources is empty; raw sync is hidden and is not the source population workflow. Actual find/show and lists refresh populate normalized snapshots. The generic raw-store warning is not evidence that saved lists have no population path.
- Generic verify cannot execute the synthetic noncanonical show target. A focused safe proof shows mock-value rejected with usage exit2, while a genuine source URL dry-run succeeds with no home creation or network. Validation is preserved rather than weakened for fake IDs. See polish-show-generic-probe-triage.json.
- The only thin Short is an unregistered conditional platform-client profile list leaf in this no-auth CLI. Actual14-tool stdio inventory omits it; accepted with a concrete note in the tools ledger.
- The21 remaining generated/reserved gosec findings each have path/validation/permission/error-scope rationale. G119 acceptance covers the generated callback's guarded original-host/secure-chain re-stamping, not universal cross-origin/custom-header protection. Native Tabelog uses a separate exact HTTPS English-origin allowlist, with no generated authorization/config headers. See phase19-security-triage.md.
- Score-only suggestions for larger README, bulk raw sync, insight commands, remote MCP transport, learning, browser/service dependencies or extra features are outside approved travel scope. They were not added.

Manual MCP judgment covers all 14 actual tools and required/optional parameters; see phase19-mcp-judgment.md. The false closed-world refresh hint was fixed. A fresh protocol-only capture in polish-runtime-mcp-catalog-final.json verifies the corrected hints and unchanged 14-tool catalog without invoking a tool or issuing a source GET.

The documented global Cobra flag-parser plain-text error limitation remains a framework boundary; it is a deliberate skipped finding, not an unresolved domain/partial-refresh bug.

## Resource evidence

The authoritative existing replay measurement is phase17-cluster-a-measurements/measurement-results.json, not the historical top-level measurement file. Cold find uses 1247 tokens, p95 wall 22.457 ms; warm find p95 is 18.741 ms, with 0 repeat HTTP; max RSS is 28,688,384 bytes. These are independent bounded replay measurements, not a new live-origin latency benchmark. Formatting, explicit close handling and MCP metadata do not change source payloads or find work; no broad rerun was added for score alone.

## Final source and acceptance

The final 144-file Go inventory and both working binary hashes still match phase19-source-freeze.json. Working CLI SHA256: 743183fa161dfa91231f9f1d2b9c3c393358f1c5ae8927ee6b497cb9f1de9b98. Working MCP SHA256: 5ec093ef0e01c051ad8aba995620ec419f1d57918b33f78db4653438a36ac721. Final post-metadata vet is clean; polish-gosec-final.json has the identical 21 accepted tuples and zero new/relevant handwritten findings. Scorecard-final remains 82/A. Source/CLI behavior checks and the independent output review remain applicable because the late delta changes registration hints only.

Corrected canonical marker: phase5-acceptance.json, full PASS 60/60 mandatory, 0 failures, 127 explicit nonmandatory skips/unverified. Marker SHA256: 0cf2b91a7e5db2fca27a515da7cac896775c7f164bb9c1fa69268bab1e30d9f5; source_fingerprint: 2ec6b885c306712fa145566d457030d4ef4816aa6f4b1237cff0e08524a85465. The actual accepted stage CLI SHA256 is 5d14bc325f315491147f6c702274f76f2837629d57066b35132de20006b196b7, distinct from the working build hash above. The runner owns fingerprint/build-stage consistency evidence. The four mutator samples add/note/refresh/remove omit --dry-run, and the runner verifies membership, changed note, refreshed timestamp and consumed removal membership. This supersedes the earlier marker that accidentally previewed remove. No marker was manually synthesized or edited.

Ship is recommended for the parent's local promotion gates; this is not publication approval. There are no unresolved scoped product issues. Dogfood fields below denote the triaged noncritical gate result: original tool JSON truthfully remains WARN for two individually accepted structural warnings. Those WARNs are preserved in the diagnostic summary and skipped_findings; they are not rewritten as raw tool PASS.


---POLISH-RESULT---
scorecard_before: 82
scorecard_after: 82
verify_before: 93.75
verify_after: 93.75
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 4
gosec_after: 0
tools_audit_before: 1 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Explicit best-effort closes preserve existing read/initialization failures; successful persistence failures remain propagated.
- Added a narrow fixed-private-cache-path G304 rationale and formatted the alternatives delegate.
- Corrected refresh and recipe MCP hints with preserved handlers, schemas, local-write classification and tenant ownership.
- Restored current compact authored docs after dogfood sync; saved durable cleanup and metadata patches.
skipped_findings:
- Raw dogfood WARNs are accepted framework noise: unused compatibility helpers and hidden generic raw-sync population advice.
- Synthetic noncanonical show probe remains rejected; genuine canonical dry-run succeeds without I/O.
- Thin Short belongs to an unregistered platform leaf; actual runtime catalog omits it.
- 21 generated/reserved scanner findings are individually accepted with constrained behavior evidence; no universal auth-client safety claim.
- Score-only README/insight/dead-code suggestions do not justify expanding approved scope.
- Global Cobra flag-parser errors may remain plain text, as accurately documented in README; domain/partial-refresh JSON behavior is unchanged.
remaining_issues: []
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: All scoped gates and actual mutation acceptance pass; another polish invocation would repeat accepted framework tradeoffs.
---END-POLISH-RESULT---
