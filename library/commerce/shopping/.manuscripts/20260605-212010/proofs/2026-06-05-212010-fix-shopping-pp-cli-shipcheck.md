
=== verify ===
Runtime Verification: /Users/nick/printing-press/.runstate/cli-printing-press-1205424d/runs/20260605-212010/working/shopping-pp-cli/shopping-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
agent-context                  read         PASS   PASS     PASS     3/3
amazon                         read         PASS   PASS     PASS     3/3
analytics                      data-layer   PASS   PASS     PASS     3/3
api                            local        PASS   PASS     PASS     3/3
arbitrage                      read         PASS   PASS     PASS     3/3
auth                           local        PASS   PASS     PASS     3/3
compare                        read         PASS   PASS     PASS     3/3
deals                          read         PASS   PASS     PASS     3/3
doctor                         local        PASS   PASS     PASS     3/3
export                         data-layer   PASS   PASS     PASS     3/3
feedback                       read         PASS   PASS     PASS     3/3
import                         data-layer   PASS   PASS     PASS     3/3
index                          read         PASS   PASS     PASS     3/3
leaderboard                    read         PASS   PASS     PASS     3/3
price-drops                    read         PASS   PASS     PASS     3/3
profile                        read         PASS   PASS     FAIL     2/3
retailers                      read         PASS   PASS     PASS     3/3
search                         data-layer   PASS   PASS     PASS     3/3
status                         read         PASS   PASS     PASS     3/3
sync                           data-layer   PASS   PASS     PASS     3/3
tail                           data-layer   PASS   PASS     PASS     3/3
watch                          read         PASS   PASS     FAIL     2/3
which                          read         PASS   PASS     PASS     3/3
workflow                       read         PASS   PASS     FAIL     2/3

Path-Param Probes (nested commands with <positional> args):
  PASS auth set-token
  PASS profile delete
  PASS profile save
  PASS profile show
  PASS profile use
  PASS retailers categories get-retailer
  PASS retailers products find-retailer-cursor
  PASS retailers products get-price-history
  PASS retailers products get-retailer-details-by-sku
  PASS retailers shopping get-retailer-product-details-by-sku
  PASS retailers shopping get-retailer-products
  PASS watch add

Data Pipeline: PASS: sync completed (sql unavailable, table validation skipped)
Pass Rate: 100% (36/36 passed, 0 critical)
Verdict: PASS

=== validate-narrative ===
OK: 9 narrative commands resolved and full examples passed

=== dogfood ===
dogfood: using spec /Users/nick/printing-press/.runstate/cli-printing-press-1205424d/runs/20260605-212010/working/shopping-pp-cli/spec.yaml (bundled)
dogfood: caller --spec=/tmp/lemmebuyit/openapi.yaml overridden by bundled /Users/nick/printing-press/.runstate/cli-printing-press-1205424d/runs/20260605-212010/working/shopping-pp-cli/spec.yaml
Dogfood Report: shopping-pp-cli
================================

Path Validity:     0/0 valid (N/A)

Auth Protocol:     MATCH
  Generated: Uses "unknown" prefix
  Detail: no bot/bearer/basic scheme detected

OAuth Scope Cover: 0/0 endpoints covered (SKIP)
  Detail: no OAuth-scoped endpoints in spec

Dead Flags:        0 dead (PASS)

Dead Functions:    0 dead (PASS)

Data Pipeline:     GOOD
  Sync: calls domain-specific Upsert methods (GOOD)
  Search: calls domain-specific Search methods (GOOD)
  Domain tables: 1

Examples:          10/10 commands have examples (PASS)

Novel Features:    7/7 survived (PASS)

MCP Surface:       PASS (MCP surface mirrors the Cobra tree at runtime)

Verdict: PASS

=== workflow-verify ===
Workflow Verification: shopping-pp-cli
================================

Overall Verdict: workflow-pass
  - no workflow manifest found, skipping

=== verify-skill ===
=== shopping-pp-cli ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
  ✓ canonical-sections passed

=== scorecard ===
Quality Scorecard: shopping

  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          10/10
  README               10/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          10/10
  MCP Desc Quality     10/10
  MCP Token Efficiency 4/10
  MCP Remote Transport 10/10
  MCP Tool Design      N/A
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      5/10
  Breadth              9/10
  Vision               10/10
  Workflows            10/10
  Insight              10/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           10/10
  Auth Protocol           8/10
  Data Pipeline Integrity 10/10
  Sync Correctness        10/10
  Live API Verification   N/A
  Type Fidelity           4/5
  Dead Code               5/5

  Total: 93/100 - Grade A
  Note: omitted from denominator: mcp_tool_design, mcp_surface_strategy, live_api_verification

Sample Output Probe (live command sample)
  Binary refresh: fresh_fallback (same-name runnable binary is newer than Go sources)
  Passed: 6/7  (86% pass rate, 0 skipped)
  Failures:
    - Local retail index sync: exit 4: Error: GET /retailers/walmart/products returned HTTP 401: {"error":"API key required"}
hint: check your API key. Set it with: export SHOPPING_API_KEY=<your-key>
      See API docs: https://www.lemmebuyit.com/developer
      Run 'shopping-pp-cli doctor' to check auth status.

Gaps:
  - mcp_token_efficiency scored 4/10 - needs improvement
  - MCP: 9 tools (0 public, 9 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         7.485s
  validate-narrative  PASS    0         191ms
  dogfood           PASS    0         1.391s
  workflow-verify   PASS    0         35ms
  verify-skill      PASS    0         1.033s
  scorecard         PASS    0         458ms

Verdict: PASS (6/6 legs passed)
