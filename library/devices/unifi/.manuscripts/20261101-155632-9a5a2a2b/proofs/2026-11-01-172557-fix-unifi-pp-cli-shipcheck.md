# Shipcheck proof: unifi-pp-cli
Commands: shipcheck --dir <work> --spec spec.json --research-dir <run> (2 loops; logs shipcheck-1.log, shipcheck-2.log)
Loop 1: 5/7. validate-narrative failed on a wrong recipe path; dogfood failed on 2 naming violations (network/protect `info`).
Fixes: recipe now `network list-devices`; commands renamed `get-info` in source, overlays, tests, manifest, docs.
Loop 2: 7/7 PASS, rc=0. verify 99% (188/189, 0 critical). Scorecard 96/100 Grade A.
Remaining verify failure: `raw` mock probe (a synthetic argument is not a valid raw request). Manual dry-run `raw GET network/integration/v1/info` exits 0.
Scorecard omits path_validity, auth_protocol, live_api_verification, so 96 says nothing about live behavior.
Sample probe: reach 401 and rule-predict empty-mirror without a key, as expected.
Cache freshness 5/10 accepted: auto-refresh is deliberately off, since a sync costs a large share of the 100 req/min connector budget.
Ship recommendation: ship to review phases. Final ship depends on the live matrix, which needs an API key.
