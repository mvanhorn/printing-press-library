# OpenArt CLI - Shipcheck Proof

**Run ID:** 20260513-152641
**Verdict:** PASS (6/6 legs)
**Scorecard:** 81/100 - Grade A

## Leg-by-leg

| Leg | Result | Notes |
|---|---|---|
| dogfood | PASS | 0 wiring violations |
| verify | PASS | 25/25 commands (HELP/DRY-RUN/EXEC), 100% pass rate. `forms` EXEC scored 2/3 (mutating endpoint, non-blocking). |
| workflow-verify | PASS | No workflow manifest declared (CLI is not workflow-shaped) |
| verify-skill | PASS | Round 1 caught --type and --by mismatches (fixed), round 2 clean. |
| validate-narrative | PASS | All 10 narrative examples (5 quickstart + 5 recipes) resolve and dry-run cleanly. |
| scorecard | PASS | 81/100 Grade A. Sample-probe 7/9 — 2 failures are auth-required commands hitting 401 (resolves in Phase 5 dogfood once cookies are imported). |

## Fixes applied during shipcheck loops

- Renamed local `truncate` to `truncateUtf8` to avoid colliding with framework helpers.
- Removed unused custom `max()` and `sumSpent()` helpers.
- Added `--type` alias (sharing `--family` semantics) on `models cheapest` so the example in research.json resolves.
- Added `--by spend|count|recency` flag to `prompts top` so the example resolves.
- Renamed `media stats` → `stats` in research.json to match the actually-shipped command.
- Replaced `auth login --chrome` quickstart entry with a hermetic `version` step + prose guidance, since `auth login --chrome` is interactive and can't be dry-run-validated.
- Replaced `prompts replay <fixture-id>` recipe with a hermetic `models cheapest` recipe to avoid depending on local store state.
- Replaced aspirational `--prompt-file` and `--notify-on-done` recipe with the real `video gen --notify` shape.
- Reshaped `gaps` array from `[{name,description}]` objects to plain strings (matches the scorecard's expected schema).

## Sample output probe summary

7/9 novel-feature examples ran successfully against an unauthenticated session. Two failed:

- `credits forecast` (auth required for `/user/my-info`)
- `compare` (auth required for `/projects/default`)

These are expected and will resolve in Phase 5 once `auth login --chrome` has imported the user's OpenArt session cookies.

## Outstanding gaps (not blocking)

- **MCP remote transport: 5/10** - Default stdio-only. Adding HTTP transport would require enabling `mcp.transport: [stdio, http]` in the spec; deferred to v0.2 since the user's first need is local invocation.
- **MCP tool design: 5/10** - Could be improved by declaring multi-step intents in the spec. Deferred for the same reason.
- **Insight: 7/10** - The headline `compare` and `cost estimate` are insight-shaped; the score will rise once Phase 5 confirms they produce useful output against real data.
- **Type Fidelity: 3/5** - One naming violation in the generated `internal/cli/user_info.go` (verb `info` instead of `get`). Filed as retro candidate for the generator (machine bug, not a printed-CLI fix).
- **Auth Protocol: 5/10** - The cookie-auth path scores lower than API-key-bearer, but it's the only auth OpenArt provides; expected.

## Verdict

**ship** - All ship-threshold conditions met. No known functional bugs in shipping-scope features. Ready for Phase 5 live dogfood.
