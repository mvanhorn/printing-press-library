# 2026-08-23 Plan Batch ShareDB Verbs

## Intent

Preserve agent batch itinerary edits across reprints:

- `plan section swap-days --day I --with-day J` swaps the two day sections' `blocks` arrays in one JSON0 batch (`od`/`oi` on `itinerary.sections.i.blocks` twice). One journal record. Dry-run default.
- `plan fill-day --day N --stops-json` inserts N place blocks (`li`) using place-add resolve + closed-place policy. Optional start/end land on the inserted block; `note_md` uses the markdown compiler.
- `plan place replace` writes only nested `place`. Keep text, schedule, attachments, hotel, budget links, id, type. Re-run closed-place check.
- `plan block apply --ops-file` and `plan raw op --ops-file` parse a JSON0 array and use the same apply path as `plan raw op`. Dry-run default.

## Touched Surface

- `internal/cli/plan_batch.go` / `plan_batch_test.go`: constructors and op builders.
- `internal/cli/plan.go`: register `fill-day`.
- `internal/cli/plan_edit.go`: register `place replace` and `block apply`.
- `internal/cli/plan_edit_more.go`: register `section swap-days`; `raw op` accepts `--ops-file` as an alternative to `--op`.
- `internal/cli/which.go` / `which_test.go`: index the new commands.

## Verification

- `go test ./internal/cli/ -count=1 -timeout 120s`
