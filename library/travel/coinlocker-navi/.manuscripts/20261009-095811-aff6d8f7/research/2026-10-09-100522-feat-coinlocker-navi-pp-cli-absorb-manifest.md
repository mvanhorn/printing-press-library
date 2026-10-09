# coinlocker-navi absorb manifest

Sources: coinlocker-navi.com (primary, HTML replay) and api.multiecube.com (secondary, live vacancy only). No competing CLI, MCP server, SDK, or skill exists (searched GitHub, npm, PyPI, MCP directories 2026-10-09). Only community code: alltheplaces spider multi_ecube_jp.py (confirms Ekicube params).

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Keyword locker search (station/area name) | coinlocker-navi.com /search | coinlocker-navi-pp-cli near <keyword> | Normalized JSON: sizes S/M/L/XL with yen and box count, IC-card list, hours, change machine, explicit null for 情報なし, source URL + fetched_at |
| 2 | Nearest lockers to coordinates | coinlocker-navi.com POST /search/gps/nearest_cl | (behavior in coinlocker-navi-pp-cli near) --lat/--lon path | Distance-sorted, no phone GPS needed; filters --size, --ic, --walk-up-only (excludes bookable ecbo cloak rows) |
| 3 | Locker detail page | coinlocker-navi.com /cl/<id> | coinlocker-navi-pp-cli locker <id> | Full normalized record + 5 neighbours with distance + Google Maps link |
| 4 | Area / station browse pages | coinlocker-navi.com /<pref>/area/, /<pref>/eki/ | (behavior in coinlocker-navi-pp-cli near) keyword path covers station/area names | No separate browse tree (less is more) |
| 5 | Live vacancy map Maihama | coinlocker-navi.com /areamap/maihamaeki | coinlocker-navi-pp-cli vacancy --station maihama | Per-size installed vs empty counts with source as-of time |
| 6 | Multi Ekicube live per-size empties | api.multiecube.com GET /v1/location/ph2 | (behavior in coinlocker-navi-pp-cli vacancy) <keyword> or --lat/--lon path | Inside/outside gate, English + Japanese names, size filter, usage fee vs ¥500 reservation fee split, caveat "0 reservable != full" |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Live join on nearest lockers | near --live | 9/10 | hand-code | coinlocker-navi list rows joined to Multi Ekicube ph2 (nearest bank within 30 m, labelled as possibly a neighbouring bank) and Maihama ajax blocks (exact /cl/<id>); attaches live{source,observed_at,source_as_of,match_basis,ekicube|maihama} | Brief thesis + User Vision ("include real-time vacancy"); verified Ekicube JSON; site FAQ confirms realtime pages | Use this command for walk-up lockers near a place or coordinates, with live empties attached where a source publishes them. Do NOT use this command for a full live board of one vacancy-publishing station; use 'vacancy' instead. |
| 2 | Hours-window filter | near --open-during HH:MM-HH:MM [--strict] | 8/10 | hand-code | Parses 利用時間 (clock ranges, 24時間, 始発～終電) from list rows and Ekicube business_hours; unknown hours kept with open_during:"unknown" (and train-hours rows with "train_hours") unless --strict | User Vision "what hours"; 利用時間 often 情報なし | none |
| 3 | Gate side across both sources | near --gate inside|outside | 8/10 | hand-code | Ekicube inside/outside_ticket_gate copied only when exactly one Ekicube bank lies within 15 m; no text guessing; otherwise gate=null, gate_source=null; --gate never shows unknown-gate rows as matches (they are excluded and counted in meta.excluded.gate_unknown) | User Vision "inside or outside the ticket gates" | none |

## Gate decision (Phase 8)
Approved 2026-10-09 by the orchestrator (user delegated absorb-gate approval): option 1 with recommended cuts.
- Commands: near (<keyword> | --lat/--lon; --size, --ic, --walk-up-only), locker <id>, vacancy (Multi Ekicube keyword or --lat/--lon, plus --station maihama).
- CUT: hankyu-umeda vacancy (brittle template).
- Novel flags on near: --live, --open-during HH:MM-HH:MM [--strict], --gate inside|outside.
- N3 gate only from Multi Ekicube structured fields; unknowns never shown as matching.
- Honesty rules: "0 reservable != full" warning on every Ekicube count; fetched_at plus "source gives no record date"; ~1 req/s pacing; on-demand only, no bulk sync (site terms).
- Hand-code: 6 of 6, no stubs.
