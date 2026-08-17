# Conductor live workflow smoke

Date: 2026-08-05

- `launch`: created a disposable workspace and queued a harmless prompt.
- `monitor`: observed transcript movement before accepting the final idle state.
- `steer`: queued a follow-up prompt and collected a second transcript-backed completion receipt.
- `run`: launched and completed a disposable bounded task with `transcript-change-then-idle` proof.
- `plan-implement`: completed separate planner and implementation sessions in one disposable workspace.
- `archive`: archived every disposable workspace and confirmed archived status.
- Archived workspaces reject new messages and sessions with explicit API errors; Conductor's public API does not expose a restore endpoint.

No repository files were edited, and no commit, merge, or deployment action was permitted in any smoke prompt.
