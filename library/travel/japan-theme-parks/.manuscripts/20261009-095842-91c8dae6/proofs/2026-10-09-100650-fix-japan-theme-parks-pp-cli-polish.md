# Polish: japan-theme-parks-pp-cli

Polish pass:
  Verify:      100% -> 100% (28/28, mock)
  Scorecard:   86 -> 86
  Tools-audit: 0 -> 0 pending findings
  gosec (hand-authored code): 1 -> 0
  Dogfood:     WARN -> WARN (generator false positive: isDryRunResponseForClient has 3 callers)
  Fixed:
  - QueryWaitRows (internal/store/japan_theme_parks_migrations.go): one fixed
    parameterized SQL statement (gosec G202), 3 more filter test cases.
  - dates human table: NOTE column shows the hours-unknown reason next to the
    ticket reason (datesNote in internal/cli/dates.go), with a unit test.

Skipped findings (retro candidates, not CLI defects):
- gosec v2.26.1 and @latest cannot load Go 1.27 export data; full scan was run
  under GOTOOLCHAIN=go1.26.9. Build still uses go1.27.2.
- 30 gosec findings in generator-emitted files.
- Scorecard cache_freshness/vision/data_pipeline/error_handling/mcp_quality are
  structural (no sync command, internal spec). Not changed to game the score.
- Live check skips snapshot (writes data).

```
---POLISH-RESULT---
scorecard_before: 86
scorecard_after: 86
verify_before: 100
verify_after: 100
dogfood_before: WARN
dogfood_after: WARN
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 1
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
ship_recommendation: ship
further_polish_recommended: no
---END-POLISH-RESULT---
```
