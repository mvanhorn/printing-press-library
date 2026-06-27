
=== verify ===
Runtime Verification: /Users/jarvis/printing-press/.runstate/vibe-signal/runs/20260627-vibe-signal-001/working/vibe-signal-pp-cli/vibe-signal-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
agent-context                  read         PASS   PASS     PASS     3/3
doctor                         local        PASS   PASS     PASS     3/3
evidence                       read         PASS   PASS     PASS     3/3
feedback                       read         PASS   PASS     PASS     3/3
hn                             read         PASS   PASS     PASS     3/3
profile                        read         PASS   PASS     FAIL     2/3
report                         read         PASS   PASS     PASS     3/3
search                         data-layer   PASS   PASS     PASS     3/3
sources                        read         PASS   PASS     FAIL     2/3
sync                           data-layer   PASS   PASS     PASS     3/3
which                          read         PASS   PASS     PASS     3/3
workflow                       read         PASS   PASS     FAIL     2/3

Path-Param Probes (nested commands with <positional> args):
  PASS hn item
  PASS profile delete
  PASS profile save
  PASS profile show
  PASS profile use

Data Pipeline: PASS: sync completed (sql unavailable, table validation skipped)
Pass Rate: 100% (17/17 passed, 0 critical)
Verdict: PASS

=== validate-narrative ===
OK: 7 narrative commands resolved and full examples passed

=== dogfood ===
dogfood: using spec /Users/jarvis/printing-press/.runstate/vibe-signal/runs/20260627-vibe-signal-001/working/vibe-signal-pp-cli/spec.yaml (bundled)
dogfood: caller --spec=/Users/jarvis/printing-press/.runstate/vibe-signal/runs/20260627-vibe-signal-001/vibe-signal.yaml overridden by bundled /Users/jarvis/printing-press/.runstate/vibe-signal/runs/20260627-vibe-signal-001/working/vibe-signal-pp-cli/spec.yaml
Dogfood Report: vibe-signal-pp-cli
================================

Path Validity:     0/0 valid (SKIP)
  Detail: internal-yaml spec: paths validated at parse time

Auth Protocol:     MATCH
  Generated: Uses "unknown" prefix
  Detail: no bot/bearer/basic scheme detected

OAuth Scope Cover: 0/0 endpoints covered (SKIP)
  Detail: no OAuth-scoped endpoints in spec

Dead Flags:        0 dead (PASS)

Dead Functions:    1 dead (WARN)
  - writeNoop (defined, never called)

Data Pipeline:     PARTIAL
  Sync: calls domain-specific Upsert methods (GOOD)
  Search: uses generic Search only or direct SQL
  Domain tables: 1

Examples:          7/7 commands have examples (PASS)

Novel Features:    3/3 survived (PASS)

MCP Surface:       PASS (MCP surface mirrors the Cobra tree at runtime)

Verdict: FAIL
  - 1 dead helper functions found
  - defaultSyncResources empty: sync command is a runtime no-op; store-dependent novel commands have no advertised population path
  - 3/3 novel features missing data-source strategy: report (report.go) — missing // pp:data-source <auto|local|live|computed> annotation; evidence (evidence.go) — missing // pp:data-source <auto|local|live|computed> annotation; sources list (sources_list.go) — missing // pp:data-source <auto|local|live|computed> annotation
  - pure-logic packages with no tests: source

=== workflow-verify ===
Workflow Verification: vibe-signal-pp-cli
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
=== vibe-signal-pp-cli ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
  ✓ canonical-sections passed

=== scorecard ===
Quality Scorecard: vibe-signal

  Output Modes         10/10
  Auth                 10/10
  Error Handling       8/10
  Terminal UX          9/10
  README               10/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          10/10
  MCP Desc Quality     10/10
  MCP Token Efficiency 7/10
  MCP Remote Transport 10/10
  MCP Tool Design      N/A
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      5/10
  Breadth              7/10
  Vision               8/10
  Workflows            10/10
  Insight              10/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           6/10
  Auth Protocol           N/A
  Data Pipeline Integrity 7/10
  Sync Correctness        10/10
  Live API Verification   N/A
  Type Fidelity           4/5
  Dead Code               4/5

  Total: 83/100 - Grade A
  Note: omitted from denominator: mcp_tool_design, mcp_surface_strategy, auth_protocol, live_api_verification

Sample Output Probe (live command sample)
  Binary refresh: fresh_fallback (same-name runnable binary is newer than Go sources)
  Passed: 3/3  (100% pass rate, 0 skipped)

Gaps:
  - MCP: 3 tools (3 public, 0 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         1.446s
  validate-narrative  PASS    0         97ms
  dogfood           PASS    0         708ms
  workflow-verify   PASS    0         11ms
  apify-audit       PASS    0         17ms
  verify-skill      PASS    0         788ms
  scorecard         PASS    0         861ms

Verdict: PASS (7/7 legs passed)
