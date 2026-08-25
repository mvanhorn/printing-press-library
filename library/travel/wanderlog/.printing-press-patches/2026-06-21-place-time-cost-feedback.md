# 2026-06-21 Place Time/Cost Feedback

## Intent

Preserve agent-facing feedback for Wanderlog place-card time and cost operations across future reprints:

- `plan block schedule` should preview the updated block schedule fields, not only the JSON0 paths.
- `plan budget expense add/edit/remove` should report the expense payload so agents can verify amount, currency, category, date, split, and `blockId`.
- Budget expenses linked to place cards with `--block-id` should validate that the itinerary block exists before apply.

## Touched Surface

- `internal/cli/plan_collab_ext.go`: schedule dry-run report now summarizes the preview block after schedule mutations.
- `internal/cli/plan_edit.go`: report schema includes budget/expense/payment payloads and block summaries include schedule fields.
- `internal/cli/plan_budget.go`: expense/payment/budget reports include payloads; expense block links validate against itinerary blocks.
- `internal/cli/plan_budget_test.go`: block-link validation and expense summary coverage.
- `references/budget.md` and `references/itinerary-editing.md`: document place-card "Add time" and "Add cost" mappings.

## Verification

- `go test ./...`
- `go build -o ./wanderlog-pp-cli ./cmd/wanderlog-pp-cli`
- `go build -o ./wanderlog-pp-mcp ./cmd/wanderlog-pp-mcp`
- Fake `--block-id` budget expense dry-run failed before mutation.
- Real `--block-id` budget expense dry-run returned an `expense` payload with `blockId`.
- Schedule dry-run returned a block payload with `startTime` and `durationMinutes`.
