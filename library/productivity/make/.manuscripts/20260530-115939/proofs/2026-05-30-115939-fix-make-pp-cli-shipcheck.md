# make-pp-cli Shipcheck Proof

## Pass 1 verdict: FAIL (validate-narrative)

3 broken examples in `research.json`:
1. `sync --team <teamId>` — global `sync` does not take `--team`
2. `scenarios list --all-teams --active` — absorbed `list` does not have `--all-teams`; that lives on the sibling `list-all`
3. `connections audit --expiring 7d` — Go duration parser rejects `7d`; use `168h`

## Fix applied

Edited `research.json` narrative:
- `sync --team <teamId>` → `sync` (the global sync walks all configured teams)
- `scenarios list --all-teams --active --select id,name,team,lastEdit` → `scenarios list-all --active --json --select rows.id,rows.name,rows.teamId,rows.lastEdit`
- `connections audit --expiring 7d --errored 7d --json --select id,name,expire,errorCount` → `connections audit --all-teams --expiring 168h --unused --json --select rows.id,rows.name,rows.expire,rows.issues`

`validate-narrative --strict --full-examples` now reports `OK: 11 narrative commands resolved and full examples passed`.

## Pass 2 verdict: PASS (6/6 legs)

| Leg | Result | Elapsed |
|-----|--------|---------|
| verify | PASS | 11.8s |
| validate-narrative | PASS | 1.9s |
| dogfood | PASS | 2.7s |
| workflow-verify | PASS (no manifest needed) | 0.1s |
| verify-skill | PASS | 0.7s |
| scorecard | PASS | 0.6s |

## Scorecard

**94/100 - Grade A**

Perfect on: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Remote Transport, MCP Tool Design, MCP Surface Strategy, Local Cache, Breadth, Vision, Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness, Dead Code.

Sub-perfect:
- MCP Quality: 8/10
- Cache Freshness: 5/10 (acceptable default)
- Workflows: 8/10
- Insight: 4/10 (scorecard "gap" — flagged as needing improvement; the data layer + cross-team joins satisfy the design intent, but the scorecard's heuristic is shallow here)
- Agent Workflow: 9/10
- Type Fidelity: 4/5

## Dogfood signal

- 8/8 novel features built (synced into README.md `## Unique Features` and SKILL.md `## Unique Capabilities`)
- 10/10 commands have realistic examples
- 0 dead flags, 0 dead functions
- Auth protocol matches spec (`Token` prefix)
- Data pipeline: GOOD (calls domain-specific Upsert + Search)

## Final ship recommendation: **ship**

All shipcheck ship-threshold conditions met. Behavioral correctness for transcendence features will be verified against the live API in Phase 5.
