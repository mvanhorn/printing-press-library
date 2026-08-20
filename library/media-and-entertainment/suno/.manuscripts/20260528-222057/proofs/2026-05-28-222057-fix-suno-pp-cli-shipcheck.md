# Suno CLI — Shipcheck Proof

## Verdict: ship

Shipcheck umbrella: **PASS (6/6 legs)**.

| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (93/100, Grade A) |

## Scorecard highlights
- Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Quality, MCP Desc, MCP Remote Transport, Local Cache, Workflows: 10/10.
- Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness: 10/10.
- Lower dims (non-blocking): MCP Token Efficiency 7, MCP Tool Design 7, Cache Freshness 5, Type Fidelity 3/5, Insight 7, Breadth 9, Vision 9.

## Fixes applied during shipcheck (2 loops)
1. `sync` — dry-run/verify short-circuit moved before the auth gate (so `sync --dry-run` is reachable without creds).
2. `generate` — `--lyrics-file` read moved after dry-run short-circuit (verify-safe).
3. `analytics` — `Use:` simplified to `analytics` (removed phantom positional inference).
4. `research.json` — recipes use real example values (not `<placeholder>`); removed side-effectful `auth login --chrome` from quickstart (auth setup lives in auth_narrative).
5. `workspace add/remove` — split from a dynamic-`Use` shared builder into two constructors with literal `Use:` strings, registered directly in workspace.go (so verify-skill's static scanner resolves the 3-level path); `Use:` trimmed to `add <workspace_id>` so `--clip` flag placeholders aren't counted as positionals.
6. SKILL.md / README.md — added `workspace add`/`remove` to the command catalog; real-valued workspace/download recipes.

## Sample Output Probe note (not blockers)
The scorecard's live probe reported 3/6 because the run had no Suno credential and an empty local DB:
- grep / sql → empty store returns `[]` (correct).
- credits --forecast → HTTP 401 (no auth).
These are environmental, not functional bugs. All six novel commands were verified against a seeded DB in Phase 3 (grep/analytics/top/sql/lineage/credits) with correct output.

## Generation captcha boundary (documented gap, by design)
`generate/extend/cover/remaster` require an hCaptcha `--token` (or `--no-captcha`) — no resident browser solver, per the replayable-HTTP rule. Documented in README Authentication + Troubleshooting and SKILL anti-triggers.

## Ship recommendation: ship
All ship-threshold conditions met. No functional bugs in shipping-scope features.
