# Tesla CLI Polish

Polish ran in mid-pipeline (forked context) per the Press skill. Ship recommendation: ship. No remaining issues.

## Deltas
- Scorecard: 90/100 (no change; clean)
- Verify: 100% (no change)
- Verify-skill: 0 errors
- Tools-audit: 0
- PII-audit: 0
- Go vet: 0

## Fixes applied
- 4 doctor.go cache-status hints now point at `tesla snap --all` instead of the intentional no-op `tesla sync`
- 6 data_source.go error messages re-pointed to `tesla snap --all`
- analytics.go open-database error message re-pointed to `tesla snap --all`
- tesla_cost.go ledger Long help + zero-result Note describe the snap → timeline → cost ledger hydration path
- README "vehicle_data drain" troubleshoot dropped the fabricated `--no-wake` flag and recommends snap-on-cron with asleep-state awareness
- README "$0 supercharger sessions" troubleshoot dropped fabricated `--include charge-history` flag and recommends `snap --all` + `timeline`
- SKILL.md + README "Offline-friendly" bullets name actual offline-capable commands (analytics, timeline, vampire, cost, ready)
- research.json narrative.troubleshoots updated for future re-renders

## Skipped findings (structural)
- dogfood "defaultSyncResources empty" WARN: Tesla has no bulk-list endpoints; sync is intentionally a no-op. The real hydration command (`tesla snap --all`) now appears in every user-facing hint, error, and doc surface.
- 7 verify exec-score=2 cases: mock-harness limitation — pure parent commands (cost, keys, supercharger, workflow) print help and exit 0, and 2 auto-promoted endpoints (logs, notification-preferences) need 20+ body flags the mock harness can't populate. Verify still passes 100% (failed=0, critical=0).
- scorecard live_api_verification N/A: no fresh Tesla bearer in sandbox.
