
=== dogfood ===
Dogfood Report: dogecoin-pp-cli
================================

Path Validity:     0/0 valid (SKIP)
  Detail: internal-yaml spec: paths validated at parse time

Auth Protocol:     MATCH
  Generated: Uses "unknown" prefix
  Detail: spec not provided or no bot/bearer/basic scheme detected

Dead Flags:        0 dead (PASS)

Dead Functions:    17 dead (WARN)
  - apiErr (defined, never called)
  - authErr (defined, never called)
  - classifyAPIError (defined, never called)
  - emitTruncationWarning (defined, never called)
  - extractPaginatedItems (defined, never called)
  - extractResponseData (defined, never called)
  - green (defined, never called)
  - notFoundErr (defined, never called)
  - paginatedGet (defined, never called)
  - rateLimitErr (defined, never called)
  - rawAtPath (defined, never called)
  - red (defined, never called)
  - replacePathParam (defined, never called)
  - wantsHumanTable (defined, never called)
  - writeAPIErrorEnvelope (defined, never called)
  - writeNoop (defined, never called)
  - yellow (defined, never called)

Data Pipeline:     PARTIAL
  Sync: calls domain-specific Upsert methods (GOOD)
  Search: uses generic Search only or direct SQL
  Domain tables: 3

Examples:          10/10 commands have examples (PASS)

Novel Features:    7/7 survived (PASS)

MCP Surface:       PASS (MCP surface mirrors the Cobra tree at runtime)

Verdict: WARN
  - 17 dead helper functions found
  - 1 source client file(s) without rate-limit handling: internal/rpc/client.go — outbound HTTP without rate limiter or typed 429 handling
  - pure-logic packages with no tests: rpc
  - 5 naming violations: verb info→get in internal/cli/blockchain_info.go; verb info→get in internal/cli/mempool_info.go; verb info→get in internal/cli/mining_info.go; verb info→get in internal/cli/network_info.go; verb info→get in internal/cli/wallet_info.go

=== verify ===

Verification verdict WARN (pass rate 100%, threshold 80%). Running fix loop (max 3 iterations)...

2026/05/16 22:22:55 fix loop: execution failure for stats requires manual fix
2026/05/16 22:22:55 fix loop: skip exec_fail for stats: manual execution fix required
Runtime Verification: /Users/jorge/printing-press/.runstate/printing-press-machine-0322731a/runs/20260516-193909/working/dogecoin-pp-cli/dogecoin-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
agent-context                  read         PASS   PASS     PASS     3/3
api                            local        PASS   PASS     PASS     3/3
blockchain                     read         PASS   PASS     PASS     3/3
blocks                         read         PASS   PASS     PASS     3/3
doctor                         local        PASS   PASS     PASS     3/3
feedback                       read         PASS   PASS     PASS     3/3
health                         data-layer   PASS   PASS     PASS     3/3
import                         data-layer   PASS   PASS     PASS     3/3
mempool                        read         PASS   PASS     PASS     3/3
mining                         read         PASS   PASS     PASS     3/3
network                        read         PASS   PASS     PASS     3/3
node                           read         PASS   PASS     PASS     3/3
peers                          read         PASS   PASS     PASS     3/3
profile                        read         PASS   PASS     PASS     3/3
search                         data-layer   PASS   PASS     PASS     3/3
stats                          read         PASS   PASS     FAIL     2/3
sync                           data-layer   PASS   PASS     PASS     3/3
wallet                         read         PASS   PASS     PASS     3/3
which                          read         PASS   PASS     PASS     3/3

Data Pipeline: FAIL: sync crashed
Pass Rate: 100% (19/19 passed, 0 critical)
Verdict: WARN

Fix Loop: 1 iterations, improved: false
  Iteration 1: 100% -> 100% (+0%), 1 fixes applied

=== workflow-verify ===
Workflow Verification: dogecoin-pp-cli
================================

Overall Verdict: workflow-pass
  - no workflow manifest found, skipping

=== verify-skill ===
=== dogecoin-pp-cli ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, unknown-command)
  ✓ canonical-sections passed

=== validate-narrative ===
OK: 10 narrative commands resolved and full examples passed

=== scorecard ===
Quality Scorecard: dogecoin

  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          9/10
  README               8/10
  Doctor               8/10
  Agent Native         10/10
  MCP Quality          9/10
  MCP Desc Quality     N/A
  MCP Token Efficiency 10/10
  MCP Remote Transport 10/10
  MCP Tool Design      5/10
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      0/10
  Breadth              7/10
  Vision               5/10
  Workflows            6/10
  Insight              4/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           0/10
  Auth Protocol           N/A
  Data Pipeline Integrity 7/10
  Sync Correctness        7/10
  Live API Verification   N/A
  Type Fidelity           3/5
  Dead Code               0/5

  Total: 59/100 - Grade C
  Note: omitted from denominator: mcp_description_quality, mcp_surface_strategy, auth_protocol, live_api_verification

Gaps:
  - insight scored 4/10 - needs improvement
  - path_validity scored 0/10 - needs improvement
  - dead_code scored 0/5 - needs improvement

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  dogfood           PASS    0         1.631s
  verify            PASS    0         2.412s
  workflow-verify   PASS    0         31ms
  verify-skill      PASS    0         219ms
  validate-narrative  PASS    0         418ms
  scorecard         PASS    0         73ms

Verdict: PASS (6/6 legs passed)
