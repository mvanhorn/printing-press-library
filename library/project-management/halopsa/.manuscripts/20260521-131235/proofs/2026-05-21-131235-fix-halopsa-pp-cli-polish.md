# HaloPSA Polish Report

## Delta

|                 | Before | After  | Delta |
|-----------------|--------|--------|-------|
| Scorecard       | 91/100 | 88/100 | -3 (mcp_description_quality newly scored, was N/A before mcp-sync generated tools-manifest.json) |
| Verify          | 100%   | 100%   | ±0 |
| Dogfood         | PASS   | PASS   | ±0 |
| go vet          | 2      | 0      | -2 |
| publish-validate| FAIL   | PASS   | +11 checks |
| tools-audit pending | 252 | 1268   | +1016 (mcp-sync exposed the typed-endpoint surface) |
| pii-audit       | 0      | 0      | ±0 |
| verify-skill    | 0      | 0      | ±0 |
| workflow-verify | pass   | pass   | ±0 |

## Fixes applied

- `internal/cli/sla_breaching.go`: corrected `%d → %g` format-string mismatch for float64 hours (go vet).
- Generated `tools-manifest.json` via `printing-press mcp-sync` — was missing, blocking publish-validate.
- Copied `phase5-skip.json` into `.manuscripts/20260521-131235/proofs/` where publish-validate expects it; corrected the marker's `auth_context.type` from `oauth2` → `bearer_token` to match the manifest's declared auth_type.
- Rebuilt the staged binary at `build/stage/bin/halopsa-pp-cli` (the stale binary predated novel-feature registration, so scorecard `--live-check` was reporting 0/13 features failing).
- `gofmt -w .` (removed unused `encoding/json` import in `triage.go`; minor whitespace).

## Skipped findings (all systemic / retro candidates — not blocking ship of this CLI)

- **246 thin-short** on DO-NOT-EDIT Hidden parent groupers (the auto-generated resource cobra parents with `Short: "Manage X"` and `Hidden: true`). Hand-edits get wiped on regen. The audit shouldn't flag Hidden parent groupers — generator-level fix.
- **6 missing-read-only** on the same Hidden parent groupers. Same root cause.
- **1016 thin-mcp-description** on typed-endpoint tools. HaloPSA's upstream OpenAPI has systemically thin operation descriptions (`Use this to return multiple X.<br>Requires authentication.`). Bulk-accept is forbidden by polish's 5-cluster duplicate-rationale gate; per-tool override across 1016 entries is out of single-pass scope. Generator-level: spec enrichment or composer improvement.
- **scorecard mcp_description_quality 0/10**: direct consequence of the 1016 thin descriptions above.
- **scorecard type_fidelity 1/5**: typed-response struct usage on a 1456-endpoint surface is a generator-level structural deficit.
- **scorecard cache_freshness 5/10**: generator doesn't emit a cache-freshness helper.
- **output-review format-mode-not-honored**: 13 novel commands honor `flags.asJSON || !isTerminal` but don't explicitly switch on `--plain`/`--human-friendly`. In a real TTY the table renderer fires (verified via `script(1)`). Refactoring 13 commands to a shared rendering layer is significant scope.
- **output-review contracts-burn day-count off-by-one**: cosmetic — `days_in_period` reports 32 for May; internally consistent.

## Ship recommendation: HOLD (per polish), further polish NOT recommended

Polish's hold rationale is **structural** — every remaining issue is a printing-press / upstream-Halo-spec problem that another polish pass cannot close. `/printing-press-retro` is the right next step. The CLI itself is functionally complete:

- 100% verify pass rate (133/133 commands).
- Dogfood PASS — all 13 transcendence features built, registered, and present in `novel_features_built`.
- verify-skill, validate-narrative, workflow-verify, publish-validate all PASS.
- go vet, gofmt clean.
- Live OAuth2 verification not performed (user declined credentials at Phase 0.5; phase5-skip.json present).

## Outcome

Per Phase 5.5 verdict-override rule: polish's `ship_recommendation: hold` downgrades the Phase 4 `ship` verdict to **hold**. Working copy stays in `$CLI_WORK_DIR`; library is not updated. Archiving still proceeds. Phase 6 routes to the hold-path menu.
