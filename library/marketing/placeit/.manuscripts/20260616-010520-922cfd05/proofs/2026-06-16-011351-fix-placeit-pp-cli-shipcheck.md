# Placeit CLI — Shipcheck Report

## Verdict: ship

## Shipcheck umbrella: PASS (6/6 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS — 80/100, Grade A |

## Live output probe: 6/7 (86%)
- top, pod, kit, industry-map, search, watch: pass (all work cold via live Algolia).
- gaps: empty cold — by design (whole-catalog pivot requires a synced mirror; prints a clear "run sync" hint + [] with exit 0). Not a broken flagship.

## Fixes applied during shipcheck
1. **verify-skill FAIL → PASS**: `gaps` Use string had `<a> <b>` angle brackets parsed as required positionals → changed to `Use: "gaps"`. Auth/sync commands referenced inline in README/SKILL prose (single-quoted) tripped the parser → reworded `research.json` auth_narrative + troubleshoots to keep runnable commands in code blocks only, removed a bogus `--live` flag reference (real flag is `--data-source live`).
2. **Live probe 2/7 → 6/7**: made `top`/`pod`/`kit`/`industry-map` live-first (Algolia best-selling replica / printify filter / live classify / live drill-down) so they work with no prior sync; `--data-source local` still reads the offline mirror.

## Scorecard breakdown (80/100, Grade A)
- 10/10: Output Modes, Auth, Error Handling, README, Doctor, Agent Native, MCP Desc/Token/Remote, Local Cache, Workflows.
- Weak (polish targets): auth_protocol 2/10 (cookie auth on an API-key-oriented rubric), insight 4/10, vision 6/10, cache_freshness 5/10, breadth 7/10, MCP partial-readiness (account/bookmarks auth-gated).

## Known Gaps (documented, non-blocking)
- **Editor render/download** is browser-only (async blender render); shipped as `open` deep-link resolver, not a fake render command.
- **gaps/rank** require a synced mirror (whole-catalog analytics); both print clear sync hints when cold.
- **Brand-kit** (absorbed row 16) is an explicit out-of-scope stub (needs editor session).

## Behavioral correctness: VERIFIED against real Algolia + mirror
All 7 novel features produce correct real output (purchase ranking, printify filter, kit slot coverage, taxonomy counts, watch diff, facet pivot, percentile).
