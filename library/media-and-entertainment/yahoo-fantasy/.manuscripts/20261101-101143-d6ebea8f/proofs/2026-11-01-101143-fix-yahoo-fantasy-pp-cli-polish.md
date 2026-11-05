# Polish Report: yahoo-fantasy-pp-cli

Public library not found locally; proceeding on internal as canonical.

```
Polish Results for yahoo-fantasy-pp-cli:

                    Before    After     Delta
  Scorecard:        92/100    92/100    +0
  Verify:           100%      100%      +0%
  Live matrix:      not_exercised -> not_exercised
  Tools-audit:      0         0         0 pending findings
  Gosec (hand):     1         0         -1 finding

Fixes applied:
  - Added #nosec G101 rationale on public Yahoo OAuth2 token URL constant in internal/cli/yahoo_transport.go
  - Updated waiver-wire recipe --select field paths in research.json to match WaiverUpgrade JSON tags (add_player_name, percent_owned, net_upgrade_ppg)

Skipped findings:
  - 4 dead helper functions in internal/cli/root.go (hasChangedLocalFlags, requiredInputWaived, retainCLIQueryParams, retainExplicitQueryParams): generator-emitted template helpers in DO NOT EDIT file; routed as retro candidate

Remaining issues:
  - None

---POLISH-RESULT---
scorecard_before: 92
scorecard_after: 92
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: not_exercised
dogfood_live_matrix_after: not_exercised
govet_before: 0
govet_after: 0
gosec_before: 1
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Added #nosec G101 rationale on public Yahoo OAuth2 token URL constant in internal/cli/yahoo_transport.go
- Updated waiver-wire recipe --select field paths in research.json to match WaiverUpgrade JSON tags
skipped_findings:
- internal/cli/root.go dead helper functions: generator-emitted DO NOT EDIT template helpers
remaining_issues: []
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: Verify is 100% (64/64), scorecard is 92/100 (Grade A), tools-audit and pii-audit have 0 pending findings, and gosec has 0 findings in hand-authored code.
---END-POLISH-RESULT---
```
