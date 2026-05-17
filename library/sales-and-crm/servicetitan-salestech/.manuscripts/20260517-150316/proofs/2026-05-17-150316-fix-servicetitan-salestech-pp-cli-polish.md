# servicetitan-salestech-pp-cli Polish Report

**Run via:** Skill `printing-press-polish` (forked context, non-standalone — main SKILL owns publish).
**Verdict:** `ship-with-gaps`

## Delta

|  | Before | After | Delta |
|---|---|---|---|
| Scorecard | 84/100 | 84/100 | 0 (MCP Desc Quality moved N/A → 10/10 but was not counted in denominator before) |
| Verify | 100% | 100% | 0 |
| Dogfood | PASS | PASS | — |
| Go vet | 0 | 0 | 0 |
| Tools-audit pending | 4 | 0 | −4 |
| PII-audit | clean | clean | — |
| Verify-skill | PASS | PASS | — |
| Workflow-verify | PASS | PASS | — |
| Publish-validate | FAIL (phase5 + manifest) | FAIL (phase5 only) | manifest now PASS |

## Fixes Applied

1. **`printing-press mcp-sync`** generated the previously-missing `tools-manifest.json`. publish-validate's manifest check now PASS.
2. **`mcp-descriptions.json`** written with rich agent-grade overrides for `estimates_dismiss_estimates` and `estimates_unsell_estimates` — the spec descriptions for these endpoints were empty and the auto-generated text was just the operationId. MCP Desc Quality bumped from N/A → 10/10.
3. **4 thin-short findings accepted** on the DO-NOT-EDIT generated parent groupers (`dismiss`, `items`, `sell`, `unsell`) with distinct per-finding rationales flagging the generator-template gap (parentNoSubcommandRunE groupers should be exempted from description-length checks or get richer defaults). Tagged as retro-candidate for the Printing Press itself.
4. **mcp-sync re-run** after writing overrides so the descriptions propagate into `tools-manifest.json` and `internal/mcp/tools.go`.

## Skipped Findings (documented, not addressed by polish)

- **`mcp_token_efficiency 0/10`** — structural: the threshold calibration assumes 50+ endpoint APIs where `surface_strategy=hidden` + `orchestration=code` materially reduce per-turn token cost. With 13 endpoints, the runtime-walking surface is already small (~13 tools) and adding the explicit `mcp` orchestration block would not behaviorally change the result. The spec already declares `x-mcp: {transport: [stdio, http], orchestration: code, endpoint_tools: hidden}`. Polish would be cargo-cult here.
- **`type_fidelity 1/5`** — structural Steinberger dim assessing the generated CLI's response-type round-tripping. Not addressable from polish without re-running the generator.
- **`cache_freshness 5/10`, `breadth 5/10`** — small surface area; polish can't move these without adding fake commands.
- **publish-validate `phase5`** — mid-pipeline structural artifact: `phase5-acceptance.json` already exists at the run-state proofs dir; the parent pipeline's promote step (Phase 5.6) relocates it to the published CLI's `.manuscripts/<run>/proofs/` path where publish-validate expects it. Not a CLI defect.
- **publish-validate `manuscripts` WARN** — same structural concern; manuscripts are populated by Phase 5.6 promote/archive.

## Remaining Issues

None on the CLI side. Phase 5.6 promote will resolve the publish-validate `phase5` + `manuscripts` artifacts by relocating the existing run-state files into the published `.manuscripts/` location.

## Ship Recommendation

`ship-with-gaps` — the CLI passes every behavioral and structural check that polish can verify. The remaining publish-validate failure is structural pipeline plumbing that Phase 5.6 owns. `further_polish_recommended: no` per polish's own assessment: another polish pass would not resolve the structural issues.

## Polish Skill Notes

- Linter (gofmt) normalized formatting in `internal/cli/estimates_import.go` and `internal/salestech/{entities.go, reports.go, csvimport.go, salestech_test.go, health.go}`. Behavioral semantics unchanged; build + tests still PASS.
- The polish skill's Publish Offer was correctly suppressed because it was invoked via Skill-tool without `--standalone` — the main `/printing-press` SKILL owns the publish flow at Phase 6 (or post-publish via `/printing-press-publish`).
