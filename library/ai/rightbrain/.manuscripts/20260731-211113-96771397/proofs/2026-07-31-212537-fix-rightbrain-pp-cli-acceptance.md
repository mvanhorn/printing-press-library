# Rightbrain CLI — Phase 5 acceptance report

    Acceptance Report: rightbrain
      Level: Quick Check (binary matrix) + a manual read-only live sweep
      Tests: 13/13 passed (binary matrix), plus the manual sweep below
      Gate: PASS

## Scope constraint

The operator supplied an OAuth access token and explicitly chose a **read-only**
sweep: no creates, no deletes, no task or agent runs, no credit spend. The
binary's `--level full` matrix walks every leaf subcommand including mutating
ones, so it was deliberately not run. `--level quick` was used instead, and its
executed set was audited afterwards to confirm the constraint held:

    3 error_path   (invalid-argument probes)
    2 happy_path   (analytics, approvals — both read-only)
    6 help
    2 json_fidelity

No create, update, delete, or run operation executed against the workspace.

## Binary matrix (dogfood --live --level quick)

`status: pass`, `matrix_size: 13`, `passed: 13`, `failed: 0`. Marker written by
the runner to `phase5-acceptance.json` (not hand-authored).

## Manual read-only live sweep

Run against the operator's real workspace. Identity fields redacted here.

| Check | Result |
|---|---|
| `doctor` | All green: config ok, auth configured, env vars 1/1, API reachable, credentials valid |
| `whoami` | 200, resolved the authenticated viewer and their org/project |
| `project task list` | 200, empty result set (workspace has no tasks) — empty envelope rendered correctly, not an error |
| `project task-agent list` | 200, **2 real agents returned** |
| `sync` (unfiltered) | 28 records across 35 resources; `org`, `skills` (7), `whoami` all persisted |
| `sync --resources project_task_agent` (filtered, fresh DB) | 0 records — see finding below |
| `changelog --since 30d` | **Real audit events from the live workspace**, UUIDs resolved to names |
| `approvals` | Live call; honest empty result with an explanatory note |
| `drift --since 7d` | Ran against the live-synced mirror |
| `search "agent"` | Correctly reported no server-side search endpoint and fell back to local data |
| `scope show` / `scope use` / `scope clear` | Resolution order verified: flag > env > saved file |

## Behavioral verification against a synthetic store

Because the live workspace has no task-run or eval history, the analytic
commands were additionally proven against a seeded local mirror with known
values:

- `drift` — mean credits 5.00 -> 11.67 reported as `+133.334%`; p95 2s -> 12s as
  `+500%`; a below-threshold group correctly excluded with `filtered_out: 1`; a
  group with no previous window reported `new: true` with `null` deltas rather
  than `+Inf`.
- `changelog` — known UUIDs resolved to names, and an unknown UUID fell back to
  the bare id with `resolved: false` rather than inventing a name.
- `eval-flake` — a case that passed then failed on the same revision classified
  `flaky`; a case failing every run classified `consistent-failure`; a case
  passing every run classified `stable-pass`.

Negative and guard paths: a nonexistent task returns an empty result with a
note (no fabrication); `--data-source live` on the local-only commands exits 2;
a missing mirror prints a stderr hint, emits `[]`, and exits 0; a missing
required positional exits 2.

## Finding raised during the sweep (fixed)

**Filtered first sync silently returns nothing.** `sync --resources project_task,project_task_run`
on a fresh machine reported `success, 0 records` because every Rightbrain
resource is parent-keyed on `org` -> `project`, and a filtered sync does not
cascade upward to populate those parents. The documented remedy is to run an
unfiltered `sync` first. The README/SKILL quickstart previously showed the
filtered form as step one, which would have sent every new user straight into a
silent empty state; it now shows plain `sync`.

## Printing Press issues for retro

Recorded separately in `retro-candidates.md` (6 items, including a confirmed
data race in generated framework code and a `generate --force` pass that
reverted implemented novel commands).

Gate: **PASS**
