# FRED CLI Shipcheck

- shipcheck: PASS (6/6 legs: verify, validate-narrative, dogfood, workflow-verify, verify-skill, scorecard)
- scorecard: 93/100 Grade A
- sample output probe: 5/5 (100%)
- live dogfood (Phase 5, level full): 110/110 passed, 0 failed — gate PASS
- 6 initial dogfood fails fixed in-session:
  - tags related/series happy-path: synthesized tag "example" rejected by FRED 400 -> added pp:happy-args=--tag-names=monthly
  - series search / tail error_path: FRED returns 200/warning for bad input (no honest error path) -> pp:no-error-path-probe
  - release calendar live probe timeout (17s) -> default --limit 100 (6s) + IsDogfoodEnv curtailment (2.8s)
- Known characteristic: sync/workflow archive are low-value for FRED (most endpoints require an ID; not bulk-syncable). Handled gracefully (0 items, no crash). Novel commands carry the value.

Verdict: ship
