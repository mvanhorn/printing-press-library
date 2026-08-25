# Polish Report — sarvam-pp-cli

## Delta
- Scorecard: 91 → 93/100 (+2; Dead Code 4→5, Insight 4→6)
- Verify: 100% (50/50) throughout
- Live dogfood: 173/173 → 186/186 (+13 after docai schema subcommand refactor)
- Tools-audit: 2 pending → 0
- Gosec novel-file findings: 8 → 0
- PII: clean throughout

## Fixes applied
- Removed dead `collectionItemsForOutput` helper (scorecard Dead Code 4→5)
- Refactored `docai schema` from action-args to real Cobra subcommands (list/save/get/diff/delete) — resolves depth-mismatch warning and adds 13 testable commands
- Enriched thin MCP descriptions on `client list` and `learnings list`
- Fixed gosec findings in novel files: 0755→0750 dir perms, justified #nosec for user-facing 0644 outputs and user-supplied file reads
- Fixed output-review warning: `voices preview` now exits non-zero (6) when all speakers fail

## Skipped findings
- gosec G201/G202/G304/G404 in generator-emitted files (store.go, platform/, jobs.go): generator parameterized-query and cache-path patterns, retro candidates not CLI defects
- dogfood depth-check WARN on `docai schema list` (depth-3 command): checker caps at depth 2; command resolves correctly (agent-context confirms)

## Remaining issues
- scorecard `live_api_verification` unverified (fixture-dependent stateful commands); authoritative live dogfood 186/186 covers them via typed exit codes

## Live matrix
- exercised (186/186 with real API key)

---POLISH-RESULT---
scorecard_before: 91
scorecard_after: 93
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 8
gosec_after: 0
tools_audit_before: 2 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- removed dead collectionItemsForOutput helper
- refactored docai schema into real subcommands (list/save/get/diff/delete)
- enriched thin MCP descriptions on client list and learnings list
- fixed gosec findings in novel files (perms + justified nosec)
- voices preview now exits non-zero when all speakers fail
skipped_findings:
- generator-emitted gosec findings: retro candidates, not CLI defects
- docai schema list depth-check WARN: checker depth-2 cap, command resolves correctly
remaining_issues:
- scorecard live_api_verification unverified; covered by live dogfood 186/186
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: All mechanical diagnostics are clean; the only remaining scorecard dimension requires real batch-job fixtures only the user can supply.
---END-POLISH-RESULT---
