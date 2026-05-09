
=== dogfood ===
dogfood: synced .printing-press.json (novel_features) from novel_features_built
dogfood: synced README.md (Unique Features) from novel_features_built
dogfood: synced SKILL.md (Unique Capabilities) from novel_features_built
dogfood: synced internal/cli/root.go (Highlights) from novel_features_built
Dogfood Report: substack-pp-cli
================================

Path Validity:     0/0 valid (SKIP)
  Detail: internal-yaml spec: paths validated at parse time

Auth Protocol:     MATCH
  Generated: Uses "unknown" prefix
  Detail: spec not provided or no bot/bearer scheme detected

Dead Flags:        0 dead (PASS)

Dead Functions:    0 dead (PASS)

Data Pipeline:     PARTIAL
  Sync: calls domain-specific Upsert methods (GOOD)
  Search: uses generic Search only or direct SQL
  Domain tables: 1

Examples:          10/10 commands have examples (PASS)

Novel Features:    8/8 survived (PASS)

MCP Surface:       PASS (MCP surface mirrors the Cobra tree at runtime)

Verdict: PASS

=== verify ===
Runtime Verification: /Users/chirantan/printing-press/.runstate/cli-printing-press-5afd2fcc/runs/20260509-103230/working/substack-pp-cli/substack-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
agent-context                  read         PASS   PASS     PASS     3/3
api                            local        PASS   PASS     PASS     3/3
auth                           local        PASS   PASS     PASS     3/3
categories                     read         PASS   PASS     PASS     3/3
comments                       read         PASS   PASS     PASS     3/3
discover                       read         PASS   PASS     FAIL     2/3
doctor                         local        PASS   PASS     PASS     3/3
drafts                         read         PASS   PASS     PASS     3/3
engage                         read         PASS   PASS     PASS     3/3
export                         data-layer   PASS   PASS     PASS     3/3
feed                           read         PASS   PASS     PASS     3/3
feedback                       read         PASS   PASS     PASS     3/3
growth                         read         PASS   PASS     PASS     3/3
images                         read         PASS   PASS     FAIL     2/3
import                         data-layer   PASS   PASS     PASS     3/3
inbox                          read         PASS   PASS     PASS     3/3
notes                          read         PASS   PASS     PASS     3/3
posts                          read         PASS   PASS     PASS     3/3
profile                        read         PASS   PASS     PASS     3/3
profiles                       read         PASS   PASS     PASS     3/3
recommendations                read         PASS   PASS     FAIL     2/3
recs                           read         PASS   PASS     PASS     3/3
search                         data-layer   PASS   PASS     PASS     3/3
sections                       read         PASS   PASS     FAIL     2/3
settings                       read         PASS   PASS     PASS     3/3
subs                           read         PASS   PASS     PASS     3/3
sync                           data-layer   PASS   PASS     PASS     3/3
tags                           read         PASS   PASS     PASS     3/3
voice                          read         PASS   PASS     PASS     3/3
which                          read         PASS   PASS     PASS     3/3
workflow                       read         PASS   PASS     PASS     3/3

Data Pipeline: PASS: sync completed (table validation skipped — sql command unavailable)
Pass Rate: 100% (31/31 passed, 0 critical)
Verdict: PASS

=== workflow-verify ===
Workflow Verification: substack-pp-cli
================================

Overall Verdict: workflow-pass
  - no workflow manifest found, skipping

=== verify-skill ===
=== substack-pp-cli ===
  ✘ 3 error(s), 0 likely false-positive(s)
    [flag-names] substack-pp-cli sync: --pub is referenced in SKILL.md but not declared in any internal/cli/*.go
    [flag-commands] substack-pp-cli sync: --pub is not declared anywhere
    [positional-args] substack-pp-cli voice fingerprint: got 2 positional args; Use: "fingerprint" expects 0–0
      evidence: > client-voice.json
  ✓ canonical-sections passed

=== validate-narrative ===
FAILED [quickstart]: substack-pp-cli sync --pub on --since 30d → full example failed: /Users/chirantan/printing-press/.runstate/cli-printing-press-5afd2fcc/runs/20260509-103230/working/substack-pp-cli/substack-pp-cli sync --pub on --since 30d --dry-run: exit status 1: Error: unknown flag: --pub
unknown flag: --pub
FAILED [quickstart]: substack-pp-cli notes new --body "Stop refreshing the feed. Spend 15 minutes in your inbox replying to commenters and you'll outgrow 90% of writers who don't." --dry-run → full example failed: /Users/chirantan/printing-press/.runstate/cli-printing-press-5afd2fcc/runs/20260509-103230/working/substack-pp-cli/substack-pp-cli notes new --body Stop refreshing the feed. Spend 15 minutes in your inbox replying to commenters and you'll outgrow 90% of writers who don't. --dry-run: exit status 1: Error: unknown flag: --body
unknown flag: --body
FAILED [recipes]: substack-pp-cli sync --pub on --since 24h && substack-pp-cli growth attribution --days 7 --agent --select rank,note_excerpt,subs_acquired && substack-pp-cli engage reciprocity --days 7 --agent --select handle,net,drift → full example failed: /Users/chirantan/printing-press/.runstate/cli-printing-press-5afd2fcc/runs/20260509-103230/working/substack-pp-cli/substack-pp-cli sync --pub on --since 24h && substack-pp-cli growth attribution --days 7 --agent --select rank,note_excerpt,subs_acquired && substack-pp-cli engage reciprocity --days 7 --agent --select handle,net,drift --dry-run: exit status 1: Error: unknown flag: --pub
unknown flag: --pub
EMPTY [recipes]: cat week-of-notes.yaml | substack-pp-cli notes schedule --batch --guard --dry-run has no subcommand words to verify
FAILED [recipes]: substack-pp-cli recs find-partners --my-pub on --top 5 --json --select handle,pub,overlap_score | substack-pp-cli recs draft-outreach --based-on top --tone peer → full example failed: /Users/chirantan/printing-press/.runstate/cli-printing-press-5afd2fcc/runs/20260509-103230/working/substack-pp-cli/substack-pp-cli recs find-partners --my-pub on --top 5 --json --select handle,pub,overlap_score | substack-pp-cli recs draft-outreach --based-on top --tone peer --dry-run: exit status 1: Error: unknown flag: --based-on
unknown flag: --based-on
FAILED [recipes]: substack-pp-cli voice fingerprint --handle client-handle --json --select metric,value > client-voice.json && substack-pp-cli notes generate --voice-file client-voice.json --topic "morning routine" → full example failed: /Users/chirantan/printing-press/.runstate/cli-printing-press-5afd2fcc/runs/20260509-103230/working/substack-pp-cli/substack-pp-cli voice fingerprint --handle client-handle --json --select metric,value > client-voice.json && substack-pp-cli notes generate --voice-file client-voice.json --topic morning routine --dry-run: exit status 1: Error: unknown flag: --voice-file
unknown flag: --voice-file
DONE: 5 ok, 0 missing, 1 empty-words, 5 failed-examples, 0 unsupported
narrative validation failed

=== scorecard ===
Quality Scorecard: substack

  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          9/10
  README               8/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          10/10
  MCP Desc Quality     N/A
  MCP Token Efficiency 7/10
  MCP Remote Transport 10/10
  MCP Tool Design      5/10
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      10/10
  Breadth              10/10
  Vision               8/10
  Workflows            8/10
  Insight              4/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           4/10
  Auth Protocol           2/10
  Data Pipeline Integrity 7/10
  Sync Correctness        10/10
  Live API Verification   N/A
  Type Fidelity           3/5
  Dead Code               5/5

  Total: 74/100 - Grade B
  Note: omitted from denominator: mcp_description_quality, mcp_surface_strategy, live_api_verification

Sample Output Probe (live command sample)
  Passed: 8/8  (100% pass rate)

Gaps:
  - insight scored 4/10 - needs improvement
  - path_validity scored 4/10 - needs improvement
  - auth_protocol scored 2/10 - needs improvement
  - MCP: 39 tools (0 public, 39 auth-required) — readiness: partial

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  dogfood           PASS    0         880ms
  verify            PASS    0         3.474s
  workflow-verify   PASS    0         8ms
  verify-skill      FAIL    1         162ms
  validate-narrative  FAIL    1         139ms
  scorecard         PASS    0         77ms

Verdict: FAIL (2/6 legs failed)
