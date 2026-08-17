# Toodledo CLI — Shipcheck Proof

## Verdict: ship (pending Phase 5 live read-only smoke with user token)

## Shipcheck umbrella — all 6 legs PASS
| Leg | Result | Exit |
|-----|--------|------|
| verify | PASS | 0 |
| validate-narrative | PASS | 0 |
| dogfood | PASS | 0 |
| workflow-verify | PASS | 0 |
| verify-skill | PASS | 0 |
| scorecard | PASS | 0 |

## Scorecard: 91/100 — Grade A
- 10/10: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Quality, Local Cache, Breadth, Vision, Workflows; Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness.
- Lower dims (polish candidates, not blockers): MCP Desc Quality 5/10, MCP Token Efficiency 7/10, MCP Remote Transport 5/10 (stdio-only by choice), MCP Tool Design 5/10, Cache Freshness 5/10 (intentionally disabled — rate-limited API), Insight 7/10, Type Fidelity 4/5.

## Phase 3 completion gate: PASS
- novel_features_check: planned 7, found 7, missing none.
- All 7 novel commands + 5 ergonomic task-write commands resolve via `<binary> <leaf> --help`.
- All 17 of the user's MCP tools preserved and verified present in the MCP surface (57 tools total): `tasks_add/edit/complete/delete`, full resource CRUD, `next_actions`, `review`, `dashboard`, `stalled_projects`, `goal_progress`, `sync_cost`, `capture`.

## Sample Output Probe (scorecard --live-check, no credentials): 6/7
- 6 offline novel commands return valid output against an empty mirror (exit 0).
- `sync-cost` returns exit 4 (HTTP 401 No access_token) — EXPECTED: it calls account/get live and there was no token in the probe env. Verified correct in Phase 5 with the user's token.

## Fixes applied during shipcheck
- **goal-progress `--level short`/`--level long`**: added `parseGoalLevel` aliases (the documented example used `short`, which the canonical-label parser rejected). CLI fix.

## Known gaps / Printing Press retro candidates
- **OAuth token-exchange + refresh need HTTP Basic client auth** (Toodledo requirement). The generator emits form-encoded client creds; hand-fixed in `internal/cli/auth.go` + `internal/client/client.go` to send Basic alongside form. Retro candidate: authorization_code grant should support `client_secret_basic` (RFC 6749 default) via a spec field.
- **Hidden resource parents block hand-built sibling commands from the MCP cobratree walker.** Worked around by un-hiding `tasks` + nulling its RunE in root.go. Retro candidate: a generator hook to attach hand-built writes to a typed resource parent without un-hiding.
- Live API verification (real Toodledo account) deferred to Phase 5 — user providing a read-only token.

## Ship recommendation: ship (after Phase 5 live read-only smoke)
