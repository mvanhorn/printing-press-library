# spothero shipcheck
Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         10.756s
  validate-narrative  PASS    0         303ms
  dogfood           PASS    0         4.529s
  workflow-verify   PASS    0         27ms
  apify-audit       PASS    0         55ms
  verify-skill      PASS    0         10.287s
  scorecard         PASS    0         2.36s

Verdict: PASS (7/7 legs passed)
Pass Rate: 98% (45/46 passed, 0 critical)
  Total: 83/100 - Grade A

Fixes: dead helpers wired, flags declared explicitly for verify-skill, pacing through one choke point, inventory host derived from the spec. Sample probe auth failures are the probe's sandbox having no session; those commands pass live. Recommendation: ship.
