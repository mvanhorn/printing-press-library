# HotelTonight CLI — Shipcheck

## Final verdict: ship

`printing-press shipcheck` — **6/6 legs PASS**:

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (83/100, Grade A) |

- Sample Output Probe (live novel-feature sampling): **6/6 (100%)**.
- dogfood `novel_features_check`: planned 6, found 6, none missing.

## Blockers found and fixed (2 fix loops)
1. **`why_we_like_it` type mismatch** — the field is an object (`{title, line_items}`), not a string. Fixed the struct + flattened line items.
2. **Cross-market numeric type variance** — `deal_id`/`id`/lat/lng/prices are string in some markets, number in others. Switched all upstream numerics to `json.Number` + `numF`/`numI` helpers. Caught by datescan against Austin.
3. **Daily Drop extraction** (user-requested) — the deal is hidden behind an account-gated unlock and absent from the anonymous feed; switched to brace-counting the embedded `featured_deal` JSON on the SSR results page and resolving the hotel name by id. Fixed startDate (calendar today, not the API's lagging `current_day`) and the nested-array brace bug.
4. **verify-skill / validate-narrative doc bugs** — quickstart used `sync --latitude` (sync takes no geo flags) and recipes used `inventory search` (promoted command is `inventory`, no `search` subcommand). Fixed in research.json, README, and SKILL.

## Machine gap (retro candidate)
- Scorecard's live-check resolves the binary from `build/stage/bin/` first, but hand-built novel commands only land in the root binary after a manual `go build`. The staged binary stayed stale (from the original `generate --validate`), so the live probe reported every novel command as "unknown command" until I rebuilt `build/stage/bin/`. The skill/generator should rebuild the staged binary after Phase 3 hand-coding, or live-check should prefer the freshest binary.

## Behavioral correctness (live, anonymous)
Every base + novel command verified against the real API:
- `markets list/get/nearby`, `inventory` — correct live data, header-only.
- `deals --category luxe,hip --sort discount` — Argonaut 68% off ($188 vs $587), tier filter works.
- `compare-neighborhoods --metro 1` — 6 SF neighborhoods ranked by median.
- `datescan --metro 72` — Austin tonight/tomorrow/weekend, $84 min.
- `watch --metro 1 --below 150` — 3 hits flagged below threshold.
- `history` / `verdict` — read the local time-series; verdict classifies correctly (and returns "typical" on no-spread, not a false "cheap").
- `daily-drop --metro 1` — reveals the hidden Daily Drop (e.g. Hotel Griffon, $137 was $195, 30% off) + `--history`.

## Scorecard gaps (non-blocking)
- Workflows 4/10 — no multi-step `workflow` definitions (acceptable for a read-only price-intelligence CLI).
- Cache Freshness 5/10, Type Fidelity 3/5 — minor; polish candidates.
