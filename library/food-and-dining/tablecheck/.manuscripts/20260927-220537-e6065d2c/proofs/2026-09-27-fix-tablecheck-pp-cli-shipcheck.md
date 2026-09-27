# Shipcheck proof

Verdict: ship. Printing Press4.32.5 umbrella passes all7 current legs (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard). See shipcheck-final.json.

Two umbrella runs: first had one full-example failure because selected live fields do not exist in a dry-run request plan. The research recipe now requests a bounded JSON shortlist; real field-selection examples/tests remain. Standalone strict/full narrative validation passes, then the complete umbrella passes.

Verify before/after:100% (27/27),0critical failures. Scorecard before/after:83/100,A. Novel behaviors:5planned/5built across8approved commands. Full race suite, vet and build passed. Every approved behavior was exercised by planning-live-check.py; the source cutoff varies at subsecond precision between independent reads, with preservation checked against the original source payload. No production functional bug remains.

Structural warnings:3unused generated helpers and empty defaultSyncResources because country-wide sync is intentionally outside scope. Planning commands read live/cache directly and do not depend on the generated store. Official partner auth and course-specific slots remain disclosed limitations of the approved public consumer approach.
