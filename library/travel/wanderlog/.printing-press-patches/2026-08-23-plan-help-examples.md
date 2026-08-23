# 2026-08-23 Plan help Examples and live plan-url

## Intent

Preserve dogfood `--live` help Examples and happy_path identifiers across reprints:

- Every leaf `plan *` command has an `Example:` that prints an `Examples:` help section.
- Read examples use `--plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent`.
- Write examples use that URL plus `--dry-run --agent`. Never `--apply`.
- Do not use fake `--target-key abcdefgh` / `abcdefghijklmnop` in Examples; those 404.

## Touched Surface

- `internal/cli/plan_budget.go`
- `internal/cli/plan_history.go`
- `internal/cli/plan_collab_ext.go`
- `internal/cli/plan_outline.go`
- `internal/cli/plan_votes.go`
- `internal/cli/plan_edit.go`
- `internal/cli/plan_edit_more.go`
- `internal/cli/plan_batch.go`
- `internal/cli/plan_reservation.go`

## Verification

- `go test ./internal/cli/ -count=1 -timeout 90s`
