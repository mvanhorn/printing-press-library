# Shipcheck

Final canonical shipcheck: PASS, 7/7 legs. Five planned features recognized and five live output samples passed. Scorecard 80/100 Grade A (structural, not substituted for source correctness). Public live matrix/source agreement proofs are in project evidence/live.

d (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
  ✓ canonical-sections passed

=== scorecard ===
Quality Scorecard: weathernews

  Output Modes         10/10
  Auth                 10/10
  Error Handling       8/10
  Terminal UX          9/10
  README               10/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          9/10
  MCP Desc Quality     10/10
  MCP Token Efficiency 10/10
  MCP Remote Transport 10/10
  MCP Tool Design      N/A
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      5/10
  Breadth              7/10
  Vision               9/10
  Workflows            10/10
  Insight              8/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           5/10
  Auth Protocol           N/A
  Data Pipeline Integrity 7/10
  Sync Correctness        10/10
  Live API Verification   N/A
  Type Fidelity           5/5
  Dead Code               1/5

  Total: 80/100 - Grade A
  Note: omitted from denominator: mcp_tool_design, mcp_surface_strategy, auth_protocol, live_api_verification

Sample Output Probe (live command sample)
  Binary refresh: rebuilt (staged binary was older than Go sources)
  Passed: 5/5  (100% pass rate, 0 skipped)

Gaps:
  - dead_code scored 1/5 - needs improvement
  - MCP: 2 tools (2 public, 0 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         5.245s
  validate-narrative  PASS    0         68ms
  dogfood           PASS    0         2.105s
  workflow-verify   PASS    0         12ms
  apify-audit       PASS    0         21ms
  verify-skill      PASS    0         1.246s
  scorecard         PASS    0         5.509s

Verdict: PASS (7/7 legs passed)
