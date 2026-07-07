# AutoTempest CLI — Shipcheck Report

## Verdict: ship (pending Phase 5 live dogfood + polish)

## Shipcheck umbrella (6/6 legs PASS)
| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative (--strict --full-examples) | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

Re-run clean after the slug-normalization fix — still 6/6.

## Scorecard: 80/100, Grade A
Perfect (10/10): Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Desc Quality, MCP Remote Transport, Local Cache, Workflows, Sync Correctness, Dead Code (5/5).
Strong: MCP Quality 9, Agent Workflow 9, Breadth 7, Vision 7, Data Pipeline 7, MCP Token Efficiency 7, Type Fidelity 4/5.
Gaps (Phase 5.5 polish targets): Insight 4/10, Path Validity 4/10, Cache Freshness 5/10.

## Behavioral correctness (verified live against autotempest.com)
- `find "honda civic" --zip 33701 --radius 200` → 20 real Civic listings, prices parsed to cents, VINs, source codes. (token signing works live)
- `find "toyota tacoma"` → 12 listings; `find "ford f-150"` → 15–20 F-150 listings (post slug fix).
- `dedupe` → 50 VIN groups w/ per-source price arrays.
- `deal "Camry"` → 50 rows with computed deal_scores (e.g. 20% under $23,695 median).
- `spread "f-150"` → 5 source rows (min/median/max per marketplace).
- `watch add/ls/run` → registers + replays live.
- `sources` → 9 sources. `makes`/`models` → live reference data.
- Missing-mirror guard: novel commands on empty store → `[]` + sync hint, exit 0 (no crash).

### Scorecard "Sample Output Probe" 2/6 — explained, not a bug
The probe runs local-store commands (drops/deal/spread/watch) COLD with example queries (Camry/F-150/civic-fl) against an empty store, so they correctly return empty/missing-mirror hints. These same commands return correct populated results once `find`/`watch run` has synced matching data (proven live above). This is the known stateful-local-command probe limitation; Phase 5 dogfood (populate → query) is the authoritative behavioral gate.

## Top blockers found + fixed
1. Model-slug hyphen handling — `find "ford f-150"` sent unrecognized slug `f-150` (AutoTempest uses `f150`) → 0 results. Fixed: `NormalizeSlug` (strip non-alphanumerics) applied to make/model params + local filters. Re-verified live.
2. Generated novel stubs were TODO placeholders → implemented all 6 with real store-query logic.
3. queue-results is async (status:1 + empty until backend populates) → added polling. st/fbm divergent envelopes → best-effort, tagged in fetch_failures.

## Reachability
Browser-free plain HTTP, no auth, no Cloudflare. Token computed natively (SHA-256 + salt extracted from site bundle, verified against live server). Risk: if AutoTempest rotates the salt or changes the endpoint contract, `find`/`watch run` break until re-cracked — documented in README.
