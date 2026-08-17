# Novel Features Brainstorm — gl-inet (audit trail)

## Customer model
- **P1 The Venue Hopper** — lands at hotel/cafe/abroad, needs router online fast; headline pain = venue WiFi invisible because its channel is forbidden under current regdomain (US hides EU 2.4GHz ch12/13). Wants one command per arrival.
- **P2 The Profile Keeper** (user's verbatim #1) — keeps known-good "home" config, mutates per venue, wants clean revert + option-level "what did this venue change" + safe restore that won't brick a different firmware.
- **P3 The Cloud-Averse Power User** — distrusts GoodCloud, wants scriptable local-only control across all 43 GL modules + the UCI tree, lives in `uci get/set` and raw `rpc/ubus call`, runs from agent/MCP, verifies VPN egress changed.

## Survivors (≥5/10) → transcendence rows
1. `snapshot save <name>` (10) — UCI export + GL module configs → config_snapshots w/ provenance stamp.
2. `snapshot apply <name>` / `snapshot revert` (10) — version+model-gated restore (warn on fw/luci drift, refuse on model mismatch w/o --force).
3. `snapshot diff <a> [<b>]` (10) — option-level diff, snapshot vs current / vs snapshot.
4. `config summary` (9) — structured per-subsystem current-config report.
5. `wifi region diagnose` / `wifi region set <CC>` (10) — scan vs country→allowed-channels table; name permitting country; set+commit+reload (Italy fix).
6. `venue connect <ssid>` (10) — scan→region-check→join→captive-portal prep→restore macro.
7. `vpn toggle <tunnel>` (10) — start + killswitch + egress-IP verify, typed exit on leak.
8. `doctor` (10) — model/fw/openwrt/luci + reachable surfaces + per-feature availability.
9. `rpc call` / `ubus call` / `uci get|set` (9) — raw authed passthrough escape hatch.
10. `config find <term>` (7) — FTS option search across whole UCI tree + GL modules.
11. `wan mode <ethernet|repeater|tethering>` (6) — WAN source switch + reconnect verify.

## Killed candidates
- `clients new` rogue detection (4) — speculative; single-user travel router, can't see venue clients in repeater mode.
- `status history` trends (3) — needs scheduled polling / background daemon (scope creep).
- captive-portal auto-login (4) — external per-venue form, not generically automatable/testable (prep steps survive inside `venue connect`).
- scheduled/auto firmware upgrade (4) — needs background process; risky unattended; upgrade already absorbed.
- speed test/benchmark (3) — no backing endpoint; needs external service; unverifiable in dogfood.
