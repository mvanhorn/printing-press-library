# Phase 4.95 local code review: gusto-pp-cli

- Review path: direct subagent dispatch (feature-dev:code-reviewer persona), then `/simplify` (4 parallel cleanup agents: reuse, simplification, efficiency, altitude). The run dir is not a git repo, so there are no commit hashes; the diffs are in the working tree.
- Autofix summary: round 1 fixed its findings in place (stale review totals, strict employee matching, WritableErr guard, mileage upsert, unlisted-employee mismatch, dry-run bootstrap guard, read-back errors, prep pre-checks). Round 2 found 4 more (rate shown rounded in the mileage description; hours save zeroing double-overtime and missed-break hours; any "Mileage ..." description treated as the mileage line; `reimburse list` hiding a bad --employee) plus 2 hardening items (prep rejects one employee under two spellings; timesheet rejects negative values). All fixed with tests.
- Convergence outcome: findings cleared at round 3.
- /simplify applied: removed dead `MileagePrefix` and `API.SessionID`; one `upsert` loop behind `UpsertLine`/`UpsertMileage`/`UpsertReimbursement`; `SameDescription` replaces repeated case-folding; `gusto.Money` replaces two formatters; `API.Review(calculate)` replaces two branches; `loadTimesheet` shared by verify and prep; compare reuses the history it loaded (one GraphQL round trip saved).
- /simplify skipped (would change the Gusto request sequence or flag declarations, or is speculative): parallel LoadDraft queries, one MarkCompanyTouchedHours per prep, partial reload after writes, poll backoff, includePreviewPay reuse, moving WritableErr into the save functions, a shared read-back helper, a flag-binding helper (verify-skill needs flags declared inline), google/uuid instead of NewUUID, dropping the PeriodDays guard.

## Template-shape retro candidates
- `internal/client/chrome.go` RoundTrip sent plain `http` URLs to the h2-only transport, so verify's local mock failed. Patched in the printed CLI (preserved across regen); belongs in the chrome transport template.
- `internal/cli/auth.go` cookie-tool probe leaked a pycookiecheat traceback into `doctor --json`. Patched via `pythonImportProbe`; belongs in the auth template.

## Out-of-scope retro candidates
- None in `internal/cliutil/` or `internal/mcp/cobratree/`.

## Surface-to-user findings
- None open.
