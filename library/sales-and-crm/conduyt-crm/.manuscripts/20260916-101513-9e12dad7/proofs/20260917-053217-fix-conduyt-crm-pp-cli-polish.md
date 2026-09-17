# Polish Results for conduyt-crm-pp-cli (phase 19, 2026-09-17 ~5:31 AM ET)

Mid-pipeline invocation, Phase 3 bundle 8/8 transcendence rows, no sub-60 reprint hold. Divergence check: the public copy is the May v4.2.0 print; this tree is a wholesale v4.32.1 reprint, proceeded without syncing.

                    Before         After          Delta
  Scorecard:        99/100         99/100         0
  Verify:           100% (488)     100% (488)     0
  Live matrix:      not_exercised  exercised      keyed 8/8 pass
  Tools-audit:      2 pending      0 pending      -2 (accepted, generated framework files)
  PII-audit:        0              0
  go vet:           0              0
  gosec (hand):     3              0              -3
  verify-skill:     0 findings     0 findings
  workflow-verify:  pass           pass

Fixes applied:
- imports watch --verify reads the real side-effect keys retryableFailed / exhaustedFailed (legacy fallback kept); the documented example verifies live, exit 0
- root Short/Long: "Conduyt Crm" -> "Conduyt CRM", truncated Short replaced
- gosec G104 x3 fixed in send_check.go, imports_blame.go, contacts_verify_line_type.go
- send-check partial reason names the cap value, contacts checked, and the --limit fix
- automations hours-audit: summary.unpublished counter under --published-only
- reports compare: NON-COMPARABLE listing after the table, capped at 10 + "and N more"
- research.json narrative: send-check example --limit 2000; 429 fix = 100 req/min per key; 403 fix drops invented scope names; README/SKILL re-rendered
- README Health Check shows real doctor output (paths anonymised)
- tools-audit ledger: 2 thin-short accepted with rationale; PII ledger written, no findings

Skipped findings:
- dogfood dead function formatReportedTotal: false positive (called in generated helpers.go)
- gosec 38 findings in generator-emitted files: retro candidates
- unkeyed live-check 0/8 HTTP 401: environmental; keyed re-run 8/8; phase 5 acceptance owns the live gate
- output-review: dialer coverage agent-scoped queue depth (no pool column), reports scorecard UUID ordering / no summary: contract-changing UX decisions deferred to Paul

---POLISH-RESULT---
scorecard_before: 99
scorecard_after: 99
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: not_exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 3
gosec_after: 0
tools_audit_before: 2 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
remaining_issues:
ship_recommendation: ship
further_polish_recommended: no
---END-POLISH-RESULT---

Post-polish gate (fable): Codex adversarial round on the polish diff and a full live matrix rerun to re-bind the phase 5 acceptance marker to the polished source.

## Close-out (2026-09-17 11:05 AM ET)
- Polish fix sets fix57–fix65 (imports blame/watch strictness, reports compare structured segments, dialer coverage meta validation, send check verdicts, watch counters) each followed by a Codex adversarial round; r42–r50 each found one fail-open or lenient-parsing gap, fixed in the next set; r51 APPROVE on b16abde.
- go build / go vet / go test green (1,864 passed, 2 skipped); binary rebuilt.
- Live matrix rerun after the final fix: 1757/1757 passed, 0 failed, 2179 skipped, 0 hollow (20260917-101900-dogfood-results.json); phase5-acceptance.json pass; key-leak grep clean.
