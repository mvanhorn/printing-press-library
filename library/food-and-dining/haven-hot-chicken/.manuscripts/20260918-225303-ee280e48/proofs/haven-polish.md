# Haven Hot Chicken polish

Mid-pipeline same-model skill fork. Public divergence checked: local clone has mvanhorn/printing-press-library upstream and no Haven CLI; the current print is canonical. Parent released its same-run lock. Printing Press4.32.2 meets minimum4.0.0. Host Go1.26.5 currency advisory is resolved through project auto-toolchain1.26.8. Free disk24.6GB.

## Diagnostic results

Scorecard92/100 before and after; verify33/33 (100%) before and after. Prior parent score91 is not counted as a polish improvement. Verify-skill reports no errors. Workflow verifier reports workflow-pass with no workflow manifest, so no separate workflow execution is claimed. Vet emitted no diagnostics. Live-check evaluated5 samples and passed5/5. Strict PII audit reports no findings. Tools audit ends with no pending findings (two individually justified generated thin-Short accepts).

Dogfood before/after exits0 with literal verdict WARN for five generated helper candidates. Separate full-live parent acceptance records106passed,0failed,83skipped. Gosec2.26.1 completes with29 raw issues both times; zero are in handwritten Haven code. All29 remain visible in the raw JSON and inventory below. This is generated-framework triage, not a claim that the scanner is clean or every signal is a false positive. Parent owns final acceptance freshness and promotion. Public validation is skipped in mid-pipeline mode.

## Changes

- Removed unsupported exclusivity boilerplate restored by dogfood from README/SKILL after final sync.
- Added CLI to the README title.
- Recorded individual source-grounded accepts for DO-NOT-EDIT platform_client.go list and teach.go list Shorts. Binary confirms no pending findings; no finding was manually marked fixed.
- Updated the existing patch record with haven_endpoint_examples.go and both new regression files.
- No Go implementation changes; parent confirms acceptance source hashes unchanged.

## Required judgment passes and triage

Read complete command descriptions/classifications and six typed MCP endpoint descriptions against actual GET contracts. Haven refresh is local-write; saved reads are read-only; endpoint tools identify required location/menu/item IDs and public routing defaults. Earlier verified recipes and help/source checks remain applicable. Three recipe examples plus five capability examples cover all five unique tools; this read-only scope has no order/payment mutation recipe.

Five generated helper candidates: handleBinaryResponseDelivery, readSecretFromStdin, retainCLIQueryParams, retainExplicitQueryParams, successfulNoop in internal/cli/helpers.go. retainExplicitQueryParams is referenced by its sibling helper. Preserve generator-owned transport/auth/pagination helpers for upstream review rather than locally deleting shared templates.

Tools accepts are individually justified: platform_client.go:517 List client profiles accurately names its grouped platform operation; teach.go:847 List recorded learnings accurately names taught local query mappings. Both carry DO-NOT-EDIT headers. Richer defaults belong upstream.

Gosec findings are retained as generator/framework retro candidates. Explicit reviewed examples: G119 client.go:337 re-stamps Authorization only after same-origin guard; G101 platform/gate.go:22 is the invalid_credentials enum; G202 platform/migration.go:156 doubles single quotes for VACUUM INTO (framework source lacks generator header). Reserved cliutil/testenv remains untouched. Other raw SQL, path, permissions, secret-pattern and unchecked-error signals are listed below for upstream triage. No external issue was sent. Existing immutable-WAL read and unbounded HTTP-body candidates remain in phase-4.95-findings.md.

Stdio-only MCP and public/no-auth scope are intentional. No scoring scaffolding, new features or external providers added. Output-review fork /root/skill_review/polish_output assessed5 passing features plus populated samples and returned PASS; see polish-output-review.md.

Evidence files: polish-dogfood-before/after.json (diagnostic preamble then JSON); polish-verify-before/after.json; polish-workflow-verify-before/after.json; polish-verify-skill-before/after.json; polish-scorecard-before/after.json; polish-gosec-before/after.json and logs; empty polish-vet-before/after.log; final polish-tools-after.log and polish-pii-after.log.

Automatic approval review rejected an earlier report draft because its skill-specified post-triage gosec count0 could obscure29 raw issues. This report explicitly preserves29 raw findings and separates handwritten count0. No scan or ledger was altered to change results.

---POLISH-RESULT---
scorecard_before: 92
scorecard_after: 92
verify_before: 100
verify_after: 100
dogfood_before: WARN
dogfood_after: WARN
full_live_acceptance: PASS (106 passed, 0 failed, 83 skipped)
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 29
gosec_after: 29
gosec_handwritten_haven_before: 0
gosec_handwritten_haven_after: 0
gosec_generated_framework_triage: 29 raw findings preserved for upstream review
tools_audit_before: 2 pending
tools_audit_after: 0 pending (2 accepted)
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- README title, unsupported exclusivity prose, justified tools accepts, existing patch metadata.
skipped_findings:
- Five generated-helper warnings and29 raw generated-framework security signals preserved for upstream triage.
- Previously recorded immutable-WAL and uncapped-response-body candidates remain documented.
- No workflow manifest; no separate workflow execution claimed.
remaining_handwritten_issues: []
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: Required local gates pass; remaining scanner candidates belong to generated framework code, and another local polish pass would repeat these checks.
---END-POLISH-RESULT---
## Raw security scanner inventory

- G119 internal/client/client.go:337 - Sensitive headers should not be re-added in redirect policy callbacks
- G101 internal/platform/gate.go:22 - Potential hardcoded credentials
- G703 internal/cli/teach.go:210 - Path traversal via taint analysis
- G201 internal/store/store.go:3970-3973 - SQL string formatting
- G201 internal/store/store.go:3667-3670 - SQL string formatting
- G201 internal/store/store.go:1315 - SQL string formatting
- G201 internal/store/store.go:1291 - SQL string formatting
- G202 internal/store/learnings.go:609 - SQL string concatenation
- G202 internal/platform/migration.go:156 - SQL string concatenation
- G304 internal/store/store.go:253 - Potential file inclusion via variable
- G304 internal/platform/ratelimit.go:148 - Potential file inclusion via variable
- G304 internal/platform/profile.go:510 - Potential file inclusion via variable
- G304 internal/learn/teach_log.go:112 - Potential file inclusion via variable
- G304 internal/learn/playbooks.go:73 - Potential file inclusion via variable
- G304 internal/config/config.go:105 - Potential file inclusion via variable
- G304 internal/cli/teach_playbook.go:374 - Potential file inclusion via variable
- G304 internal/cli/teach.go:150 - Potential file inclusion via variable
- G304 internal/cli/teach.go:127 - Potential file inclusion via variable
- G304 internal/cli/feedback.go:201 - Potential file inclusion via variable
- G304 internal/cli/feedback.go:66 - Potential file inclusion via variable
- G304 internal/cli/export.go:78 - Potential file inclusion via variable
- G117 internal/learn/journal.go:265 - Marshaled struct field "SessionKey" (JSON key "session_key") matches secret pattern
- G302 internal/platform/profile.go:302 - Expect file permissions to be 0600 or less
- G302 internal/cliutil/testenv/testenv.go:76 - Expect file permissions to be 0600 or less
- G302 internal/client/client.go:680 - Expect file permissions to be 0600 or less
- G104 internal/store/store.go:3758 - Errors unhandled
- G104 internal/store/store.go:3501 - Errors unhandled
- G104 internal/learn/journal.go:278 - Errors unhandled
- G104 internal/client/client.go:1348 - Errors unhandled
