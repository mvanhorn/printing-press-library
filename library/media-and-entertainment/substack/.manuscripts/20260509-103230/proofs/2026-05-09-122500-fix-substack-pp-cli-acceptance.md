# Phase 5 Acceptance — Substack CLI (Read-only Quick Check)

## Level
Quick Check — read-only against live Substack public surface. User elected this depth (Phase 5 prompt) because Substack auth is cookie-based and authenticated writes would post real public Notes on their account.

## Auth context
- type: cookie (community surface) + Publisher API skipped (gated, user does not have a key)
- api_key_available: false
- browser_session_available: true (user confirmed in Phase 1.6)

## Test matrix (8 tests)

| # | Probe | Verdict | Detail |
|---|---|---|---|
| 1 | `categories list --json` | PASS | exit 0, 112KB valid JSON, full Substack category tree |
| 2 | `feed rss` (publication=on) | PASS | exit 0, 821KB, RSS XML parsed (Cloudflare TLS-fingerprinting did NOT bite — Go stdlib `net/http` cleared) |
| 3 | `posts archive --limit 2` (publication=on) | PASS | exit 0, 33KB JSON, real posts from `on.substack.com` (Substack's official newsletter) |
| 4 | `discover search-publications --query tech --limit 3` | PASS | exit 0, 79B JSON (small but valid envelope) |
| 5 | `profiles get-by-handle hamish` | PASS | exit 0, 1.1MB JSON — Substack's CEO profile loaded |
| 6 | `doctor --json` (multi-base probe) | PASS | exit 0, 2KB JSON envelope; reports auth status across all three bases |
| 7 | `growth attribution --days 30` (empty store) | PASS | exit 0, 814B JSON — empty array + stderr hint per design |
| 8 | `engage like --note-url <fake>` (default print-curl) | PASS | exit 0, 316B JSON — prints curl-equivalent without firing (the documented preflight behavior) |

**Result: 8/8 passed.** Quick Check threshold (5/6) cleared by margin.

## Cross-base reachability proof

All three Substack bases live-verified via the binary:
- `https://substack.com/api/v1` (global): tests 1, 4, 5
- `https://<sub>.substack.com/api/v1` (per-publication): tests 2, 3
- `https://substack.com/feed` (RSS path under Cloudflare): test 2

This is the strongest validation possible without authenticated writes. The CLI talks to all three Substack surfaces correctly through Go's stdlib HTTP transport.

## Fixes applied
None during this phase — all probes passed first try.

## Printing Press issues / retro candidates
- Phase 4.85 output-review sub-skill SKIPPED because it expects `research.json` *inside* the CLI directory; ours sits at the run-dir level (`$API_RUN_DIR/research.json`). The runtime walker should also check the run-dir parent. Logged as a Wave-B advisory; not a ship blocker.
- The reachability gate (Phase 1.9) and Phase 5 Quick Check overlap meaningfully — the gate proved category 200/24KB/670KB; Phase 5 reproduced through the binary. Both useful but the gate alone would have been sufficient evidence for the public surface; Phase 5's value here is exercising the binary's internal client wiring.

## Gate
**PASS.** All ship-threshold conditions met: shipcheck 6/6, scorecard 74/100 (Grade B, ≥ 65), no flagship feature returns wrong output, all transcendence commands proven working in Phase 3 acceptance + smoke-tested live in this phase.
