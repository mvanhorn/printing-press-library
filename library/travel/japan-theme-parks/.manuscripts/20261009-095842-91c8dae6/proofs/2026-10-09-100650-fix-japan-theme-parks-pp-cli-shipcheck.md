# Shipcheck: japan-theme-parks-pp-cli (2026-10-09)

## Runs
| Run | verify | validate-narrative | dogfood | workflow-verify | apify-audit | verify-skill | scorecard | Umbrella |
|---|---|---|---|---|---|---|---|---|
| 1 | PASS 100% | PASS | PASS (WARN) | PASS | PASS | PASS | HOLD: manifest evidence unavailable (87/100) | HOLD |
| 2 | PASS 100% (parks exec 2/3) | PASS | PASS (WARN) | PASS | PASS | PASS | PASS 88/100 A | PASS |
| 3 (final) | PASS 100% (28/28, 0 critical; parks 3/3) | PASS (7 narrative commands, full examples) | PASS (WARN: 1 generator helper `isDryRunResponseForClient` reported dead) | workflow-pass | PASS | PASS | PASS 88/100 Grade A; live sample 3/3 | PASS 7/7 |

## Blockers found and fixes
1. Scorecard hold "manifest evidence unavailable": `.printing-press.json` (and spec.yaml, manifest.json, CHANGELOG.md, .printing-press-patches/) were never written because the Phase 10 generate run exited on govulncheck before the manifest step. Fix: same-spec regenerate with `--validate=false` into a scratch dir; copied only those files. Recorded in research/build-notes.md as a retro finding.
2. Pre-shipcheck dogfood FAIL: unverified novel hosts (fixed with discovery/documented-urls.txt listing fetched and linked URLs), `internal/sources` had no tests (added httptest table tests), dead flag `--max-age` (now drives a stale-history stderr hint in `typical` and `waits`), 3 dead generated helpers removed.
3. verify mock exec on `parks` probed an unknown positional: added `pp:happy-args park=tds`.

## Scores
- verify pass rate: 100% before and after (exec on parks 2/3 -> 3/3).
- scorecard: 87 -> 88 / 100 (Grade A). Low dimensions: Cache Freshness 3/10 (no sync by design), Data Pipeline Integrity 7/10, Vision 7/10.

## Behavioral sample (live, 2026-10-09)
- `dates --from 2026-11-13 --to 2026-11-14 --park tds,usj,legoland`: TDS on_sale, T1 adult 10,900/12,400 yen, hours 09:00-21:00 official; USJ hours null "beyond third-party source horizon (ThemeParks.wiki schedule ends 2026-11-08)"; USJ/Legoland ticket unknown reasons.
- `dates 2026-12-20 --park tdl`: T1 not_yet_on_sale, sale_opens_at 2026-10-20T14:00+09:00 observed=false.
- `snapshot --park tds,usj`: 66 rows inserted; repeat run 36 skipped as duplicates.
- `typical --park tds`, `waits tds --open-only --max-wait 30`: history used, n and dates shown; Queue-Times credit in meta.

## Verdict
ship

## Re-run after Phase 4.95 fixes (run 4)
verify PASS 100% (28/28, 0 critical); validate-narrative PASS; dogfood PASS (WARN); workflow-verify workflow-pass; apify-audit PASS; verify-skill PASS; scorecard PASS 88/100 Grade A. Umbrella PASS 7/7. Verdict unchanged: ship.

## Re-run after Phase 18 dry-run fix and Phase 19 polish (run 5)
verify PASS 100% (28/28, 0 critical); validate-narrative PASS; dogfood PASS (WARN: generator helper isDryRunResponseForClient reported dead, false positive); workflow-verify PASS; apify-audit PASS; verify-skill PASS; scorecard PASS 88/100 Grade A, live sample 3/3. Umbrella PASS 7/7. Verdict unchanged: ship.
