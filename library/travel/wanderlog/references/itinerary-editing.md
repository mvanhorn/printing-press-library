# Itinerary Editing Reference

Days, sections, blocks, notes, places, schedules, attachments, and checklists. Inspect/apply rules: `SKILL.md`. Reservations: `reservations-attachments.md`. Raw ops: `sharedb-json0.md`.

## Workflow

1. `plan outline --target-key KEY [--day N] --agent`.
2. Dry-run the named command (default).
3. `--apply` after target approval.
4. `plan inspect --target-key KEY --check=unformatted,lodging-coverage --agent`.
5. `plan undo --apply` if wrong.

Prefer `--day`. Use `--section-index` or section id only after outline. Indexes shift after add/delete. Closed-place hits: move, replace, or mark optional — `ignore` only for intentional backups.

## Content

- `plan note add` — reminders and rationale.
- `plan place add` — `--place-id` or query. Default blocks places closed on the section date. `--closed-place-policy warn` for drafts; `ignore` only for backups.
- `plan checklist add` and `plan checklist item add|check|remove`. Item text is a rich-text object; string items can render blank.

## Block Rework

- `plan block move` / `plan block delete` — prefer `--block-id`.
- `plan block edit-text --markdown` — **format notes here.** What it compiles, and what it refuses: Gotchas in `SKILL.md`.
- `plan block rename --name "Property"` — sets `place.name` on lodging cards and geocoder street names.
- `plan block set-field` — non-protected fields; `--json-value` for non-strings. `place` is protected.
- `plan block schedule` — start/end/duration/timezone.
- `plan block attachment list/add/remove` — files/links on an existing block. Standalone Attachment item: `plan reservation add --kind attachment`.
- Costs: `plan budget expense add --block-id` (`budget.md`).

Do not hand-write Quill JSON0 for bold or bullets. `plan raw op` is the hatch when `--markdown` cannot express the field.

Note shape: bold label line only when the note mixes several kinds of info; one fact per bullet; bold last admission, cash-only, reservation-required, closing time.

## Day Sections

`plan section add-day`, `plan section set-field`, `plan section delete` (empty sections only).

## Batches

Write inspectable TSV/JSON first. Order: headings, day notes, place blocks, schedules. Avoid nested quoting in place-lookup loops. After each batch, outline or `plan inspect --target-key KEY --check=unformatted,lodging-coverage --agent`. Always write `--check=NAMES`: the space form `--check NAMES` silently runs all five checks and discards the argument.

Undo journal: `sharedb-json0.md`.
