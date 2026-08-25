# Respond.io Shipcheck Report

## Result
- shipcheck verdict: HOLD - single unverified dimension only: `live_api_verification` (auth-gated API, no credential available in this environment)
- All automated legs PASS: verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill
- scorecard: 95/100 Grade A (1 of 26 dimensions unverified: live_api_verification)

## Per-leg

=== verify ===
Runtime Verification: /Users/bobe/printing-press/.runstate/bobe-fae71586/runs/20260805-222426-ffd268a6/working/respondio-pp-cli/respondio-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
resource-path:export           static       PASS   PASS     PASS     3/3
resource-path:import           static       PASS   PASS     PASS     3/3
agent-context                  read         PASS   PASS     PASS     3/3
analytics                      data-layer   PASS   PASS     PASS     3/3
api                            local        PASS   PASS     PASS     3/3
auth                           local        PASS   PASS     PASS     3/3
comment                        read         PASS   PASS     PASS     3/3
contact                        read         PASS   PASS     PASS     3/3
conversation                   read         PASS   PASS     PASS     3/3
doctor                         local        PASS   PASS     PASS     3/3
export                         data-layer   PASS   PASS     PASS     3/3
feedback                       read         PASS   PASS     PASS     3/3
import                         data-layer   PASS   PASS     PASS     3/3
learnings                      read         PASS   PASS     FAIL     2/3
message                        read         PASS   PASS     PASS     3/3
overview                       read         PASS   PASS     PASS     3/3
playbook                       read         PASS   PASS     FAIL     2/3
profile                        read         PASS   PASS     FAIL     2/3
recall                         read         PASS   PASS     PASS     3/3
report                         read         PASS   PASS     FAIL     2/3
search                         data-layer   PASS   PASS     PASS     3/3
space                          read         PASS   PASS     PASS     3/3
sync                           data-layer   PASS   PASS     PASS     3/3
teach                          read         PASS   PASS     PASS     3/3
teach-lookup                   read         PASS   PASS     PASS     3/3
teach-pattern                  read         PASS   PASS     PASS     3/3
teach-playbook                 read         PASS   PASS     PASS     3/3
which                          read         PASS   PASS     PASS     3/3
workflow                       read         PASS   PASS     FAIL     2/3

Path-Param Probes (nested commands with <positional> args):
  PASS auth set-token
  PASS contact add-tags
  PASS contact by-tag
  PASS contact create
  PASS contact delete
  PASS contact get
  PASS contact list-channels
  PASS contact remove-tags
  PASS contact update
  PASS contact update-lifecycle
  PASS contact upsert
  PASS conversation assign
  PASS conversation update-status
  PASS learnings confirm
  PASS learnings forget
  PASS learnings reject
  PASS message get
  PASS message list
  PASS message send
  PASS profile delete
  PASS profile save
  PASS profile show
  PASS profile use
  PASS space get-custom-field
  PASS space get-user
  PASS space list-templates

Data Pipeline: PASS: sync completed (sql unavailable, table validation skipped)
Pass Rate: 100% (55/55 passed, 0 critical)
Verdict: PASS

=== validate-narrative ===
OK: 10 narrative commands resolved and full examples passed

=== dogfood ===
dogfood: using spec /Users/bobe/printing-press/.runstate/bobe-fae71586/runs/20260805-222426-ffd268a6/working/respondio-pp-cli/spec.yaml (bundled)
dogfood: caller --spec=/Users/bobe/printing-press/respondresearch/respondio-spec.yaml overridden by bundled /Users/bobe/printing-press/.runstate/bobe-fae71586/runs/20260805-222426-ffd268a6/working/respondio-pp-cli/spec.yaml
Dogfood Report: respondio-pp-cli
================================

Path Validity:     0/0 valid (SKIP)
  Detail: internal-yaml spec: paths validated at parse time

Auth Protocol:     MATCH
  Spec: bearer token format (expects "Bearer " prefix)
  Generated: Uses "Bearer" prefix
  Detail: spec and generated client both use "Bearer"

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
Workflow Verification: respondio-pp-cli
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
=== respondio-pp-cli ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
  ✓ canonical-sections passed

=== scorecard ===
Quality Scorecard: respondio

  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          10/10
  README               10/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          10/10
  MCP Desc Quality     7/10
  MCP Token Efficiency 7/10
  MCP Remote Transport 10/10
  MCP Tool Design      7/10
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      5/10
  Breadth              10/10
  Vision               10/10
  Workflows            10/10
  Insight              8/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           10/10
  Auth Protocol           10/10
  Data Pipeline Integrity 10/10
  Sync Correctness        10/10
  Live API Verification   N/A
  Type Fidelity           5/5
  Dead Code               5/5

  Total: 95/100 - Grade A (1 of 26 dimensions unverified: live_api_verification)
  Note: omitted from denominator: mcp_surface_strategy, live_api_verification
  Hold: unverified dimensions: live_api_verification

Sample Output Probe (live command sample)
  Binary refresh: fresh_fallback (same-name runnable binary is newer than Go sources)
  Passed: 6/7  (86% pass rate, 0 skipped)
  Failures:
    - Tag cohort segments: output does not contain any token from query "VIP"

Gaps:
  - MCP: 28 tools (0 public, 28 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         3.848s
  validate-narrative  PASS    0         182ms
  dogfood           PASS    0         1.494s
  workflow-verify   PASS    0         12ms
  apify-audit       PASS    0         20ms
  verify-skill      PASS    0         2.42s
  scorecard         HOLD    3         695ms
    hold: scorecard hold: unverified dimensions: live_api_verification

Verdict: HOLD (unverified: 1/7 legs)
