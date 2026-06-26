# Polish Pass — sigma-computing-pp-cli

Scorecard 97→97, Verify 100→100, dogfood PASS, tools-audit/pii-audit/go-vet clean.

Fix applied: wired `hintIfUnsynced` into `workbook stale` and `export bulk` so an empty/unsynced local store emits a stderr "run sync first" hint instead of silently returning empty results (stdout stays clean). Tests green.

ship_recommendation: hold — BUT the sole reason is publish-validate's phase5 live-acceptance gate, which requires authenticated live testing we skipped (no credentials this run). Not a quality defect. A valid phase5-skip.json (auth_required_no_credential) is present, which the Phase 5.6 promote gate accepts.

further_polish_recommended: no — everything polish controls is at max.
