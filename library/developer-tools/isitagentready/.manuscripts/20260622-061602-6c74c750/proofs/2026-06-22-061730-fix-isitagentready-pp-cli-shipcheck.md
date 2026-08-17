# Shipcheck - isitagentready-pp-cli

## Umbrella verdict: PASS (7/7 legs)

| Leg | Result |
|-----|--------|
| verify | PASS (exit 0) |
| validate-narrative (--strict --full-examples) | PASS |
| dogfood | PASS (no wiring/path/skip failures; novel_features 6/6) |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS (SKILL flags/commands all resolve) |
| scorecard | PASS leg; total 64/100 (Grade C) |

## Scorecard: 64/100
Strong (10/10): Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native,
MCP Desc Quality, MCP Remote Transport, MCP Tool Design, Local Cache, Path Validity, Workflows, Insight.
Weak (Polish targets or inherent):
- **vision 3/10** - Polish target (README/SKILL narrative).
- **dead_code 1/5** - GENERATED template helpers unused by this API (`allowPartialFailure` flag,
  `detectPartialFailure`, `partialFailureErr` - partial-failure machinery for batch-mutating APIs).
  Template-shape dead code -> retro candidate (do not hand-patch generated files); Polish may strip.
- **data_pipeline_integrity 1/10, sync_correctness 2/10** - inherent: the upstream API is a single
  stateless POST with no list/sync endpoint, so there is no sync pipeline to score. Not a defect.
- **MCP Quality 6/10, Token Efficiency 7/10** - Polish target. (1 endpoint tool + cobratree mirror.)
- **Breadth 7/10** - inherent (single-endpoint API).

## Behavioral correctness (sample probe + manual)
- Manual end-to-end (live, no-auth): check, advice, report (+--only-failing/--select), guide (fetches
  real SKILL.md), gate (exit 1 below min, exit 0 at/above), diff, history, compare (21-check matrix),
  open-advice (cross-site backlog), batch - ALL produce correct output. No flagship returns wrong output.
- Automated live sampler (10s/command): 5/6. The lone miss is a live-scanning command timing out: a
  real scan runs ~21 upstream probes and takes ~3-12s, so gate/compare occasionally exceed the 10s
  sampler limit. This is a sampler-timeout-vs-slow-upstream artifact, not empty/wrong output - the same
  commands return full output under the normal 60s timeout (compare verified at 6.6s, gate at ~3-7s).

## Fixes applied this phase
- compare: parallelized live scans via cliutil.FanoutRun (12s+ sequential -> ~max-of-one-scan).
- diff: insufficient-data path now emits a non-empty info object (was `[]`).
- gate: sample example switched to a site that meets the bar (pass case).
- store.Append: mutex-guarded for safe concurrent persists (compare fan-out).

## Verdict: ship (pending mandatory Phase 5.5 Polish to lift score over 65)
All legs pass; all features verified working. The 64 total is held down by (a) Polish-addressable dims
(vision, MCP, dead_code) and (b) dims structurally N/A for a stateless single-endpoint API
(data_pipeline, sync). Carrying into Phase 5 dogfood + Phase 5.5 Polish.
