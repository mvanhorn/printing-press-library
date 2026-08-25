# 2026-06-21 Plan Validity Guards

## Intent

Preserve two plan-mode fixes across future reprints:

- Checklist blocks must store each `items[].text` value as Wanderlog rich text, not a plain string, or the UI can render blank checklist bullets.
- `plan place add` must validate the target section date against fetched place `opening_hours` and block explicit closed-day placements by default.
- `plan sections` must report structured itinerary `issues` for existing place blocks that are closed on their dated section, so agents catch invalid placements during read/inspect workflows.

## Touched Surface

- `internal/cli/plan_collab_ext.go`: checklist item text shape.
- `internal/cli/plan_edit.go`: checklist summaries, closed-place validation for `plan place add`, and read-time itinerary issue reporting.
- `internal/cli/plan_edit_test.go`: focused shape and closed-day tests.
- `internal/cli/which.go`: command router text for `plan sections` issue reporting.
- `references/itinerary-editing.md`: agent instructions for checklist rich text and closed-place policy.

## Verification

- `go test ./...`
- `go build -o ./wanderlog-pp-cli ./cmd/wanderlog-pp-cli`
- `go build -o ./wanderlog-pp-mcp ./cmd/wanderlog-pp-mcp`
- `wanderlog-pp-cli plan checklist add ... --agent --select block` returned `item_text_types: ["richText"]`.
- `wanderlog-pp-cli plan place add --target-key myzniabckwmhlepn --day 1 --place-id ChIJq6qrMnZp5TQR8Q22HwB-Pbw --apply --agent` failed before mutation because Yunangi is closed on Sunday 2026-08-30.
- Temporary disposable-clone insertion with `--closed-place-policy ignore` produced `plan sections` issue `place_closed_on_section_date`; `plan undo --apply` removed it and the issue disappeared.
