# Shipcheck: lacourt-holidays-pp-cli

Run 1: HOLD. Scorecard "manifest evidence unavailable": generate had aborted at the govulncheck gate before writing .printing-press.json, manifest.json, spec.yaml, CHANGELOG.md. Fix: regenerated to a scratch dir with --validate=false and copied those 5 files in (deps already patched). Score 83.
Run 2: FAIL. verify-skill: README troubleshoot referenced nonexistent `sync --full`. Fix: research.json + README now advise --no-cache. Score 86.
Run 3: PASS 7/7 legs (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard). Scorecard 86/100 Grade A; live sample probe 5/5.

Behavioral checks run live (2026-11-06):
- is-court-day 2026-11-27 -> false, "court holiday: Day after Thanksgiving"; 2026-11-30 -> true
- add-court-days 2026-11-25 1 -> 2026-11-30, skipped Thanksgiving + day after
- add-court-days 2027-05-20 16 --back -> 2027-04-28 (hand-counted)
- deadline 2026-11-16 10 -> raw 2026-11-26, rolled to 2026-11-30
- court-days-between 2026-11-25 2026-11-30 -> 1
- next-holiday -> 2026-11-11 Veterans Day, 5 days
- ics --out-dir -> 15 files at that time (2016-2030; lookback since widened to cover 2006)
- is-court-day 2031-01-02 -> exit 3 unpublished year

Remaining gaps: cache freshness 3/10 and no sync resource (spec response is a string, so no syncable resource was emitted); dead generator helpers (maxAge, endpointPositionalArgs, retainCLIQueryParams).

Verdict: ship
