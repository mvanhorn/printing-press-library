# Bland CLI Polish Results

## Diagnostics

| Check | Result |
|---|---:|
| Verify | 39/39 (100%) |
| Scorecard baseline | 92/100, grade A |
| Scorecard after | 91/100, grade A |
| Workflow verify | workflow-pass; no workflow manifest found, so no configured workflow was executed |
| Verify-skill | 0 findings; 33 recipes checked |
| Tools audit | 0 pending findings |
| PII audit | 0 findings, including manuscripts |
| Go build and vet | pass |
| Gosec | 36 generator-emitted findings; 0 in novel-feature files |
| Phase 5 live matrix | 101/101 executed checks passed; 136 skipped/unverified; coverage hollow for both novel features |
| Scorecard live sample | 0 evaluated, 2 skipped (task run mutates; local recall store unsynced) |

The promotion marker is a full-matrix `pass`, but it records `coverage_hollow: true` for `calls task run` and `calls search-task`. Separately, the live sampler's automatic binary refresh regenerated Go files after that marker was written. The refreshed tree therefore does not match the acceptance marker's source fingerprint. No marker was hand-edited to hide either condition, and no outbound call or call-stop action was run.

No validated local clone of `mvanhorn/printing-press-library` was found. The internal library has no Bland entry yet.

## Fixes carried from the review and full matrix

- Task call polling returns permanent API errors and reports final outcomes in human output.
- Replaced a real response identifier in a generated help example with a synthetic UUID; privacy scans found no original test call ID or phone in the run tree.
- Mapped Bland's `error_message` response field correctly.
- `active` now filters the supported calls endpoint (`completed=false`) after the documented `/v1/active` route returned 404.
- Added missing help examples and fixed jobs dry-run handling.
- Corrected README/SKILL event-stream syntax and removed an unsupported uniqueness claim.

## Skipped findings

- 36 gosec findings are in generator-emitted framework files (including G304, G201, G302, G104, and G202); none occur in the two hand-authored task feature files. These are Printing Press generator retro candidates and were not patched in generated files.
- `internal/cli/helpers.go` has one generated dead helper (`handleBinaryResponseDelivery`); no generated-code deletion was made.
- Output plausibility could not be rated because no novel-feature sample was evaluated.
- The scorecard is 91/100 after a fresh run; its live matrix remains unexercised.

## Promotion status

**HOLD.** The workflow requires live feature coverage and an unchanged Phase 5 source fingerprint before promotion. This run satisfies neither because task-run dogfood was deliberately read-only and because the scorecard refresh rebuilt the source. The working copy remains at `working/bland-pp-cli`; the machine-readable phase receipt marks the polish phase blocked.

---POLISH-RESULT---
scorecard_before: 92
scorecard_after: 91
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: not_exercised
dogfood_live_matrix_after: not_exercised
govet_before: 0
govet_after: 0
gosec_before: 0
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Fixed task outcome polling, human output, Bland error field decoding, active-list route, missing help examples, and jobs dry-run behavior before polish.
- Corrected README and SKILL event-stream syntax and removed an unsupported uniqueness claim.
- Re-ran the full read-only dogfood matrix after the scorecard refresh; 101 of 101 executed checks passed.
skipped_findings:
- 36 generator-emitted gosec findings retained as Printing Press retro candidates; no findings in hand-authored task commands.
- One dead helper in generated internal/cli/helpers.go retained.
- Live output plausibility not reviewed: scorecard evaluated zero samples, skipped two.
remaining_issues:
- Phase 5 acceptance is hollow for calls task run and calls search-task; the live sampler evaluated neither.
- Phase 5 source fingerprint no longer matches after the scorecard live-check regenerated Go source files.
- No validated public-library checkout was available for divergence review.
ship_recommendation: hold
further_polish_recommended: no
further_polish_reasoning: The hold is caused by required live-feature authorization/fixture coverage and acceptance-marker source drift, neither of which a repeat polish pass can safely resolve.
---END-POLISH-RESULT---
