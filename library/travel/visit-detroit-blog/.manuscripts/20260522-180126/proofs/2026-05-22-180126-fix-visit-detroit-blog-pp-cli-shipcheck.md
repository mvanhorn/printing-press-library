# Inside the D CLI — Shipcheck

## Verdict: ship

`printing-press shipcheck` → **PASS (6/6 legs)**.

| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | runtime + auto-fix loop, no critical failures |
| validate-narrative | PASS | all quickstart/recipe commands resolve |
| dogfood | WARN→PASS leg | 1 dead flag + 12 dead helpers (orphaned generated API-response helpers); examples 9/9, novel features 4/4 |
| workflow-verify | PASS (workflow-pass) | no workflow manifest (single-entity CLI) |
| verify-skill | PASS | SKILL flags/commands/sections all honest |
| scorecard | PASS | **66/100 Grade B** |

## Scorecard breakdown (66/100)
Strong: Output Modes 10, Auth 10, Error Handling 10, Doctor 10, Agent Native 10, Local Cache 10, Sync Correctness 10, MCP Remote Transport 10, Agent Workflow 9, Terminal UX 8, README 8, MCP Quality 8.
Soft (inherent to a focused single-entity content CLI): Breadth 6, Vision 6, Workflows 6, Insight 7, Cache Freshness 5, MCP Token Efficiency 7.
Gaps:
- **dead_code 0/5** — 12 orphaned generated helpers + `allowPartialFailure` flag, dead because the endpoint-mirror command was replaced with store-backed commands. → Phase 5.5 polish removes these (expected +5).
- **path_validity 0/10** — structural: this CLI has zero endpoint-mirror commands (every command reads the local store), so the path validator has nothing to validate. Documented characteristic of an all-store-backed CLI, not a defect.
- type_fidelity 3/5 — related to the dead code; should improve after polish.

## Behavioral correctness (verified live, after `sync`)
All 4 novel features + absorbed commands produce correct output against the real Algolia-synced store:
- `sync` → 748 articles; `categories` Dining=288 (matches live facet exactly).
- `search`, cross-axis `blogs list`, `blogs get`, `blogs related` (scored), `blogs coverage`, `blogs reading-list` (md/file), `recent`, `regions` — all correct.
- Error paths: not-found exit 3, usage exit 2, no-arg help, dry-run exit 0.

## Scorecard live probe: 3/4 (documented limitation)
The scorecard `--live-check` Sample Output Probe runs novel-feature examples in an **isolated, unsynced store**, so store-backed features can't be relevance-checked without a sync step the probe doesn't perform (AGENTS.md: "search and sql return empty until sync"). `blogs related donuts` returns an empty (composable) result + a "run sync first" hint there. Real behavior is correct (verified above). **Retro candidate:** scorecard live-probe should sync store-backed CLIs before sampling, or tolerate empty/not-synced results for store-backed novel features.

## Before/after
- First generation → shipcheck PASS 6/6, scorecard 66.
- Fix loop 1: fixed `loadAllBlogs` truncation bug (List default LIMIT 200 → all 748); added empty-store hints; `blogs related` returns composable empty result on unsynced store. Shipcheck re-run → PASS 6/6, scorecard 66.

## Deferred to Phase 5.5 polish
- Remove 12 dead generated helpers + `allowPartialFailure` flag (→ dead_code 5/5, type_fidelity up).
- Assess path_validity / soft dimensions.
