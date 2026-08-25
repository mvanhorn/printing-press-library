# Polish Report: openrouter-image-pp-cli

## Polish Results

| Metric | Before | After | Delta |
|---|---|---|---|
| Scorecard | 99/100 | 99/100 | 0 |
| Verify | 100% (48/48) | 100% (48/48) | 0 |
| Live matrix | not_exercised (no API key) | not_exercised | 0 |
| Tools-audit | 1 pending | 1 pending (accepted) | 0 |
| gosec (hand-authored) | 6 | 0 | -6 |
| go vet | 0 | 0 | 0 |
| pii-audit | clean | clean | 0 |
| Shipcheck | 7/7 PASS | 7/7 PASS | 0 |

## Fixes applied
- Added `#nosec` suppressions with durable reasons for 6 benign gosec findings in hand-authored novel files:
  - G304 (file inclusion): user-named reference-image and CSV paths are explicit CLI inputs, not attacker-controlled
  - G306/G301 (file permissions): output images must be world-readable (0o644) for the user's tools; output dirs use standard 0o755

## Skipped findings
- tools-audit thin-short on `learnings list` (teach.go:692): generator-emitted framework file ("DO NOT EDIT"); Short "List recorded learnings" is accurate and brief. Retro candidate, not polish-owned.
- 25 remaining gosec findings: all in generator-emitted files (client.go, store.go, auth.go, learn/, cliutil/, config.go). Printer retro candidates; fixing them by hand-editing generated files would be wiped on regen.

## Remaining issues
- None. Live matrix not exercised (bearer auth, no OPENROUTER_API_KEY); verification ran against mocks. Catalog sync + models rank + cost-estimate additionally verified against the real public catalog outside the skip.

---POLISH-RESULT---
scorecard_before: 99
scorecard_after: 99
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: not_exercised
dogfood_live_matrix_after: not_exercised
govet_before: 0
govet_after: 0
gosec_before: 6
gosec_after: 0
tools_audit_before: 1
tools_audit_after: 1
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Added #nosec suppressions (G304/G306/G301) to generate.go, batch.go, regenerate.go for user-named file paths and world-readable image outputs
skipped_findings:
- tools-audit thin-short on learnings list: generator-emitted framework file, accurate Short, retro candidate
- 25 gosec findings in generator-emitted files: printer retro candidates, not polish-owned
remaining_issues:
- live matrix not exercised (no OPENROUTER_API_KEY; mock verification only)
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: Scorecard 99/100, verify 100%, zero gosec findings in hand-authored code, all 7 shipcheck legs green; remaining items are environmental (no live key) or generator retro candidates that polish must not touch.
---END-POLISH-RESULT---
