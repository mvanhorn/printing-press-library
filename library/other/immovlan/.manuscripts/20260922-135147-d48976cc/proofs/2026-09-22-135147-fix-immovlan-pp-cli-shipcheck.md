# immovlan-pp-cli shipcheck (2026-09-22-135147)

Loop 1: verify-skill FAIL — SKILL recipe used `saved add ... --epc F,G`; --epc comes from the shared addCritFlags helper, invisible to the static flag walker. Fix: recipe rewritten without --epc in research.json; dogfood re-synced SKILL/README.

Loop 2: PASS 7/7 (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard). Scorecard 86/100 Grade A. Verify pass rate 80% (4/5, 1 skipped).

Behavioural checks (live, 2026-09-22): find on 1030,1210 F/G (towns fix) returns only those postcodes; show/enrich read PEB letter, rented flag, cadastral income, agency software; peb-trap, split-candidates, same-as (3 matches on 40 vs 820 immoweb rows), relisted, agencies all produce data from the store.

Verdict: ship

```
Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         7.33s
  validate-narrative  PASS    0         360ms
  dogfood           PASS    0         3.844s
  workflow-verify   PASS    0         17ms
  apify-audit       PASS    0         37ms
  verify-skill      PASS    0         4.987s
  scorecard         PASS    0         930ms

Verdict: PASS (7/7 legs passed)
```
