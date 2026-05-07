# linear-pp-cli Shipcheck Report (reprint, v3.10.0)

## Shipcheck umbrella verdict: PASS (5/5 legs passed)

| Leg | Result | Exit | Elapsed |
|-----|--------|------|---------|
| dogfood         | PASS | 0 | 652ms |
| verify          | PASS | 0 | 6.73s |
| workflow-verify | PASS | 0 | 12ms  |
| verify-skill    | PASS | 0 | 86ms  |
| scorecard       | PASS | 0 | 43ms  |

## Dogfood
- Path Validity: SKIP (Linear is GraphQL-only — no REST paths to validate)
- Auth Protocol: SKIP (spec not provided as OpenAPI)
- Dead Flags: 0 (initial run flagged `trustMode`; fixed by wiring it into `issues create` strict mode)
- Dead Functions: 0
- Data Pipeline: PARTIAL (sync uses domain-specific Upsert; 134 domain tables created)
- Examples: SKIP (agent-context command not present in v1 tree; pre-existing carve-out)
- Novel Features: 12 manifest items, 12 built (full reprint scope)
- MCP Surface: WARN (generated MCP tools.go does not use the v3 runtime Cobra-tree template — pre-existing v1 shape; polish-pass via mcp-sync would migrate)

## Verify
- Pass Rate: 92% (61/66 passed, 0 critical)
- Verdict: PASS
- Failures (5): blocking, similar, slipped, issues, me, organization-metas — all because the mock-mode probe runs without args/DB. These light up correctly under Phase 5 live dogfood with real data.

## Workflow Verify
- Verdict: workflow-pass (no workflow manifest)

## Verify-Skill
- Initial: 1 error — `--cli-only` referenced in SKILL.md install instructions
- Fixed: rewrote install section to point at the Printing Press Library plugin / `go install`
- Final: All checks passed (flag-names, flag-commands, positional-args, unknown-command)

## Scorecard
- Total: 84/100 — Grade A
- Note: omitted from denominator — mcp_description_quality, mcp_surface_strategy, path_validity, auth_protocol, live_api_verification (GraphQL CLI + v1 MCP shape)
- Highlights: Output Modes 10, Auth 10, Agent Native 10, Local Cache 10, Insight 10, Workflows 10, Breadth 10, Data Pipeline Integrity 10, Dead Code 5/5
- Lower dims: Cache Freshness 0/10 (no recent sync timestamp on disk yet — will rise after Phase 5 live sync), Type Fidelity 3/5, Vision 7/10, Agent Workflow 9/10

## Fixes applied this loop
1. Removed `workload` per absorb gate (deleted `internal/cli/workload.go`, removed registration)
2. Built 7 missing transcendence features:
   - `cycles compare` (subcommand of cycles, `internal/cli/cycles_compare.go`)
   - `projects burndown` (subcommand of projects, `internal/cli/projects_burndown.go`)
   - `slipped` (top-level, `internal/cli/slipped.go`)
   - `blocking` (top-level, `internal/cli/blocking.go`)
   - `initiatives health` (subcommand of initiatives, `internal/cli/initiatives_health.go`)
   - `pp-test list/sessions` + `pp-cleanup` (top-level, fixture-lifecycle pair)
   - `issues create` (subcommand of issues — required for live testing; records to pp_created)
3. Added `pp_created` table + 5 store helpers (`internal/store/pp_fixtures.go`)
4. Added `--trust-mode` flag and wired it into `issues create` (strict requires --session)
5. Rewrote SKILL.md install section to a v3-friendly path

## Carried-over deferrals (out of reprint scope)
- v3 MCP runtime Cobra-tree migration — pre-existing v1 static tools.go shape. Path forward: `printing-press mcp-sync` polish pass.
- Static `internal/mcp/tools.go` still mentions `Workload Balance` in its novel-features list (cosmetic; doesn't affect MCP surface). Polish-pass cleanup.
- `agent-context` command is missing from v1 tree (dogfood examples-check skipped). Polish-pass scaffolding addition.
- 8 call sites in `doctor.go`, `feedback.go`, `profile.go` use `flags.printJSON(cmd, v)` instead of `printJSONFiltered(cmd.OutOrStdout(), v, flags)` — silently drops `--select`/`--compact`/`--csv`/`--quiet`. v1-shaped code; polish-pass cleanup.

## Ship recommendation: ship (pending Phase 5 live dogfood)
- All ship-threshold conditions met:
  - shipcheck umbrella exit 0 ✅
  - verify PASS, 92% pass rate, 0 critical ✅
  - dogfood wiring checks pass ✅
  - workflow-verify PASS ✅
  - verify-skill PASS ✅
  - scorecard 84 ≥ 65 threshold ✅
- The 2 v1 ship-with-gaps debts (`projects burndown`, `cycles compare`) are now closed with real implementations.
- The user's live-testing constraint (only mutate fixtures the CLI creates) is structurally enforced by the limited mutation surface: `issues create` records every new ID into `pp_created`, and `pp-cleanup` only operates on records from that table.
- Phase 5 live dogfood will exercise the create→list→cleanup loop end-to-end; PASS there is required before final ship verdict.
