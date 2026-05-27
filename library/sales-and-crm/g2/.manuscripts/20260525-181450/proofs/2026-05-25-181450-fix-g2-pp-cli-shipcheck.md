# g2-pp-cli Shipcheck Report

Run id: `20260525-181450`
Press version: `4.15.0`

## Verdict: PASS

All 6 shipcheck legs passed. CLI is shippable. After Phase 5.5 polish, dogfood verdict transitioned from FAIL (OAuth scope coverage 0/29) to PASS (29/29). No verdict override.

## Leg results

| Leg | Result | Exit | Notes |
|-----|--------|------|-------|
| verify | PASS | 0 | 100% pass rate (28/28 commands) |
| validate-narrative | PASS | 0 | 11 narrative commands resolved, full examples passed (after research.json sync example fix from `--resources reviews` → `--resources products,products_reviews`) |
| dogfood | PASS | 0 | Wiring, dead-flag, examples, novel-features (9/9), MCP surface checks all clean |
| workflow-verify | PASS | 0 | No workflow manifest defined; skip is acceptable |
| verify-skill | PASS | 0 | SKILL.md flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command, canonical-sections all clean |
| scorecard | PASS | 0 | 90/100 Grade A initially; 91/100 after polish |

## Scorecard breakdown (post-polish)

| Dimension | Score |
|---|---|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 10/10 |
| README | 10/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 10/10 |
| MCP Desc Quality | 10/10 (was 9/10) |
| MCP Remote Transport | 10/10 |
| MCP Token Efficiency | 4/10 (gap — 51 endpoint tools; spec-level fix) |
| MCP Tool Design | 5/10 (gap — same root cause) |
| MCP Surface Strategy | 2/10 (gap — same root cause) |
| Local Cache | 10/10 |
| Cache Freshness | 5/10 (generator-template default) |
| Breadth | 10/10 |
| Vision | 9/10 |
| Workflows | 10/10 |
| Insight | 7/10 |
| Agent Workflow | 9/10 |

Domain Correctness:
| Dimension | Score |
|---|---|
| Path Validity | 10/10 |
| Auth Protocol | 10/10 |
| Data Pipeline Integrity | 10/10 |
| Sync Correctness | 10/10 |
| Live API Verification | N/A (no API key) |
| Type Fidelity | 3/5 |
| Dead Code | 5/5 |

Total: **91/100 — Grade A** (after polish)

## Sample Output Probe

7/9 passed on live novel-feature commands. Two failures, both non-verdict-affecting:

- **Multi-product review FTS**: exit 4 with 401 Bad Credentials. Expected — no G2 API key in environment.
- **Alt-track**: output doesn't echo the input product slug when local store is empty. Cosmetic; logged as Phase 4.85 finding for downstream polish.

## Fixes applied during shipcheck loop

1. Fixed `narrative.quickstart` example: `sync --resources reviews` → `sync --resources products,products_reviews` (reviews is a hierarchical child of products in the G2 v2 spec; standalone sync fails the parent-cascade rule). Final research.json line 235.

## Known gaps (documented for transparency)

1. **MCP token efficiency / surface strategy** — 51 endpoint tools is above the recommended 50-tool threshold. The fix is spec-level (`x-mcp.orchestration: code`, `endpoint_tools: hidden` at the OpenAPI root) plus regeneration. Polish flagged as out of CLI-level scope.
2. **Cache freshness / type fidelity** — generator template defaults; not closable without upstream cli-printing-press improvements.
3. **Phase 5 live smoke skipped** — G2 Marketplace API requires paid Sell-tier subscription, which the operator declined to provide at Phase 0. Structural verification only.

## Ship recommendation

`ship`. All ship-threshold conditions met:
- Shipcheck exit 0, all legs PASS
- Verify ≥ 86%, 0 critical failures
- Dogfood wiring + novel-features clean
- workflow-verify: pass
- verify-skill: clean
- scorecard 91 ≥ 65, no flagship feature broken
