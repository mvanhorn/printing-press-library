# Shipcheck Report: openrouter-image-pp-cli

Run: 20260803-205427-851f4b39 | Date: 2026-08-03

## Verdict: SHIP (7/7 legs passed)

```
LEG                RESULT  EXIT    NOTES
verify             PASS    0       32.7s
validate-narrative PASS    0       16/16 examples resolved (fixed: sync resource image_models -> images)
dogfood            PASS    0       novel_features_check 6/6 found
workflow-verify    PASS    0
apify-audit        PASS    0
verify-skill       PASS    0       SKILL honest against CLI
scorecard          PASS    0       99/100 Grade A
```

## Scorecard: 99/100 Grade A
- Output Modes 10/10, Auth 10/10, Error Handling 10/10, Terminal UX 10/10, README 10/10, Doctor 10/10, Agent Native 10/10
- MCP Quality 8/10, Remote Transport 10/10, Tool Design 10/10, Surface Strategy 10/10
- Local Cache 10/10, Cache Freshness 10/10, Breadth 10/10, Vision 10/10, Workflows 10/10, Agent Workflow 9/10
- Path Validity 10/10, Auth Protocol 10/10, Data Pipeline 10/10, Sync Correctness 10/10, Type Fidelity 5/5, Dead Code 5/5

## Top blockers found & fixed
1. **validate-narrative FAIL (1 leg)**: README/SKILL quickstart + troubleshoot used `sync --resources image_models`, but the real syncable resource is `images` (maps to /images/models). Fixed at source in research.json + regenerated copies. 16/16 examples now pass.
2. Scorecard sample probe note (non-blocking): `regenerate gen-1234567890` (fake id) exits 3 not-found — correct behavior for a missing ledger ID.

## Before/after
- verify: PASS throughout (0 criticals)
- scorecard: 99/100 before and after (fix was narrative-only)

## Phase 5 (live dogfood)
- SKIPPED: bearer auth required, no OPENROUTER_API_KEY provided (user declined at the API Key Gate). phase5-skip.json written. Verification ran against mocks.
- Note: catalog sync + models rank + cost-estimate were additionally exercised against the REAL public catalog (38 models, live per-model pricing) outside the skip.

## Ship recommendation: SHIP
All ship-threshold conditions met: shipcheck exit 0, verify PASS, dogfood wiring clean, workflow-verify PASS, verify-skill exit 0, scorecard 99 >= 65, no flagship feature returns wrong/empty output (all 6 novel features behavior-tested; 5/6 live-sample probes passed, 1 correctly not-found on a fake ID).
