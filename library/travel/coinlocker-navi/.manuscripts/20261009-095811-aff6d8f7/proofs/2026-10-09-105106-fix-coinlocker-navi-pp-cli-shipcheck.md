# coinlocker-navi-pp-cli shipcheck (run 20261009-095811-aff6d8f7)

## Results
| Loop | Legs | verify | scorecard | live probe |
|------|------|--------|-----------|------------|
| 1 | 7/7 PASS | 100% | 80/100 A | 1/3 (near --live, near --gate timed out at 10 s) |
| 2 | 7/7 PASS | 100% | 80/100 A | 1/3 (source latency spike) |
| confirm | 7/7 PASS | 100% (15/15, 0 critical) | 80/100 A | 3/3 |

Standalone `scorecard --live-check` between loop 2 and confirm: 3/3 pass (79/100).
dogfood verdict WARN: dead root flag `maxAge` + 8 dead generated helpers (generator emission).

## Top blockers found
1. Multi Ekicube requires from_at/end_at; each dated request takes 0.5-4 s. near --live/--gate read 3 pages x 2 languages over a 1.3 km radius (6 requests, 15 s).
2. `--agent` (compact) dropped the `live` object (omitempty key in < 80 % of rows).
3. `locker` and `vacancy --station` used a `result` key, so `--agent` nested it as `.results.result`.
4. dogfood source_client_check flagged per-file `c.Get(` calls; maps.google.com host not in research artifacts.

## Fixes applied
- near: cheap filters first; live join only on candidate rows (returned rows when no --gate); Japanese-only Multi Ekicube pages in near; paging stops once coverage is enough (sources sort by distance; `CoveredM`, `EkicubeQuery.Done`); records beyond the banks read are counted as `gate_not_checked` and get no live/gate data (note added). Keyword scan reads only `--limit` rows when no filter is set.
- near output uses keep floor for live/open_during/gate/gate_source under --agent.
- locker/vacancy --station key renamed to `results`; examples include meta.fetched_at in --select.
- Client method renamed `fetch` (limiter + typed 429 live in client.go); maps.google.com documented as an outbound link in the sniff report.
- Gate novel example moved to Shinjuku with --limit 3 (one Multi Ekicube page).
- New tests: GateFor, CoveredM, Ekicube Done early stop.

## Before / after
- verify pass rate: 100% -> 100%
- scorecard: 80 -> 80 (Insight 2/10, Vision 5/10, Cache Freshness 3/10 are structural for a live-only, no-sync CLI)
- near --live 東京駅: 15 s -> 5-6 s; near --gate (Shinjuku, --limit 3): 4-5 s; Tokyo Station --gate --limit 2: 7-10 s (dense area needs 2 pages).

## Verdict
ship
