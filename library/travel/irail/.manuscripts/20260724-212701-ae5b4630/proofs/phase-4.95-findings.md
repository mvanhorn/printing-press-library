# Phase 4.95 — Local Code Review (irail-pp-cli)

Review path: inline review by the orchestrating agent. Reviewer-subagent dispatch
was unavailable: this session's harness configuration explicitly forbids calling
the Agent tool unless the user requests it. Scope, checks and fix policy below
match the reviewer-persona contract (correctness, security, maintainability).

Scope: internal/cli/ (hand-authored files), internal/store/irail_migrations.go,
internal/irailref/. Out of scope: internal/cliutil/, internal/mcp/cobratree/.

## Autofix summary
2 findings autofixed in-place across 1 round.

1. **Panic on missing timestamps (correctness, high).** Five call sites across
   changes.go, leave_by.go and transfer_risk.go sliced an RFC3339 string at a
   fixed `[11:16]` offset to render HH:MM. `unixToLocal` returns "" whenever
   iRail omits a time field, which its own open issues confirm happens on the
   connections endpoint. Reproduced the panic directly
   (`slice bounds out of range [:16] with length 0`). Fixed by routing every
   human render through a new `clockOf` helper that returns "--:--" for short
   input. Regression test added (TestClockOfSurvivesMissingTimes).
2. **Duplicate command registration (correctness, medium).** `stations facilities`
   and `disruptions route` appeared twice under Available Commands because the
   generator already attaches them to promoted parents. Removed from the local
   novel-command hook.

## Verified clean
- SQL: every query in the hand-authored store layer and novel commands is
  parameterised; no string concatenation of user input into SQL.
- Bounds: all `args[0]` reads sit behind an explicit length check; no remaining
  fixed-offset string indexing.
- Resources: `rows.Close()`, `rows.Err()` and `db.Close()` handled on every path;
  drain-first ordering respected so no follow-up query runs on an open `*sql.Rows`.
- Arithmetic: the punctuality average is guarded against a zero denominator.
- Partial failures: `observe` keeps per-target fetch errors out of recorded totals
  and reports them via `fetch_failures` plus a stderr warning.
- govulncheck: PASS (via shipcheck).
- gosec: not installed on this host; not run.

## Template-shape retro candidates (not fixed in place)
1. `internal/cli/sync.go` builds its query map from scratch and never applies
   spec param defaults, so any API requiring a constant query parameter
   (here `format=json`) silently syncs the wrong content type. Worked around by
   pinning the parameter into each endpoint path.
2. Cache-freshness pre-read assumes every resource is syncable; resources with
   required parameters produce upstream 400/500 before the command runs.
3. `root.go` renders the CLI title from the slug ("Irail") rather than the
   research-authored `narrative.display_name` ("iRail").

## Surface-to-user findings
None. Both in-scope findings were mechanical and behaviour-preserving.

## Convergence
Findings cleared at round 1. Build, vet and the full test suite pass.
