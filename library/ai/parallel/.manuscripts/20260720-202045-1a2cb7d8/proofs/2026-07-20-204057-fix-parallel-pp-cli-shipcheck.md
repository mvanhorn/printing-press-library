
=== verify ===
warning: no servers defined in spec; generated CLI will require base_url in config
warning: accepted root-level 'mcp:' for backwards compatibility; rename to 'x-mcp:' per OpenAPI extension convention
Runtime Verification: /Users/apple/printing-press/.runstate/apple-0b5cfea6/runs/20260720-202045-1a2cb7d8/working/parallel-pp-cli/parallel-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
resource-path:tail             static       PASS   PASS     PASS     3/3
resource-path:export           static       PASS   PASS     PASS     3/3
resource-path:import           static       PASS   PASS     PASS     3/3
agent-context                  read         PASS   PASS     PASS     3/3
analytics                      data-layer   PASS   PASS     PASS     3/3
api                            local        PASS   PASS     PASS     3/3
auth                           local        PASS   PASS     PASS     3/3
balance                        read         PASS   PASS     FAIL     2/3
chat                           read         PASS   PASS     FAIL     2/3
doctor                         local        PASS   PASS     PASS     3/3
export                         data-layer   PASS   PASS     PASS     3/3
extract                        read         PASS   PASS     FAIL     2/3
feedback                       read         PASS   PASS     PASS     3/3
import                         data-layer   PASS   PASS     PASS     3/3
learnings                      read         PASS   PASS     FAIL     2/3
playbook                       read         PASS   PASS     FAIL     2/3
profile                        read         PASS   PASS     FAIL     2/3
recall                         read         PASS   PASS     PASS     3/3
research                       read         PASS   PASS     FAIL     2/3
search                         data-layer   PASS   PASS     PASS     3/3
session                        read         PASS   PASS     FAIL     2/3
sync                           data-layer   PASS   PASS     PASS     3/3
tail                           data-layer   PASS   PASS     PASS     3/3
teach                          read         PASS   PASS     PASS     3/3
teach-lookup                   read         PASS   PASS     PASS     3/3
teach-pattern                  read         PASS   PASS     PASS     3/3
teach-playbook                 read         PASS   PASS     PASS     3/3
websearch                      read         PASS   PASS     FAIL     2/3
which                          read         PASS   PASS     PASS     3/3
workflow                       read         PASS   PASS     FAIL     2/3

Path-Param Probes (nested commands with <positional> args):
  PASS auth set-token
  PASS learnings confirm
  PASS learnings forget
  PASS learnings reject
  PASS profile delete
  PASS profile save
  PASS profile show
  PASS profile use
  PASS research recall

Data Pipeline: PASS: sync completed (sql unavailable, table validation skipped)
Pass Rate: 100% (39/39 passed, 0 critical)
Verdict: PASS

=== validate-narrative ===
OK: 9 narrative commands resolved and full examples passed

=== dogfood ===
dogfood: using spec /Users/apple/printing-press/.runstate/apple-0b5cfea6/runs/20260720-202045-1a2cb7d8/working/parallel-pp-cli/spec.json (bundled)
warning: no servers defined in spec; generated CLI will require base_url in config
warning: accepted root-level 'mcp:' for backwards compatibility; rename to 'x-mcp:' per OpenAPI extension convention
dogfood: synced README.md (Unique Features) from novel_features_built
dogfood: synced SKILL.md (Unique Capabilities) from novel_features_built
dogfood: synced README.md (Troubleshooting) from research.json narrative
Dogfood Report: parallel-pp-cli
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
  Domain tables: 4

Examples:          10/10 commands have examples (PASS)

Novel Features:    7/7 survived (PASS)

MCP Surface:       PASS (MCP surface mirrors the Cobra tree at runtime)

Verdict: PASS

=== workflow-verify ===
Workflow Verification: parallel-pp-cli
================================

Overall Verdict: workflow-pass
  - no workflow manifest found, skipping

=== apify-audit ===
Apify Actor Audit
=================

No Apify actor references found.

Verdict: pass
  - no Apify actor references found, skipping

=== verify-skill ===
=== parallel-pp-cli ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
  ✓ canonical-sections passed

=== scorecard ===
Quality Scorecard: parallel

  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          10/10
  README               10/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          10/10
  MCP Desc Quality     9/10
  MCP Token Efficiency 7/10
  MCP Remote Transport 5/10
  MCP Tool Design      7/10
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      5/10
  Breadth              10/10
  Vision               10/10
  Workflows            10/10
  Insight              10/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           N/A
  Auth Protocol           N/A
  Data Pipeline Integrity 10/10
  Sync Correctness        10/10
  Live API Verification   N/A
  Type Fidelity           5/5
  Dead Code               5/5

  Total: 95/100 - Grade A
  Note: omitted from denominator: mcp_surface_strategy, path_validity, auth_protocol, live_api_verification

Sample Output Probe (live command sample)
  Binary refresh: fresh_fallback (same-name runnable binary is newer than Go sources)
  Passed: 6/7  (86% pass rate, 0 skipped)
  Failures:
    - Balance-aware run cost guard: empty output

Gaps:
  - MCP: 39 tools (0 public, 39 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         6.416s
  validate-narrative  PASS    0         343ms
  dogfood           PASS    0         3.154s
  workflow-verify   PASS    0         24ms
  apify-audit       PASS    0         44ms
  verify-skill      PASS    0         10.395s
  scorecard         PASS    0         1.33s

Verdict: PASS (7/7 legs passed)
