# Food52 Shipcheck Report

**Run:** 20260501-164925 (reprint of food52, v2.3.10 → v3.2.1)
**CLI:** food52-pp-cli
**Verdict:** **PASS — 5/5 legs**

## Summary

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| dogfood | PASS | 0 | 2.3s |
| verify | PASS | 0 | 3.3s |
| workflow-verify | PASS | 0 | 14ms |
| verify-skill | PASS | 0 | 257ms |
| scorecard | PASS | 0 | 32ms |

## Scorecard

**Total: 84/100 — Grade A** (up from 79/100 Grade B in the prior reprint).

| Dimension | Score | Δ |
|---|---|---|
| Output Modes | 10/10 | — |
| Auth | 10/10 | — |
| Error Handling | 10/10 | — |
| Terminal UX | 9/10 | — |
| README | 8/10 | — |
| Doctor | 10/10 | — |
| Agent Native | 10/10 | — |
| MCP Quality | 8/10 | — |
| MCP Token Efficiency | 4/10 | gap |
| MCP Remote Transport | 5/10 | gap |
| Local Cache | 10/10 | — |
| Cache Freshness | 5/10 | gap |
| Breadth | 7/10 | — |
| Vision | 8/10 | — |
| Workflows | 8/10 | — |
| Insight | 4/10 | gap |
| Agent Workflow | 9/10 | — |
| Data Pipeline Integrity | 10/10 | — |
| Sync Correctness | 10/10 | — |
| Type Fidelity | 3/5 | — |
| Dead Code | 4/5 | improved (1 vs 30) |

## Verify

- **Pass rate: 94%** (16/17 passed, 0 critical) — up from prior 83%.
- Two harness-side false positives remain: `print FAIL` and `which FAIL` in EXEC mode (require positional args — retro F7's verify-mock-mode dispatcher gap is unfixed in v3.2.1; not behavioral failures, won't block ship).

## Dogfood

- 0 invalid paths.
- 0 dead flags.
- **1 dead helper** (`extractResponseData` in helpers.go) — generated, unused. Polish will cleanup.
- **0 source-client violations** — `internal/food52/typesense.go` and `internal/food52/discovery.go` route through `doWithLimiter` (cliutil.AdaptiveLimiter + typed cliutil.RateLimitError on retry exhaustion).
- 7/7 novel features survived sync to `novel_features_built` (retro F4 fix confirmed working).
- Examples: 10/10 commands have examples (retro F2 fix confirmed working — `Example: strings.Trim(`...`, "\n")`).
- MCP surface mirrors the Cobra tree at runtime via cobratree.

## Verify-Skill

- All checks pass: flag-names, flag-commands, positional-args, unknown-command.

## What changed from the prior reprint (v2.3.10 vs v3.2.1)

| Retro finding (prior) | Status this reprint |
|---|---|
| F1: HTML extractor only had `mode: page`/`links` — Next.js `__NEXT_DATA__` required hand-replacing 4 generated handlers | **Resolved.** All 4 SSR-backed handlers (recipes browse/get, articles browse/get) use generator-emitted `mode: embedded-json` + `json_path` walk. The hand-built `internal/food52/nextdata.go` is no longer load-bearing for the typed endpoint surface. |
| F2: `Example: strings.TrimSpace(...)` silently broke dogfood example detection | **Resolved.** All hand-written commands use `strings.Trim(\`...\`, "\n")` (preserves indent); generator templates use the safe form too. Example detection: 10/10. |
| F3: Side-effect commands (`open`) launched browser during verify mock-mode | **Resolved.** `open` follows the print-by-default convention plus `cliutil.IsVerifyEnv()` short-circuit. |
| F4: `.printing-press.json` missing `novel_features` after generation | **Resolved.** Dogfood synced 7/7 novel_features into the manifest automatically. |
| F5: `traffic-analysis.json` schema rejected `browser_http` mode + string evidence | **Resolved.** Browser_http accepted; EvidenceRef.UnmarshalJSON accepts strings via sentinel index. (Other shape changes encountered: `protections[].notes` is now `[]string`, `generation_hints` is `[]string`, warnings are objects, auth uses `candidates` not `candidate_types`, version is `1` not `1.0`. Adapted prior traffic-analysis.json to the new schema during this run.) |
| F6: 30 dead helpers when handlers were hand-replaced | **Largely resolved.** Down to 1 dead helper (`extractResponseData`). The 30→1 drop is the direct result of F1: handlers no longer need replacement, so generator-emitted helpers stay used. |
| F7: Verify mock-mode dispatched required-positional commands as failures | **Unfixed.** 2 false positives (`print`, `which`) in EXEC mode. Doesn't block ship. |

## Ship-threshold checks

- ✅ shipcheck umbrella exits 0
- ✅ verify PASS, 0 critical failures
- ✅ dogfood PASS (no spec parsing/binary-path/skipped-examples failures)
- ✅ wiring checks pass
- ✅ workflow-verify workflow-pass (no manifest)
- ✅ verify-skill exits 0
- ✅ scorecard 84/100, no flagship feature returns wrong/empty output (re-validated by smoke-testing recipes browse, recipes search, recipes top, recipes get, articles browse, tags list, open against live food52.com)

## Verdict

**ship.** All ship-threshold conditions met. No known functional bugs in shipping-scope features. Polish phase will pick up the lone dead helper and any cosmetic README polish.
