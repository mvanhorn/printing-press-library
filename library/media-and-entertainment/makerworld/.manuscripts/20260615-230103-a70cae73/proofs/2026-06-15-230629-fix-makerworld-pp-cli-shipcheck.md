# MakerWorld CLI — Shipcheck Proof

## Verdict: ship

## Shipcheck legs (6/6 PASS)
| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS (after fixing discover `--category` example) |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS (after fixing discover `--category` example) |
| scorecard | PASS — 95/100 Grade A |

## Scorecard highlights
Output Modes 10, Auth 10, Error Handling 10, README 10, Doctor 10, Agent Native 10,
MCP Quality 10, MCP Remote Transport 10, Local Cache 10, Workflows 10. Total 95/100 Grade A.
Lower dims: MCP Token Efficiency 7, Cache Freshness 5 (no pre-read refresh), Breadth 7, Insight 7.

## Fixes applied during shipcheck
1. **discover `--category` example** — research.json auto-generated a `discover --category category_800` example, but discover has no `--category` flag (synced list rows carry no per-design category). Removed from research.json, README.md, SKILL.md. Fixed 4 verify-skill errors + 1 live-probe failure. (CLI fix.)
2. **Stale "remix lineage" headline** — root.go/tools.go/manifest/.printing-press.json carried the pre-swap headline; replaced with "multi-tag discovery" and fixed the mangled MCP playbook insight. (CLI fix.)

## Known non-blocking note
- Scorecard live-probe `tags toy fidget` reports "output does not contain fidget" because the live-check runs the local-mirror `tags` command without syncing first → graceful empty. Verified working against a real mirror (toy∩fidget → 4 real matches). Inherent to live-checking local-analytics commands; scorecard passed regardless.

## Behavioral verification (live, against api.bambulab.com)
- Absorbed: designs list/get/search/related/remixes/ratings/recommend, designers models, categories list — all return real data.
- Transcendence: discover (quality/popular/printable + live AMS enrichment), movers (+500 delta proven via injected snapshot), designers deltas (Δlike/Δdl per creator), tags (cloud + toy∩fidget intersection).
- Token-gated download + favorites: graceful exit 0 with clear hint when MAKERWORLD_TOKEN absent.

## Printing Press issues for retro
- None blocking. Minor: narrative auto-example generation invented a `--category` flag not present on the hand-coded command (research.json novel-feature examples should validate against final flags).
