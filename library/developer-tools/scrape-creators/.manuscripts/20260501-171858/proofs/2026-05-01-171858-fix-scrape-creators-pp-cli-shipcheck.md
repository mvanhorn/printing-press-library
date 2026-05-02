# Shipcheck Report — scrape-creators-pp-cli

## Run summary

- Spec: `https://docs.scrapecreators.com/openapi.json` (saved to `spec.json`).
- Working dir: `$CLI_WORK_DIR`.
- Run: `2026-05-01-171858`.

## Verdict

**Shipcheck: PASS (5/5 legs passed).** Grade A, 84/100 scorecard.

| Leg | Verdict | Notes |
|---|---|---|
| dogfood | PASS | All endpoint commands wire correctly; no stale MCP surface; novel features detected. |
| verify | PASS | 98 % pass rate, 42/43 commands; only WARN was the `which` command's runtime probe — not a critical failure. |
| workflow-verify | PASS | No workflow manifest configured; skipped per default. |
| verify-skill | PASS | Initial run flagged `sync mrbeast --platform tiktok` examples; fixed README + SKILL to use `sync --resources tiktok`. |
| scorecard | PASS | 84/100 Grade A. |

## Scorecard breakdown

```
Output Modes         10/10
Auth                 10/10
Error Handling       10/10
Terminal UX           9/10
README                8/10
Doctor               10/10
Agent Native         10/10
MCP Quality          10/10
MCP Token Efficiency  7/10
MCP Remote Transport  5/10
MCP Tool Design       5/10
MCP Surface Strategy  2/10
Local Cache          10/10
Cache Freshness       5/10
Breadth              10/10
Vision                9/10
Workflows            10/10
Insight               8/10
Agent Workflow        9/10

Domain Correctness
Path Validity           10/10
Auth Protocol            8/10
Data Pipeline Integrity  7/10
Sync Correctness        10/10
Type Fidelity            3/5
Dead Code                5/5

Total: 84/100 - Grade A
```

## Top blockers found

1. **Bad sync invocations in narrative.** `sync mrbeast --platform tiktok` was claimed in quickstart and recipes but `sync` accepts neither a positional handle nor a `--platform` flag. Fixed by replacing with `sync --resources <platform>` everywhere (research.json + SKILL.md + README.md).

## Fixes applied

- Restored `Copyright 2026 adrian-horning` in `NOTICE` (preserving original ScrapeCreators CEO attribution).
- Updated `research.json`, `SKILL.md`, and `README.md` to use `sync --resources <p>` instead of the invented `sync <handle> --platform <p>` pattern.

## Before / after

- verify pass rate: 98% (42/43, 0 critical) — unchanged through the fix loop, was already healthy.
- scorecard total: 84/100 Grade A.

## Final ship recommendation

**ship** — all five shipcheck legs pass; Grade A; no known functional bugs in shipping-scope features. Phase 5 live dogfood will exercise commands against the real API to confirm behavioral correctness.

Notable scorecard gaps surfaced as polish opportunities (not blockers): MCP Surface Strategy (2/10), MCP Remote Transport (5/10), MCP Tool Design (5/10), Cache Freshness (5/10), Type Fidelity (3/5). The polish skill in Phase 5.5 will address the high-leverage ones.
