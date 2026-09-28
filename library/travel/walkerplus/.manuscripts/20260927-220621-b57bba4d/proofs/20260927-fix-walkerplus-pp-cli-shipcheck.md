# Walkerplus shipcheck

Final verdict: ship. Full umbrella exited0; all seven legs passed.

Verification: {"mode": "mock", "pass_rate": 100, "passed": 8, "total": 8, "data_pipeline": true, "verdict": "PASS"}

Scorecard: {"steinberger": {"percentage": 80, "grade": "A", "total": 80}}

Initial umbrella had a narrative-leg failure; strict/full standalone narrative passed after the staged executable refreshed. Root inspection also identified an unsupported generated tail resolver; removal of the unused tail scaffold produced100% verify with zero critical findings. Final umbrella proof: shipcheck-final.json. No required feature dropped.

Behavioral evidence: integrated-checks.json; priority1-smoke.json; cli-live-e2e.txt; cli-live-final-targeted.txt; cli-live-city.txt; cli-live-free-admission.txt; cli-acceptance-report.json. Deterministic and race tests cover recurrence/exclusions/year boundaries/unknown pricing/dedup/city resolution/cancellation/request and cache bounds. Efficiency: efficiency-city/measurements.json and cli-selected-search.json.
