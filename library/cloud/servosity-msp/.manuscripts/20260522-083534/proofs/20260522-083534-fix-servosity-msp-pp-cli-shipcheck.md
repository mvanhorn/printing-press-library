# Phase 4 Shipcheck — servosity-msp-pp-cli

## Verdict: PASS (6/6 legs)

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| verify | PASS | 0 | 11.5s |
| validate-narrative | PASS | 0 | 0.45s |
| dogfood | PASS | 0 | 2.5s |
| workflow-verify | PASS (no manifest) | 0 | 0.01s |
| verify-skill | PASS | 0 | 2.0s |
| scorecard | PASS | 0 | 0.25s |

## Scorecard: 86/100 Grade A

**Strong (10/10):** Output Modes, Auth, Error Handling, Doctor, Agent Native, MCP Quality, Local Cache, Workflows, Insight, Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness

**Mid (5-9/10):** Terminal UX 8, README 8, MCP Token Efficiency 7, Cache Freshness 5, Breadth 8, Vision 8, Agent Workflow 9, Dead Code 5/5

**Gaps (Grade A despite):**
- **Type Fidelity 1/5** — type generation depth could be improved; v0.2 polish item
- **MCP Surface Strategy 2/10** — 293 tools is over the 50-tool threshold; would benefit from `mcp.orchestration: code` + hidden endpoint tools (Cloudflare pattern). Documented as v0.2 spec enrichment.
- **MCP Remote Transport 5/10** — currently stdio-only; could add HTTP transport for remote agents

## Verify-skill PASS — SKILL.md is honest

Every flag and command path in SKILL.md resolves against the shipped CLI source. The auto-generated SKILL.md narrative (driven by `research.json`) is mechanically consistent with what was actually built.

## Validate-narrative PASS — 10/10 examples

Every `narrative.quickstart` and `narrative.recipes` command resolves and executes successfully under `PRINTING_PRESS_VERIFY=1 --dry-run`. Including:
- `attention --top 10`
- `qbr 4421 --quarter 2026-Q1 --format pdf --out acme-q1-backup.pdf`
- `bill --reconcile invoiced-2026-05.csv --month 2026-05 --json` (after the dry-run ordering fix below)
- `restore-queue watch --interval 30s --json`

## Fixes applied during shipcheck

1. **bill_reconcile.go dry-run ordering** — Initial pass had the `--reconcile` file-existence check before the `dryRunOK(flags)` short-circuit, breaking the verify-friendly contract. Moved file check after dry-run check. One-line edit.
2. **research.json copied to working dir** — The umbrella's validate-narrative loads `<dir>/research.json` (in addition to the run-state copy); explicit copy added so both paths resolve to the same file.
3. **Top-level binary location** — Umbrella validate-narrative expects the binary at `<dir>/<binary-name>`, not `<dir>/build/stage/bin/<binary-name>`. Built to both locations.
4. **Pure-logic package tests** — Added `internal/snapshot/snapshot_test.go` (6 tests, table-driven happy paths + edge cases) and `internal/qbrreport/qbrreport_test.go` (5 tests covering RenderMarkdown / RenderHTML escape safety / ParseQuarter / CurrentQuarter). All pass.

## Known gaps documented in research.json novel_features

None — all 10 transcendence features built; v0.2 enhancements (charts in QBR PDF, restore-queue persistence, RMM bridge) are NOT in the v1 absorb manifest so they don't count as gaps.

## Ship recommendation

**Verdict: ship**, conditional on Phase 5 dogfood when an API key is available. The CLI builds, all quality gates pass, transcendence features all resolve cleanly under dry-run, and the narrative is honest.
