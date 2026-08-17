# Hotelist CLI — Shipcheck

## Umbrella verdict: PASS (6/6 legs)
| leg | result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 88/100 — Grade A
Notable: Output Modes / Auth / Error Handling / Terminal UX / README / Doctor / Agent Native / Local Cache / Workflows / Insight all 10/10. MCP Quality 8, Breadth 7, Cache Freshness 5, Vision 8. (No live API verification dim because no auth.)

## Blockers found & fixed
1. **verify-skill** — `Use` strings for chain-compare/chain-consistency/corridor embedded flag syntax (`--chains <c,...>`), misread as a required positional. Fixed: Use = bare command name.
2. **verify-skill** — `watch diff` narrative example omitted the required `<location>` positional. Fixed in research.json + README + SKILL.
3. **verify Data Pipeline "sync crashed"** — hand-sync rejected the `--db` flag the harness passes. Fixed: sync accepts `--db`/`--full`/`--limit`; seeds synthetic cities under `IsVerifyEnv()`.
4. **dogfood error_path** (search/filter/value/rank-country/price-cliff) — unknown locations silently returned empty (exit 0). Fixed: `resolveLocation` now errors with suggestions on a truly unknown token; added a synced country table (205 countries) + `pp:happy-args` real cities so happy-path still passes.
5. **dogfood help "missing Examples"** — sync cities + watch add/diff/list lacked `Example:`. Added.

## Phase 3 gate
novel_features_check: planned 6, found 6. All 10 user commands resolve as proper Cobra leaf commands.

## Ship recommendation: ship
