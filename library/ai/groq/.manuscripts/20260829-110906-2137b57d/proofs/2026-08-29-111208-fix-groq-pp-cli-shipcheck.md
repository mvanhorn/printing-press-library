# Groq CLI Shipcheck Report

Run: 20260829-110906-2137b57d
CLI: `groq-pp-cli` at `$CLI_WORK_DIR`
Spec: official Groq OpenAPI v2.1 (extracted from console.groq.com/docs/api-reference) + enrichment

## Leg results (shipcheck umbrella, live mode enabled)

| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | Live-mode 100% (52/52, 0 critical) against real API with the user's key |
| validate-narrative | PASS | 11 ok, 0 failed examples (1 side-effectful auth example = expected UNSUPPORTED) |
| dogfood | PASS | Command tree, dead flags, novel-feature planned-vs-built, wiring checks clean |
| workflow-verify | PASS | |
| apify-audit | PASS | |
| verify-skill | PASS | flag-names/flag-commands/positional-args/canonical-sections all clean |
| scorecard | PASS | **93/100 Grade A**, no hold dimensions |

## Scorecard highlights (93/100)

- Full marks (10/10): Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Quality, MCP Remote Transport, Local Cache, Cache Freshness, Breadth, Vision, Workflows, Insight, Auth Protocol, Sync Correctness, Path Validity
- 7/10: MCP Desc Quality, MCP Token Efficiency, MCP Tool Design, Insight→10 after live-check
- 9/10: Agent Workflow
- 7/10: Data Pipeline Integrity; 4/5 Dead Code (minor scorecard polish)

## Sample Output Probe (live)

- Passed 5/6: rate-limits, costs, compare, batch validate, audio batch all exercised live against real API.
- **batch diagnose: 403 `not_available_for_plan`** — Batch API requires Groq Developer plan; the test account is Free tier. The command behaves correctly (surfaces the API's real error); not a code defect. Any batch-list/get/cancel command is plan-gated on this account. **Known gap: batch commands require a Developer/paid plan to test live.**

## Fixes applied during shipcheck loop

1. Resource command surface re-mapped via `x-pp-resource` (chat/responses/audio/models/embeddings/reranking/batches/files/fine-tunings) instead of one monolithic `openai` resource.
2. OperationId renames → clean endpoint names (`chat completions`, `audio speech|transcribe|translate`, `batches create|get|cancel|list`).
3. Removed top-level `oneOf` + property `anyOf` from audio transcription/translation request schemas so multipart `--file` flags generate; fixed the empty-body bug that made those endpoints unusable.
4. Implemented all 6 transcendence commands (rate-limits, costs, compare, batch validate, batch diagnose, audio batch) with live data sources, local-ledger reads, partial-failure accounting, and typed exit codes.
5. Declared positionals in `Use:` strings (`batch validate <file.jsonl>`, etc.) — fixed verify-skill positional-args findings.
6. Corrected research.json narrative chat examples to the real `chat completions --model --messages` shape; replaced placeholder models with live-available IDs.
7. Added behavioral tests: validateBatchLine, tabulateBatchResults, computeCosts aggregation, splitTrim, round2, headerInt, orDash, isAudioExt, expandAudioInputs.

## Ship recommendation

**ship** — all ship-threshold conditions met (shipcheck exits 0, verify PASS, dogfood clean, workflow-verify pass, verify-skill clean, scorecard 93 ≥ 65, no flagship returns wrong/empty output). Known gap: Batch API endpoints require a Developer plan to exercise live (account-level, documented above).
