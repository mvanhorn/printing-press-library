# Clarify Shipcheck Report

## Umbrella result (cli-printing-press shipcheck)

| Leg | Result |
|-----|--------|
| verify | PASS (5.1s) |
| validate-narrative | PASS (strict, full examples) |
| dogfood | PASS (novel_features_check 7/7 planned=found) |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | HOLD — 97/100 Grade A; sole unverified dimension: live_api_verification |

## Scorecard highlights
- Total 97/100, Grade A. MCP remote transport / tool design / surface strategy 10/10 (Cloudflare pattern, 75 endpoint tools collapsed to search+execute).
- Cache freshness 5/10 (cache block intentionally not enabled: workspace data, manual sync + doctor path chosen).
- Sample output probe: 6/7 novel commands pass; the prep "failure" is the intended empty-mirror exit-3 hint (no mirror exists in the probe environment), not a code defect.

## Blockers found and fixed during build
1. Spec: 88 schema objects carried boolean `required` (invalid JSON Schema) — stripped mechanically before generate.
2. Auth: generated AuthHeader() sent the raw key; Clarify needs `Authorization: api-key <key>`. Patched to normalize (prepend scheme when missing); preserved across regen.
3. Narrative: sync examples referenced non-existent resource names; corrected to `sync --resources resources --path-context object=<type>`; recipes corrected to real command paths (`objects resources get`, `objects records create`).

## Verification state
- verify pass rate: PASS leg, no critical failures.
- Behavioral acceptance tests: 10 hand-written tests over the 7 novel commands (content assertions incl. negative and absence cases) — all pass.
- Live API verification: NOT RUN — requires workspace slug (Clarify keys are workspace-scoped; probe with key returned structured 401 "Invalid API key" against an arbitrary slug, proving programmatic reachability but not the credential).

## Ship recommendation
`ship` contingent on Phase 5 live dogfood once the workspace slug is provided; otherwise `ship` with live verification recorded as auth-gated skip.
