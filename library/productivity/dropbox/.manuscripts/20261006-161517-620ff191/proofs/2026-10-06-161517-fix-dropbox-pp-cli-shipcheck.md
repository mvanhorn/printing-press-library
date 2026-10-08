<!-- slop-gate: off -->
# dropbox-pp-cli shipcheck

## Loop 1
- Pre-shipcheck dogfood: FAIL (plan check leaf collided with promoted `check` resource; dogfood flagged host "check.user"). Fix: dropped files `check` resource from spec, regenerated with --force; merge dropped the hand-edit in helpers.go (scope hint), restored from git; gates green. Dogfood then WARN (10 generator-emitted helpers unused by this spec).
- shipcheck umbrella: PASS 7/7 legs (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard).
- Scorecard: 81/100 Grade A. Gaps: dead_code 0/5 (generator helpers), cache freshness 3/10 (manual index refresh by design).
- Live sample probe: 6/6 passed, 3 skipped.

```
  MCP Desc Quality     7/10
  MCP Token Efficiency 10/10
  MCP Remote Transport 5/10
  MCP Tool Design      5/10
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      3/10
  Breadth              10/10
  Vision               7/10
  Workflows            10/10
  Insight              10/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           9/10
  Auth Protocol           10/10
  Data Pipeline Integrity 7/10
  Sync Correctness        8/10
  Live API Verification   N/A
  Type Fidelity           5/5
  Dead Code               0/5

  Total: 81/100 - Grade A
  Note: omitted from denominator: mcp_surface_strategy, live_api_verification

Sample Output Probe (live command sample)
  Binary refresh: no_stage (no staged binary found)
  Passed: 6/6  (100% pass rate, 3 skipped)

Gaps:
  - dead_code scored 0/5 - needs improvement
  - MCP: 42 tools (0 public, 42 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         10.377s
  validate-narrative  PASS    0         421ms
  dogfood           PASS    0         3.666s
  workflow-verify   PASS    0         28ms
  apify-audit       PASS    0         124ms
  verify-skill      PASS    0         3.998s
  scorecard         PASS    0         965ms

Verdict: PASS (7/7 legs passed)
```

## Loop 2 + confirmation
- Live behavioral sample on a multi-million-row snapshot of the real account exposed (a) dev-folder duplicates (most dupes in node_modules) -> slice F, (b) real-scale latency -> slices G and H. All fixed and verified with output-digest equality.
- Every novel command run against real data (overview, tree, dupes, conflicts, mess, organize, plan check, apply preview, journal, search): all exit 0 with plausible results; dupes plan contains zero dev-dir paths; conflicts plan contains only identical/subset Selective Sync Conflict folders; apply preview made no writes.
- Narrative rewritten and regenerated; final shipcheck PASS 7/7, scorecard 81/100 Grade A, novel 9/9.
- Known non-blocking: dogfood WARN for 10 generator-emitted helpers unused by this spec (retro candidate); 3 generated endpoint mirrors lack examples.

## Verdict: ship
