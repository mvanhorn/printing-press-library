# zoho-campaigns Shipcheck Report

## Umbrella verdict: PASS (7/7 legs)
| Leg | Result | Elapsed |
|-----|--------|---------|
| verify | PASS | 18.8s |
| validate-narrative | PASS | 1.0s |
| dogfood | PASS | 5.9s |
| workflow-verify | PASS | 35ms |
| apify-audit | PASS | 61ms |
| verify-skill | PASS | 5.0s |
| scorecard | PASS (92/100, Grade A) | 5.6s |

## Scorecard: 92/100 Grade A
Omitted from denominator: mcp_surface_strategy, auth_protocol, live_api_verification. Dead code 5/5.

## Live sample probe: 3/6 passed in-harness
The three failures (digest, engagement, bounce-audit) are the auto-mode live-auth commands 401ing inside the probe's sandboxed environment (no credentials available to the harness). All six novel commands verified working against the live Kontur org in the real environment during Phase 3 acceptance (digest 3-campaign rollup, engagement real ranks, bounce-audit 67 contacts, journey 14-campaign named history, delta/growth honest single-snapshot states).

## Blockers found and fixed during Phase 3/4 loop
1. Recipients endpoint 20-row silent truncation → undocumented pagination encoded in spec + paginated fetcher.
2. Bare `sync` XML/0-records → resfmt=JSON embedded in five sync paths; bare sync = 44 campaigns.
3. Windows DACL writer/verifier disagreement → RestrictPrivateFile platform files + paths.go hook; headless auth loop verified end-to-end.
4. Generated-test HOME/USERPROFILE sandbox leak → 7 test files patched; suite green.
5. Unregistered auth set-token → registered.
6. root.Short narrative drift → aligned at spec + generated file.

## Verdict: ship
All ship-threshold conditions met; no known functional bugs in shipping-scope features.
