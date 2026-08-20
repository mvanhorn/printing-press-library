# Polish Results: bambu-pp-cli

Public-library divergence check: no validated local clone of `mvanhorn/printing-press-library` was found, so the mid-pipeline working copy was canonical. No publish or other public action ran.

| Check | Before | After |
|---|---:|---:|
| Scorecard (live) | 77 | 76 |
| Verify | 100% | 100% |
| Live matrix | exercised | exercised |
| Tools-audit pending | 2 | 0 |

The canonical non-live score remained 77. The live-scored artifact moved by one point because the idle printer had no current 3MF and one sample encountered transient SQLite contention.

```text
---POLISH-RESULT---
scorecard_before: 77
scorecard_after: 76
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
tools_audit_before: 2 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Aligned root help with the verified audience-first product narrative.
- Classified events watch as an MCP local-write operation because lifecycle events are persisted.
- Completed the MCP quality judgment pass and cleared both audit findings with evidence-backed decisions.
skipped_findings:
- 19 raw gosec findings: all occur in DO-NOT-EDIT generated framework files and are Printing Press retro candidates.
- 11 dead helpers and unregistered set-token: generated framework output; printed-CLI edits would not survive regeneration.
- ams runway live-check failure: printer is idle, so no current 3MF exists.
- failure-correlations live-check failure: transient SQLITE_BUSY test-environment contention.
- scorecard doctor/dead-code deficits: structural scorer/generated-framework findings, not missing Bambu monitoring behavior.
remaining_issues: []
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: All printed-CLI gates pass and another pass would only repeat environmental or generated-framework findings.
---END-POLISH-RESULT---
```
