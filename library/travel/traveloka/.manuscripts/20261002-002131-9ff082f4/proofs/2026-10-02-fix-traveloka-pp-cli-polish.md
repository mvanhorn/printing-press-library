# Traveloka CLI polish — 2026-10-02

Mid-pipeline polish completed against this run's working CLI. Recommendation: **ship** within the approved local research scope. No publication, GitHub write, global installation, promotion, or receipt update was performed by polish.

| Check | Before | After |
|---|---:|---:|
| Scorecard | 79/100 | 79/100 |
| Verify | 30/31 (96.77%) | 31/31 (100%) |
| Full live dogfood | PASS 156/156 | PASS 156/156 |
| Go vet issues | 0 | 0 |
| Gosec unresolved Traveloka-specific findings | 12 before review | 0 |
| Tools-audit pending | 4 | 0 (2 accepted generated Shorts) |
| PII pending | 0 | 0 |
| Verify-skill errors | 1 | 0 |
| Workflow verifier | failed without session/shared state | both workflows pass, all 6 steps |

The final full matrix has 156 evaluated checks, 0 failures, and 86 honestly retained skips. All five approved novel commands have passing happy-path and JSON checks. The latest full report is `19-polish-after/dogfood-full.stdout`; the actual native-written marker is `phase5-acceptance.json`. The native source fingerprint audit is `19-polish-final-fingerprint-audit.json`, with 219 source files and digest `ee7bed075277eac02d4f0d0c21e0f894104ff95970db43b1d332ea03bda6e7b1`.

The scorecard live-check exercised three local comparisons over **actual retrieved** snapshots and skipped the two live grids because its default does not allow local writes. The separate full owned matrix exercised both grids against real source requests. The independent output reviewer assessed all three eligible samples and returned PASS with no warnings; see `19-polish-output-review.md`. A new real-output history check under the same session-derived store namespace compared two distinct current retrievals and correctly found zero changes; see `19-polish-real-output-history-check.json`.

Polish fixes:

- Added eight source-grounded typed MCP descriptions through `mcp-descriptions.json` and applied them with `mcp-sync`. The final runtime exposes 34 tools; `19-polish-mcp-tool-judgment.json` records each description and classification. Source searches that save public snapshots retain read-only=false/local-write=true metadata. Typed raw source reads remain read-only.
- Made the flight annotation and hotel constructor metadata visible to static checks, preserving the existing runtime classifications and mode-specific hotel description.
- Changed the README's advanced airport example to its directly declared `--data` flag after checking actual help. Corrected the unsupported exclusivity sentence in README/SKILL to “These five commands compare explicit dates or retrieved Traveloka offers.” The generator can reintroduce its boilerplate sentence, so this correction was applied after its final sync.
- Used one explicit, absolute, confined fixture home across workflow steps. The CLI intentionally requires an absolute `--home`; this is verification setup, with no user-visible runtime-default change. Both real workflows now pass.
- Preserved earlier cleanup errors explicitly and added narrow, source-explained gosec suppressions for bounded operator-selected files, browser-cookie client replay, the validated argv-only browser backend, and owner-only directories. No source transport guard or access boundary was relaxed.

Validation:

`go test ./...` passed across all packages after the source cleanup. After the MCP description refresh, `go test ./internal/mcp/...` and both delivered CLI/MCP builds passed. Go vet passed. The pinned scoped `go run github.com/securego/gosec/v2/cmd/gosec@v2.26.1` scan completed on the final tree without installing globally. Its raw count is 88 → 76; the remaining 74 generated-file findings plus two framework findings are explicitly triaged in `19-polish-gosec-triage.json`, not claimed as a clean raw scan. There are zero unresolved findings in Traveloka-specific hand-authored code.

`tools-audit` reports no pending findings, with two individually accepted generated list Shorts; no incomplete ledger gates remain. `pii-audit --manuscripts-dir <run>` reports no findings under its stated shape-detector scope. The final exact-known-value privacy scan checked 703 files against 746 captured private values and found zero leaks (`19-polish-private-value-scan.json`). Raw capture/session exports and stdout remain only in private temporary locations; durable capture evidence contains validated booleans and operation count (`19-polish-capture-projection.json`).

Observed retries and their evidence:

- The first refreshed capture succeeded and all flight/hotel source checks passed, but the matrix was 150/156 because polish incorrectly set `TRAVELOKA_NO_LEARN=true`, suppressing six learning preview outputs. That setting was removed for the full runner; the failed report/actual marker were preserved.
- A normal capture then found no selectable outbound offer. An attempted route/date override failed local flag validation because this delivered auth capture exposes only session and shopper-context flags. No source request was made by that invalid invocation.
- Revalidating the older session through the airport doctor succeeded, but its reused flight/hotel requests returned HTTP 202. The 140/156 report and failed marker remain as stale-session evidence; airport validation alone was not treated as flight/hotel access proof.
- A fresh normal guest capture using the supported USD bootstrap context succeeded with eight profiles, HTTP verification, and its own browser closed. Public raw fixture currency fields are SGD and match the matrix's SG/en-SG/SGD shopper request context; no conversion was inferred. Fixture preparation and the full runner followed immediately, yielding 156/156. The successful session-aware verify and real workflow runs followed.
- The initial shared workflow fixture path was relative, which the CLI correctly rejected. The corrected absolute-path workflow report passes all six steps. This YAML setup correction did not alter the native CLI source fingerprint; the final audit matches the full marker.

Retained non-blocking/structural findings:

- Five unused generated helpers are template output; deleting them locally would not be a durable fix. The static config-field parser mistakes string fragments for fields, and its workflow-name matcher treats commands with flags as unmapped. The actual runtime workflow report passes all steps.
- Generic sync/data-pipeline deductions do not describe this bounded read-only source research workflow, which saves normalized public retrieval snapshots rather than performing provider mutations or exhaustive synchronization.
- Several MCP scorecard dimensions are unscored for the promoted runtime surface. `tools-manifest.json` intentionally has no static endpoint entries in this layout; runtime `tools/list` has 34 reviewed tools. No fake descriptors, commands, or score-only scaffolding were added.
- The 76 remaining framework/generated gosec findings are Printing Press follow-up candidates, including generated browser/TLS/cookie/path/subprocess handling, framework SQLite migration string construction, and an owner-executable directory in reserved test isolation. They were not silently suppressed or counted as zero raw findings.
- Full-matrix skips include non-ID/absent positional fixtures, missing list companions, preview-only local mutations, and negative probes that would otherwise mutate live state. Auth is an infrastructure subtree excluded from that matrix; real delivered capture/import evidence is separate. No skipped preview was called live coverage.

Material limits remain accurately documented: guest sessions can require refresh when individual source operations return protection responses; flight inventory is bounded and may still be polling-incomplete; prices are retrieved indicative quotes, not booking guarantees; missing policy, inclusion, timezone, or occupancy data remains unknown. Real hotel flexibility output has zero exact comparable opposite-policy pairs; only the separately labelled simulated tests cover the paired branch. Existing phase-18 representative source-price cross-checks remain unchanged and support the unchanged monetary/party normalization. This polish adds no booking, payment, or account operation.

Publish validation and any promotion/archive receipts are owned by the parent pipeline. Publish validation is skipped informationally here because this is mid-pipeline and the user prohibits publication.

---POLISH-RESULT---
scorecard_before: 79
scorecard_after: 79
verify_before: 96.77
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 12
gosec_after: 0
tools_audit_before: 4 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Added eight grounded MCP description overrides, regenerated their registration, and reviewed all 34 runtime tools.
- Exposed truthful flight/hotel metadata to static checks while preserving local-write classifications.
- Corrected the advanced airport example and unsupported README/SKILL exclusivity wording.
- Made workflow checks share one absolute confined fixture home and verified both real workflows.
- Clarified cleanup error handling and narrowly documented proven security-scan false positives.
skipped_findings:
- Two generated thin list Shorts accepted individually; richer optional-field guidance belongs in generator templates.
- Seventy-four generated plus two framework gosec findings triaged as Printing Press follow-ups; raw after count remains 76.
- Generated unused helpers, static parser/config false positives, and generic sync deductions do not justify local template edits.
- Runtime-promoted MCP scorecard dimensions remain unscored; all 34 actual runtime tools were inspected.
- Scorecard's two default-skipped grids are covered by the separate real full matrix; 86 full-matrix fixture/preview/negative skips remain disclosed.
- Expired operation sessions and failed normal capture evidence were retained; fresh approved capture and full replay resolved the access gate.
remaining_issues: []
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: The CLI-specific checks are clear; another polish pass would repeat completed work or revisit generator and session-lifecycle constraints.
---END-POLISH-RESULT---
