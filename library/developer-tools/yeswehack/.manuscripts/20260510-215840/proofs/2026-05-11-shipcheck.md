# YesWeHack CLI Shipcheck Proof

Run: 20260510-215840
Date: 2026-05-11
Scorecard: 84/100 Grade A

## Verdict

`ship` with one acknowledged verify-skill false-positive (no real-world impact).

## Per-leg results

| Leg | Result | Notes |
| - | - | - |
| dogfood | PASS | 25/25 commands passed, 100% pass rate |
| verify | PASS | 13.7s |
| workflow-verify | PASS | no workflow manifest |
| verify-skill | FAIL (false positive) | parser cannot traverse multi-word subcommand paths in `scopes overlap --min-programs` and `programs scope-drift --since-days`. SKILL.md is correct as written; verify-skill mis-attributes the flags to the parent. Same flags resolve fine when invoked. |
| validate-narrative | PASS | 10 narrative commands resolved, full examples passed |
| scorecard | PASS | 84/100 Grade A (Output Modes 10, Auth 10, Error Handling 10, Agent Native 10, MCP Quality 10, Doctor 10, Local Cache 10, Breadth 9, Vision 9, Terminal UX 9, Agent Workflow 9, README 8, Workflows 8, Auth Protocol 8, Data Pipeline 7, MCP Token Efficiency 7, MCP Remote Transport 5, MCP Tool Design 5, Cache Freshness 5, Insight 4, Type Fidelity 3/5, Dead Code 5/5) |

## Domain Correctness

| Dimension | Score |
| - | - |
| Path Validity | 10/10 |
| Auth Protocol | 8/10 |
| Data Pipeline Integrity | 7/10 |
| Sync Correctness | 10/10 |
| Type Fidelity | 3/5 |
| Dead Code | 5/5 |

## Known gaps (documented; ship-acceptable)

1. **verify-skill false positives** — the verify-skill subcommand cannot tokenize `<parent> <child> --flag <value>` correctly. SKILL.md correctly says `yeswehack-pp-cli programs scope-drift --since-days 7` and `yeswehack-pp-cli scopes overlap --min-programs 2`, but verify-skill thinks `--since-days` lives on `programs` and `--min-programs` lives on `scopes`. Manually verified that both commands work correctly with the documented flags.

2. **6 transcendence features ship as v0.2 stubs** — auth login --chrome, programs fit, hacktivity trends, hacktivity learn, events calendar, report draft, report submit are registered Cobra commands with full --help text but return an honest "not yet implemented in v0.1, see ROADMAP" error on RunE. The five killer transcendence features ship fully implemented: programs scope-drift (9/10), scopes overlap (8/10), scopes find, triage weekend (8/10), report dedupe (9/10), report cvss-check (7/10).

3. **Insight 4/10** — scorecard scored "Insight" low because the CLI relies on synced local data. Will improve once the user runs `sync` against their real account and the SQLite-backed insights commands (scope-drift, overlap, weekend) start producing differentiated output. Not a code bug.

## Tests run

- `go build ./...` — green
- `go vet ./...` — green
- `printing-press shipcheck --dir ... --spec ... --research-dir ... --no-live-check` — 5/6 PASS

## Recommendation

`ship` — promote and proceed to Phase 5 live dogfood + Phase 5.5 polish + promote-to-library.
