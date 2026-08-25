# Peekaboo CLI Shipcheck

## Shipcheck umbrella (final)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative (strict, full-examples) | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS — 89/100 Grade A |

Verdict: PASS (7/7 legs)

## Scorecard highlights
- Output Modes 10, Auth 10, Error Handling 10, README 10, Doctor 10, Agent Native 10,
  MCP Remote Transport 10, Local Cache 10, Workflows 10, Insight 10.
- Cache Freshness 3/10 (no upstream refresh path for the live deal commands — intentional; not a stateful-mirror CLI).
- MCP: 9 tools, all auth-required (guest token auto-bootstrapped); readiness full, MCPB bundled.

## Blockers found + fixed (2 fix loops)
1. verify-skill FAIL: novel command `Use:` strings embedded `<value>` after flags
   (e.g. `wallet <bank> --city <city> --category <id>`), which the parser read as 3
   positionals. Fixed: `Use:` now lists only real positionals (`wallet <bank>`), Cobra
   appends `[flags]`.
2. verify-skill FAIL: README referenced a `sync` command + local-SQLite mirror that this
   CLI does not have (novel commands are live fan-out). Fixed research.json narrative
   (headline, value_prop, rationales, troubleshoots) to describe the live approach and
   removed all sync/mirror references; regenerated.

## Behavioral correctness (ship-threshold flagship check) — all verified live
- directions 13 --city lahore -> 4 Kababjees branches w/ coords + Google Maps URLs ✓
- nearest 13 --near lahore -> closest branch + distance + directions URL ✓
- wallet hbl --city lahore --category 1 -> 4 merchants honoring HBL card deals ✓
- top-deals --city lahore --category 1 -> deals ranked, biggest 50% ✓
- expiring --city lahore --category 1 --within 60d -> 17 deals w/ days-left ✓
- open-now --city lahore --category 1 -> 37/50 merchants open ✓
- deals list --target-entity-id 13 -> deals returned (associatedDeals fix) ✓
- branches 609 / cards 13 / categories / locations / brands -> all return correct data ✓

## Ship recommendation: ship
