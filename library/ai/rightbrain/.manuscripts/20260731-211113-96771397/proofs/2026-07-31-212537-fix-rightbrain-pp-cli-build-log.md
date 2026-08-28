# Rightbrain CLI — Phase 3 build log

Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

## Priority 0 — foundation
- `internal/cli/rightbrain_scope.go` (hand-authored, markerless): org/project scope
  resolution + injection. Every Rightbrain path is `/org/{org_id}/project/{project_id}/...`,
  so all 300+ generated endpoint commands took two UUIDs positionally. Resolution order:
  `--org-id`/`--project-id` -> `RB_ORG_ID`/`RB_PROJECT_ID` -> saved `scope.json`.
  Adds `scope show|use|clear`. Injection is additive — a fully-specified invocation is
  passed through untouched. Verified across five cases (config, env, flag, explicit
  positionals, extra positional).

## Priority 2 — transcendence (7 rows)

Built directly:
- `approvals` (live) — project-scoped approval list + computed parked age, time-to-expiry,
  and the actionable/expired split, sorted by urgency. 5 test functions incl. two
  absence-of-correctness cases (empty window is honest; unparseable timestamp reads as
  "unknown" instead of sorting to the top as 1970).
- `agent-trace` (auto) — pairs tool_call with tool_result across the flat event array,
  derives per-step and per-tool durations, ranks a tool histogram by wall clock. Resolves
  the owning agent from the local mirror so only a run id is needed. 5 test functions incl.
  an unanswered-call case that must not fabricate a duration.

Delegated (one file each, strict contract, behavioral acceptance tests required):
`gate`, `rollout`, `drift`, `changelog`, `eval-flake`.
