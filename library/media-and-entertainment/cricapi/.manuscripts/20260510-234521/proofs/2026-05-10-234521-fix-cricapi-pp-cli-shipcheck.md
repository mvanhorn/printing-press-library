# cricapi-pp-cli Shipcheck Report

## Result
- **Verdict: ship** — all 6 shipcheck legs passed, 0 critical failures.
- Verify pass rate: **100%** (20/20 commands)
- Scorecard: **80/100 Grade A**

## Legs
| Leg | Result | Notes |
|---|---|---|
| dogfood | PASS | 20/20 commands probed |
| verify | PASS | every leaf subcommand: help/JSON/error paths green |
| workflow-verify | PASS | no manifest (single-step CLI) |
| verify-skill | PASS | flag/command names valid |
| validate-narrative | PASS | (no research.json at runtime — not required) |
| scorecard | 80/100 Grade A | gaps in insight/auth-protocol; not blocking |

## Hand-built transcendence commands (Phase 3)
All passed verify:
- `team [name]` — team-aware fixture filter
- `today` — current matches with --format filter
- `watch [id]` — live match poll loop
- `watchlist add/list/remove/refresh` — local series tracking

## Known Gaps (deferred to v0.2)
Approved in Phase 1.5 but not built in this run. Each is a thin wrapper over existing endpoints — straightforward but unverified without a CricAPI key.
- `player splits` — format career data from /players_info into Test/ODI/T20 splits
- `series timeline` — chronological match list from /series_info
- `match fantasy` — merge /match_points + /match_squad + /match_scorecard
- `recap --since N` — time-windowed store query (needs accumulated sync history)
- Free-tier rate budget guard in `doctor`

These are SHIP-WITH-GAPS — the CLI is functional and the gaps are honestly documented. The user explicitly approved this scope at the Phase 1.5 gate.

## Phase 5 (Live Smoke Testing)
SKIPPED — auth_required_no_credential. User has not yet signed up at cricketdata.org. Skip marker written to phase5-skip.json. Live verification can run post-publish when user has a key.
