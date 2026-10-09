# Polish pass (Phase 5.5) — coinlocker-navi-pp-cli, run 20261009-095811-aff6d8f7

Polish pass:
  Verify:      93% -> 93%
  Scorecard:   77  -> 79 (+2; Dead Code 3/5 -> 5/5)
  Tools-audit: 0   -> 0 pending findings
  Fixed:
  - Removed 7 unused generated helpers in internal/cli/helpers.go
  - Removed unused global --max-age flag (no sync in this CLI)
  - vacancy: English-name lookup only for returned rows when no --gate (TestEkicubeStopAfterNamesOnlyReturnedRows)
  Ship recommendation: ship

Skipped findings (not CLI defects): verify matches happy-args by the last command word, so top-level `locker` received `--id=2685` from `source locker` (fixed after polish: `locker` now also accepts `--id`, TestLockerIDArg); vacancy and near --live hit the 15 s / 10 s probe limits only when Multi Ekicube is slow (dated queries 0.5-12 s); dogfood false positive dead function isDryRunResponseForClient; 22 gosec findings all in generated files (gosec v2.26.1 crashes on Go 1.27; run with GOTOOLCHAIN=go1.26.9); insight/cache-freshness/vision scores follow from the live-only design.

No publish, push, PR or git config change. About 10 live requests at ~1 req/s.
