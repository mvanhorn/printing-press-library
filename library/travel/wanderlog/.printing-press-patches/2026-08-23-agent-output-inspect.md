# 2026-08-23 Agent Output And Inspect

## Intent

Preserve P1 agent-output/inspect surfaces across reprints:

- Mutation JSON omits `op_paths` and the full `sections` array unless `--verbose`.
- `--deliver file:<path>` captures to the file only; `--also-stdout` mirrors stdout. Webhook still captures and mirrors stdout by default.
- `plan outline` / `plan inspect` omit undated/non-day section block lists unless `--all-sections`, keep the section row, and add `upvoted_by_count` on place blocks.
- `plan votes --target-key` lists place/hotel upvote counts (no comments API).
- `plan history` omits `forward_ops` / `inverse_ops` unless `--include-ops`.

## Touched Surface

- `internal/cli/plan_edit.go`: `printPlanEditReport` terse vs `--verbose`.
- `internal/cli/root.go`: `--verbose`, `--also-stdout`, file deliver PreRunE writer.
- `internal/cli/deliver.go`: file capture writer; comment.
- `internal/cli/plan_outline.go` + `plan_outline_test.go`.
- `internal/cli/plan_votes.go` + `plan_votes_test.go`.
- `internal/cli/plan.go`: AddCommand votes.
- `internal/cli/plan_history.go` + `plan_history_test.go`.
- `internal/cli/deliver_test.go`.

## Verification

- `go test ./internal/cli/ -count=1 -timeout 120s`
