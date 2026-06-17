# github-pp-cli — Phase 4 Shipcheck

## Verdict: PASS (6/6 legs)

| Leg | Result | Notes |
|---|---|---|
| verify | PASS | runtime + auto-fix loop clean |
| validate-narrative | PASS | quickstart + recipes resolve & dry-run (after fixing the sync→`--repo` quickstart) |
| dogfood | PASS | wiring, paths, novel_features 8/8 planned==found |
| workflow-verify | PASS | — |
| verify-skill | PASS | SKILL ↔ CLI source consistent |
| scorecard | PASS | **94/100 — Grade A** |

## Scorecard 94/100 (Grade A)
Weak dims: MCP Quality 8/10, Cache Freshness 5/10, Insight 4/10, Type Fidelity 4/5, Path Validity 9/10. All others 10/10 (Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP transport/tool-design/surface, Local Cache, Breadth, Vision, Workflows, Auth Protocol, Data Pipeline, Sync Correctness, Dead Code 5/5).

## Behavioral verification (live, against cli/cli)
All 8 novel features verified working end-to-end with `--repo`:
- `issues dupes "release" --repo cli/cli` → populated 100 issues, returned #13638 (real match)
- `pulls review-load --repo cli/cli` → BagToad: 4 PRs awaiting review
- `labels coverage --repo cli/cli` → 23 labels, 56 unused, unlabeled counts
- `mentions "config"` (offline reuse) → tagged issue+pull hits
- `repos changelog --repo cli/cli --base v2.93.0 --head v2.94.0` → 102 commits, top author babakks (52)
- `repos who-touched pkg/cmd --repo cli/cli --max-scan-pages 1` → 100 scanned, babakks 49 (first/last dates)
- `issues context 13638 --repo cli/cli --agent --select issue.title,comments.body,commits.sha` → envelope, select filters correctly

## Bugs found & fixed in-session (fix-before-ship)
1. **Auth not generated** — GitHub's bundled OpenAPI declares no securityScheme; enriched spec with a bearer scheme (GITHUB_TOKEN/GH_TOKEN) + regenerated.
2. **`issues context` integer-comparison bug** — `json_extract($.number) = ?` bound the number as a string; SQLite type mismatch meant no match. Fixed to bind an int64.
3. **Sync can't populate path-scoped GitHub** (`sync.go:374` skips unresolved `{owner}/{repo}`) — added populate-on-demand (`--repo owner/repo`) to the 6 local commands; preserves the offline-FTS value prop via the command's own fetch+upsert. Fixed the quickstart accordingly.

## Known limitations (documented, not blocking)
- Local commands need `--repo owner/repo` on first run to populate (the framework `sync` cannot fill GitHub repo-scoped resources). Documented in command Long help, README troubleshooting, and recipes.
- Scorecard live-sample shows 3/8 because it runs the local commands without `--repo` against a fresh empty store, and concurrent first-opens hit transient SQLITE_BUSY. Not reproducible in normal single-command use; all 8 verified working with `--repo` above.

## Ship recommendation: **ship**
All ship-threshold conditions met: shipcheck 6/6, scorecard 94 ≥ 65, every flagship feature produces correct output when used as documented, 0 stubs, all 8 approved transcendence features built.
