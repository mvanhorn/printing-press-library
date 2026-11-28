# Polish Results for meta-ads-library-pp-cli

Divergence check: first print — `meta-ads-library` is not present in the public library; proceeding on working copy `$CLI_WORK_DIR` as canonical.

```
Polish Results for meta-ads-library-pp-cli:

                    Before    After     Delta
  Scorecard:        97/100    97/100    +0
  Verify:           100%      100%      +0%
  Live matrix:      exercised -> exercised (141/141 PASS, 7/7 live_check)
  Tools-audit:      0         0         0 pending findings
  PII-audit:        0         0         0 pending findings
  Gosec (custom):   25        0         -25 findings
```

Fixes applied:
- Replaced `os.MkdirAll(outDir, 0o755)` with `0o750` and switched media file writes in `internal/adlibrary/client.go` to Go stdlib `os.OpenRoot(outDir)` with `0o600` file permissions (resolved gosec `G301` and `G304`).
- Explicitly handled close return values on `rows.Close()`, `stmt.Close()`, and `resp.Body.Close()` in `internal/adlibrary/store.go` and `internal/adlibrary/client.go` (resolved all 23 gosec `G104` findings).
- Ran `gofmt -w .`, rebuilt binary, re-verified `go vet`, full `go test -count=1 ./...`, `govulncheck` (0 vulnerabilities), re-minted `phase5-acceptance.json` (141/141 PASS, 0 hollow features), and verified 7/7 `shipcheck` legs PASS.

Skipped findings:
- Generator-emitted `go.mod` / `SKILL.md` Go version pin (`1.26.10` vs `1.26.11` for `GO-2026-6859`) — `go.mod` uses `1.26.11`, while `SKILL.md` canonical section is owned by `cli-printing-press` generator template (retro candidate).
- Official Graph API `archive` requires `META_ACCESS_TOKEN`; annotated `pp:requires-tier: authenticated` and skipped on `--auth-tier free` live dogfood run while all 17 zero-auth public GraphQL + local SQLite commands passed live.

Remaining issues:
- None.

---POLISH-RESULT---
scorecard_before: 97
scorecard_after: 97
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 25
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Confined media downloads in internal/adlibrary/client.go with os.OpenRoot and 0o750/0o600 permissions (closed gosec G301/G304)
- Handled close return values in internal/adlibrary/store.go and internal/adlibrary/client.go (closed 23 gosec G104 findings)
- Re-minted phase5-acceptance.json (141/141 PASS, 0 hollow features) and verified 7/7 shipcheck PASS
skipped_findings:
- SKILL.md canonical install Go version pin (1.26.10) is generator-owned; logged as retro candidate
- archive endpoint is annotated pp:requires-tier: authenticated and skipped on free-tier live dogfood
remaining_issues: []
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: Scorecard is 97/100 Grade A, verify is 100% (42/42), live dogfood passed 141/141 with 0 hollow features, and gosec/tools-audit/pii-audit/govulncheck all have 0 findings.
---END-POLISH-RESULT---
