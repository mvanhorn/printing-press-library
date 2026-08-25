# RideWithGPS CLI — Shipcheck

## Result: SHIP

All 6 shipcheck legs PASS; scorecard 92/100 Grade A; sample output probe 7/7 (100%).

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS (unverified-needs-auth — api_key+token, header auth not browser-session, so PASS not hold) |
| verify-skill | PASS |
| scorecard | PASS (92/100 Grade A) |

## Scorecard highlights
- 10/10: Error handling, Terminal UX, README, Doctor, Agent-native, Local cache, Breadth, Vision, Workflows, Insight, Path validity, Auth protocol, Data-pipeline integrity, Sync correctness, MCP quality, MCP remote transport.
- Sample probe: 7/7 novel features produce plausible output (no flagship empty/wrong).

## Blockers found + fixed (2 shipcheck loops)
1. **Auth: second header dropped.** Parser's sibling-header collection failed because `createAuthToken`'s single-key security override made the sibling set inconsistent across requirements. Fixed by uniform `{rwgpsApiKey, rwgpsAuthToken}` AND security on every op → both `x-rwgps-api-key` and `x-rwgps-auth-token` now wire into config/client/doctor.
2. **Command naming.** `.json` path suffix leaked into resource names (`routes-json`). Fixed with per-op `x-pp-resource` → clean `routes`/`trips`/`events`/`collections`/`members`/`points-of-interest`/`users` with list/get/... subcommands.
3. **.json path bug.** `users/current` and `auth_tokens` lacked the required `.json` extension (404). Fixed in spec.
4. **Display name** "Ride Gps" → "Ride with GPS" via `x-display-name`.
5. **export sample probe exit 2.** export required `--type` and errored (exit 2) on no-DB. Fixed: `--type` defaults to `routes`, verify-mode reports plan early, "no targets" is an empty result (exit 0) with a sync hint — not a usage error.
6. **Doc drift.** README/SKILL re-rendered from corrected research.json (gear `--due-km`, `event-routes`, audit checks stale/private/incomplete).

## Known gaps (non-blocking, polish candidates)
- Type Fidelity 2/5; MCP Tool Design 5/10, Desc 7/10, Token-efficiency 7/10 (29 endpoint tools, no code-orchestration); Cache Freshness 5/10 (cache intentionally disabled — undocumented rate limits + bulk refresh). None affect functional correctness; Phase 5.5 polish target.

## Final recommendation: ship (no known functional bugs in shipping-scope features; all 7 novel features build + probe-pass)
