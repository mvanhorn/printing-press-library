
=== dogfood ===
Dogfood Report: wanderlust-goat-pp-cli
================================

Path Validity:     0/0 valid (SKIP)
  Detail: synthetic spec: path validity not applicable

Auth Protocol:     MATCH
  Generated: Uses "unknown" prefix
  Detail: spec not provided or no bot/bearer scheme detected

Dead Flags:        0 dead (PASS)

Dead Functions:    1 dead (WARN)
  - extractResponseData (defined, never called)

Data Pipeline:     PARTIAL
  Sync: calls domain-specific Upsert methods (GOOD)
  Search: uses generic Search only or direct SQL
  Domain tables: 1

Examples:          10/10 commands have examples (PASS)

Novel Features:    12/12 survived (PASS)

MCP Surface:       PASS (MCP surface mirrors the Cobra tree at runtime)

Verdict: WARN
  - 1 dead helper functions found
  - 13 source client file(s) without rate-limit handling: internal/atlasobscura/atlasobscura.go — outbound HTTP without rate limiter or typed 429 handling; internal/dispatch/anchor.go — outbound HTTP without rate limiter or typed 429 handling; internal/googleplaces/googleplaces.go — outbound HTTP without rate limiter or typed 429 handling; internal/hotpepper/hotpepper.go — outbound HTTP without rate limiter or typed 429 handling; internal/lefooding/lefooding.go — outbound HTTP without rate limiter or typed 429 handling; internal/naverblog/naverblog.go — outbound HTTP without rate limiter or typed 429 handling; internal/navermap/navermap.go — outbound HTTP without rate limiter or typed 429 handling; internal/osrm/osrm.go — outbound HTTP without rate limiter or typed 429 handling; internal/overpass/overpass.go — outbound HTTP without rate limiter or typed 429 handling; internal/reddit/reddit.go — outbound HTTP without rate limiter or typed 429 handling; internal/retty/retty.go — outbound HTTP without rate limiter or typed 429 handling; internal/tabelog/tabelog.go — outbound HTTP without rate limiter or typed 429 handling; internal/wikipedia/wikipedia.go — outbound HTTP without rate limiter or typed 429 handling
  - pure-logic packages with no tests: sourcetypes

=== verify ===
Runtime Verification: /Users/joeheitzeberg/printing-press/.runstate/cli-printing-press-5f1d6c25/runs/20260507-163338/working/wanderlust-goat-pp-cli/wanderlust-goat-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
agent-context                  read         PASS   PASS     PASS     3/3
coverage                       read         PASS   PASS     PASS     3/3
crossover                      read         PASS   PASS     PASS     3/3
doctor                         local        PASS   PASS     PASS     3/3
feedback                       read         PASS   PASS     PASS     3/3
goat                           read         PASS   PASS     FAIL     2/3
golden-hour                    read         PASS   PASS     FAIL     2/3
import                         data-layer   PASS   PASS     PASS     3/3
near                           read         PASS   PASS     FAIL     2/3
places                         read         PASS   PASS     PASS     3/3
profile                        read         PASS   PASS     PASS     3/3
quiet-hour                     read         PASS   PASS     FAIL     2/3
reddit-quotes                  read         PASS   PASS     FAIL     2/3
research-plan                  read         PASS   PASS     PASS     3/3
route-view                     read         PASS   PASS     PASS     3/3
status                         read         PASS   PASS     PASS     3/3
sync                           data-layer   PASS   PASS     PASS     3/3
sync-city                      read         PASS   PASS     PASS     3/3
which                          read         PASS   PASS     PASS     3/3
why                            read         PASS   PASS     FAIL     2/3
workflow                       read         PASS   PASS     PASS     3/3

Data Pipeline: PASS: sync completed (table validation skipped — sql command unavailable)
Pass Rate: 100% (21/21 passed, 0 critical)
Verdict: PASS

=== workflow-verify ===
Workflow Verification: wanderlust-goat-pp-cli
================================

Overall Verdict: workflow-pass
  - no workflow manifest found, skipping

=== verify-skill ===
=== wanderlust-goat-pp-cli ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, unknown-command)
  ✓ canonical-sections passed

=== scorecard ===
Quality Scorecard: wanderlust-goat

  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          9/10
  README               8/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          10/10
  MCP Desc Quality     N/A
  MCP Token Efficiency 4/10
  MCP Remote Transport 5/10
  MCP Tool Design      N/A
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      5/10
  Breadth              7/10
  Vision               5/10
  Workflows            6/10
  Insight              10/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           N/A
  Auth Protocol           N/A
  Data Pipeline Integrity 10/10
  Sync Correctness        10/10
  Live API Verification   N/A
  Type Fidelity           3/5
  Dead Code               4/5

  Total: 85/100 - Grade A
  Note: omitted from denominator: mcp_description_quality, mcp_tool_design, mcp_surface_strategy, path_validity, auth_protocol, live_api_verification

Gaps:
  - mcp_token_efficiency scored 4/10 - needs improvement
  - MCP: 2 tools (2 public, 0 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  dogfood           PASS    0         826ms
  verify            PASS    0         4.487s
  workflow-verify   PASS    0         10ms
  verify-skill      PASS    0         88ms
  scorecard         PASS    0         29ms

Verdict: PASS (5/5 legs passed)
