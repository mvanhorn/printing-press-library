# Instagram CLI Shipcheck

## Shipcheck umbrella (v4.20.1) — final
| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

Verdict: **PASS (6/6)**. Scorecard **94/100 — Grade A**. Sample-output probe 7/7 (100%).

## Blockers found + fixed
1. validate-narrative FAIL: quickstart used `sync --resources media,account-insights` (framework sync can't sync path-positional per-account resources) and a recipe used `media list --account` (generated media list takes a positional ig_user_id, not --account). Root cause: Instagram's per-account path-scoped API doesn't fit the flat framework sync model. Fix: rewrote quickstart to the real workflow (`brands discover` → `pull` → analytics) and the recipe to the positional `media list <ig_user_id>` form; mirrored fixes into README.md + SKILL.md. Re-validate: 10/10 narrative commands resolved.

## Behavioral correctness (seeded-store demo, every novel feature)
- compare: ranked by engagement-rate desc (brand-a > brand-b > brand-c (real values redacted)) ✓
- growth: brand-a +850 (+8.5%), weeks_covered 6 ✓ (after timestamp-parse fix)
- top-posts: REELS 41000 > FEED 30000 > FEED 5000; low post last ✓
- formats: FEED avg_reach 17500 / REELS 41000 + watch-time 15 ✓
- best-time: hour 9, 2 posts, avg 2500 ✓
- rivals: follower_change +900 ✓
- hashtag-perf: #coffee 900 > #latte 300 ✓

## Scorecard soft gaps (non-blocking)
- insight 4/10, cache-freshness 5/10 (cache intentionally disabled — snapshots are user-built working state), MCP token efficiency 7/10. None block ship.

## Verdict: ship
