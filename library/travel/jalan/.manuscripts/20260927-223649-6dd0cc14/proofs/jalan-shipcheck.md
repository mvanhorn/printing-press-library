# Jalan shipcheck

Recommendation: ship, subject to the remaining live acceptance and polish gates.

The final canonical umbrella (`shipcheck-final.json`) exits 0: verify, narrative validation, dogfood, workflow verification, Apify audit, skill verification and scorecard all pass. Verification remains 26/26 (100%); the Steinberger score remains 80 (A). Five approved feature rows are built; no scope was dropped.

The first umbrella failed narrative/workflow validation. Rebuilt the staging binary and changed workflow arguments to inline command arguments because this installed runner ignores the separate `args` field; expectations now check the top-level response envelope. Independent source-checked E2E tests validate deeper facts. Focused rechecks and the second umbrella pass.

A benchmark exposed a decoder defect: script charset attributes could override the HTML document encoding. The decoder now follows BOM, MIME and document meta declarations, with strict UTF-8 validation and regression tests. The original failed live variant was not retained, so its precise cause remains unconfirmed. A later comparison surfaced one parse failure as explicit partial coverage; follow-up diagnosis and final live acceptance are tracked in subsequent proofs.

Evidence: `shipcheck.json` (first run), `workflow-fixed.json`, `shipcheck-final.json`, `live-e2e-final.json`, and the final acceptance/efficiency artifacts.
