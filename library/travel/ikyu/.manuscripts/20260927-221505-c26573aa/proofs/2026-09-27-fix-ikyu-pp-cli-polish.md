# Polish results: ikyu-pp-cli

Applied printing-press-polish directly under the existing run. Root retains architecture/review/acceptance ownership per user instruction; the captured binary and this run’s own phase=polish lock were retained. No competing build or shared configuration edit. Gate bundle: 5 planned, 5 built, none missing; first print; no partial override.

Verify 100% → 100%; score 80/100 → 80/100; live samples exercised; tools-audit 0 pending; PII audit 0 pending; vet clean; owned-source gosec 0 unresolved. All required verification legs passed together in polish-shipcheck.json.

Documentation was shortened using Writing for Agents: source-backed descriptions retained, repeated generated examples/claims compacted, local use clarified, validation/measurement evidence linked and upstream raw-HTML access limitation disclosed. Canonical installation text remains explicitly conditional on a future public release; no installation/publication occurred. No Go source edit was made during polish. Final source/acceptance binding is delegated to Printing Press’s canonical promote validator (its hashes normalize Go/module metadata; raw byte hashes are not equivalent).

Manual tool review confirmed seven distinct, parameter-aware accommodation commands with read-only hints, typed outputs and bounded behavior. Framework profile/config writes remain separately classified; no reservation tool exists. Latest eligible output review assessed all five live samples and additional complete comparison/date projections without findings.

Generated gosec findings (20 raw) are retained with per-rule rationale in gosec-template-triage.json. Framework dead-helper/static-workflow warnings and two denied raw HTML probes are documented, not disguised as tested features. No unresolved defect affects the approved seven-command surface.

---POLISH-RESULT---
scorecard_before: 80
scorecard_after: 80
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 0
gosec_after: 0
tools_audit_before: 0
tools_audit_after: 0
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Concise local documentation and evidence report; no source edits.
skipped_findings:
- Generated static-analysis/template notes; reviewed individually outside the owned source adapter.
- Generated raw properties helper HTTP403; source-side access limitation outside the supported stay workflow.
remaining_issues: []
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: All supported stay workflows pass; remaining notes concern generator infrastructure or upstream denial on the raw scaffold.
---END-POLISH-RESULT---
