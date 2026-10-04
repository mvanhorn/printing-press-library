# Hostex reprint shipcheck (press 4.33.0, spec 3.15.0)

## Results
- Loop 1: verify PASS, validate-narrative PASS, dogfood FAIL (1 dead helper, 30% example coverage), workflow-verify PASS, apify-audit PASS, verify-skill PASS, scorecard 90/100 Grade A.
- Loop 2 (after fixes): 7/7 legs PASS, verify 100% (104/104, 0 critical), scorecard 91/100 Grade A, examples 10/10, 0 dead functions, novel features 7/7.

## Top blockers and fixes
- The 4.33.0 generator no longer emits placeholder `example-value` examples; 59 leaf commands had none. Wrote realistic examples (mutating commands carry `--dry-run`).
- Generator-emitted `handleBinaryResponseDelivery` is dead for a JSON-only spec; removed it.

## Notes
- Scorecard sample probe (live command sample) ran with a placeholder token (HTTP 401 on 3/7 commands); not an indication of CLI defects. Real read-only live checks happen in Phase 18.
- Scorecard gaps: Insight 4/10, Cache Freshness 5/10 (OpenAPI specs cannot opt into cache freshness).

## Ship recommendation
ship
